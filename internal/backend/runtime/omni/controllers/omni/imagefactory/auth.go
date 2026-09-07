// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/golang-jwt/jwt/v5"
	"github.com/siderolabs/image-factory/pkg/client"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/imagefactory/tokenfile"
)

// AuthControllerName is the name of AuthController.
//
// It is exported because the migration that adopts the resources previously written by the Omni
// startup path has to stamp this exact owner on them.
const AuthControllerName = "ImageFactoryAuthController"

// DefaultAuthRetryInterval is how long AuthController waits before retrying a teardown still held
// up by another controller's finalizer.
const DefaultAuthRetryInterval = 30 * time.Second

const (
	// machineTokenScope is what the machine token may do: pull installer images and download boot
	// assets, and nothing else. It is the factory's own scope name.
	machineTokenScope = "image:read"

	// machineTokenRequestTimeout bounds the token request on its own, since the factory client's
	// timeout is sized for the slowest thing it does.
	machineTokenRequestTimeout = 30 * time.Second

	// machineTokenRenewAfter is the share of the machine token's lifetime after which it is
	// replaced. The old one keeps working until it expires, so a machine that gets its config
	// regenerated late, say one powered off for months, still pulls with a valid token.
	machineTokenRenewAfter = 0.5
)

// AuthController maintains the credentials Omni and its machines use to authenticate against the
// configured image factories.
//
// Omni's own credentials come from its configuration (the basic auth pair as values, the API token
// from a file that is followed as it changes) and are copied into the ImageFactoryAuth resource. For
// a factory Omni authenticates to with an API token, the controller also creates the machine token
// on the factory: a stored, pull-only token that goes into the machine configs, and that the
// controller replaces before it expires.
//
// The set of factories comes from Omni's configuration rather than from a resource, so the
// controller has no inputs: it reconciles once, and comes back for a pending removal, a rotated
// token file, or the next machine token renewal.
type AuthController struct {
	clients *imagefactory.Clients
	tokens  *tokenfile.Set

	// accountName goes into the machine token name, so an operator can tell whose token it is in the
	// factory's token listing.
	accountName string
	factories   []config.Factory
}

// NewAuthController creates a new AuthController.
func NewAuthController(registries *config.Registries, accountName string, clients *imagefactory.Clients, tokens *tokenfile.Set) *AuthController {
	return &AuthController{
		factories:   registries.AllFactories(),
		accountName: accountName,
		clients:     clients,
		tokens:      tokens,
	}
}

// Name implements controller.Controller interface.
func (ctrl *AuthController) Name() string {
	return AuthControllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *AuthController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *AuthController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ImageFactoryAuthType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *AuthController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// The runtime queues the first reconcile itself, so the timer starts stopped: nothing is scheduled
	// until a reconcile asks for it.
	timer := time.NewTimer(0)
	timer.Stop()

	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-timer.C:
		case <-ctrl.tokens.Changed():
			logger.Info("an image factory token was rotated")
		}

		next, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			return err
		}

		if next == 0 {
			// The factories come from static configuration, so apart from a token rotation there is
			// nothing left to wake up for: the loop parks on the select above until the runtime shuts
			// down.
			logger.Debug("image factory auth is fully reconciled, no further work is scheduled")

			timer.Stop()

			continue
		}

		timer.Reset(next)
	}
}

// reconcile brings the ImageFactoryAuth resources in line with the configured factories, and
// reports when it has to be run again: zero for never.
func (ctrl *AuthController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (time.Duration, error) {
	authenticated := make([]string, 0, len(ctrl.factories))

	var (
		errs []error
		next time.Duration
	)

	for _, factory := range ctrl.factories {
		if !factory.RequiresAuth() {
			// The factory serves Omni anonymously, so it gets no resource at all.
			continue
		}

		factoryURL := imagefactory.NormalizeFactoryURL(factory.GetUrl())

		authenticated = append(authenticated, factoryURL)

		renewIn, err := ctrl.reconcileFactory(ctx, r, logger.With(zap.String("factory", factoryURL)), factory, factoryURL)
		if err != nil {
			errs = append(errs, fmt.Errorf("image factory %q: %w", factoryURL, err))
		}

		next = earliest(next, renewIn)
	}

	// Pruning runs even when a factory failed above: an unconfigured factory's credentials should go
	// regardless of whether some other factory's credentials could be written.
	pruned, err := ctrl.pruneAuth(ctx, r, authenticated)
	if err != nil {
		errs = append(errs, err)
	}

	if err := errors.Join(errs...); err != nil {
		return 0, err
	}

	if !pruned {
		// A teardown is waiting on another controller to drop its finalizer, so come back for it.
		logger.Debug("image factory auth removal is still pending, scheduling a retry")

		next = earliest(next, DefaultAuthRetryInterval)
	}

	return next, nil
}

// reconcileFactory writes the credentials of one factory, and reports in how long the machine token
// is due for renewal: zero when there is none to renew.
//
// For a factory Omni authenticates to with basic auth, the resource carries the username and
// password and nothing else: the machines pull with those. For a factory Omni authenticates to with
// an API token, the resource carries that token and the machine token. It is written even while the
// machine token could not be created yet: the machine configs refuse a factory in that state, rather
// than being generated without registry credentials for a factory that needs them.
func (ctrl *AuthController) reconcileFactory(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, factory config.Factory, factoryURL string,
) (time.Duration, error) {
	existing, err := safe.ReaderGetByID[*omni.ImageFactoryAuth](ctx, r, factoryURL)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, err
	}

	// The current content of the token file, empty for a basic auth factory.
	apiToken := ctrl.tokens.Get(factoryURL)

	if factory.GetTokenFile() != "" && apiToken == "" {
		return 0, fmt.Errorf("the token file %q of the image factory is not loaded", factory.GetTokenFile())
	}

	var machineToken string

	if existing != nil {
		machineToken = existing.TypedSpec().Value.MachineToken

		// A machine token created with a previous Omni token is not trusted to still be valid: it may
		// live on a different factory, or its records may have been wiped along with the old token.
		// The replaced token is not revoked, it expires on its own and the factory only counts live
		// tokens against its cap.
		if existing.TypedSpec().Value.ApiToken != apiToken {
			machineToken = ""
		}
	}

	var (
		renewIn   time.Duration
		ensureErr error
	)

	if apiToken != "" {
		// On a failure the current machine token is kept, which is none on a cold start. The resource
		// is written either way, the error is returned after that.
		machineToken, renewIn, ensureErr = ctrl.ensureMachineToken(ctx, logger, factoryURL, machineToken)
	} else {
		// Basic auth: the machines use the same credentials as Omni, no machine token is needed.
		machineToken = ""
	}

	// Every field is assigned, so switching a factory between basic auth and a token leaves nothing
	// of the previous mode behind.
	if err := safe.WriterModify(ctx, r, omni.NewImageFactoryAuth(factoryURL), func(res *omni.ImageFactoryAuth) error {
		res.TypedSpec().Value.Username = factory.GetUsername()
		res.TypedSpec().Value.Password = factory.GetPassword()
		res.TypedSpec().Value.ApiToken = apiToken
		res.TypedSpec().Value.MachineToken = machineToken

		return nil
	}); err != nil {
		return 0, err
	}

	return renewIn, ensureErr
}

// ensureMachineToken returns the machine token to put into the machine configs: the current one
// while it is fresh, a new one otherwise. It reports in how long the returned token is due for
// renewal.
//
// A failed request is an error, and the current token is returned with it: the machines keep it,
// it works until it expires, and the runtime retries with its backoff.
func (ctrl *AuthController) ensureMachineToken(ctx context.Context, logger *zap.Logger, factoryURL, current string) (string, time.Duration, error) {
	now := time.Now()

	if current != "" {
		lifetime, err := parseTokenLifetime(current)
		if err != nil {
			logger.Warn("the machine token cannot be parsed, replacing it", zap.Error(err))
		} else if renewIn := lifetime.renewIn(now); renewIn > 0 {
			return current, renewIn, nil
		}
	}

	factoryClient := ctrl.clients.ForURL(factoryURL)
	if factoryClient == nil {
		return current, 0, fmt.Errorf("no image factory client is configured for %q", factoryURL)
	}

	token, err := ctrl.createMachineToken(ctx, factoryClient, now)
	if err != nil {
		return current, 0, fmt.Errorf("failed to create the machine token: %w", err)
	}

	lifetime, err := parseTokenLifetime(token)
	if err != nil {
		return current, 0, fmt.Errorf("the image factory returned a machine token that cannot be parsed: %w", err)
	}

	logger.Info("created the machine token on the image factory", zap.Time("expires_at", lifetime.expiresAt))

	// A token that is already due when it arrives (the factory's clock behind Omni's by most of the
	// token's lifetime) schedules nothing, so the renewal waits for the next restart. Creating another
	// token right away would only produce another one already due, and burn the per-org cap.
	return token, lifetime.renewIn(now), nil
}

// createMachineToken creates the stored, pull-only token the machines authenticate with.
func (ctrl *AuthController) createMachineToken(ctx context.Context, factoryClient imagefactory.FactoryClient, now time.Time) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, machineTokenRequestTimeout)
	defer cancel()

	_, token, err := factoryClient.TokenCreate(ctx, client.TokenCreateOptions{
		Name:   fmt.Sprintf("omni-machines-%s-%s", ctrl.accountName, now.UTC().Format(time.DateOnly)),
		Scopes: []string{machineTokenScope},
	})
	if err != nil {
		return "", err
	}

	if token == "" {
		return "", errors.New("the image factory returned an empty token")
	}

	return token, nil
}

// tokenLifetime is when an API token was issued and when it expires, read from its claims.
type tokenLifetime struct {
	issuedAt  time.Time
	expiresAt time.Time
}

// renewIn reports in how long the token is due for renewal, or zero when it already is.
func (l tokenLifetime) renewIn(now time.Time) time.Duration {
	renewAt := l.issuedAt.Add(time.Duration(float64(l.expiresAt.Sub(l.issuedAt)) * machineTokenRenewAfter))

	return max(renewAt.Sub(now), 0)
}

// parseTokenLifetime reads the issue and expiry times of an API token.
//
// The token is parsed without verifying its signature: Omni got it from the factory itself over TLS,
// and only reads when to replace it. The factory verifies it when the machines present it.
func parseTokenLifetime(token string) (tokenLifetime, error) {
	claims := jwt.RegisteredClaims{}

	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		return tokenLifetime{}, err
	}

	if claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return tokenLifetime{}, errors.New("the token carries no issue or expiry time")
	}

	if !claims.ExpiresAt.After(claims.IssuedAt.Time) {
		return tokenLifetime{}, fmt.Errorf("the token expires (%s) before it was issued (%s)", claims.ExpiresAt, claims.IssuedAt)
	}

	return tokenLifetime{issuedAt: claims.IssuedAt.Time, expiresAt: claims.ExpiresAt.Time}, nil
}

// earliest returns the shorter of two wait times, where zero means "no wait scheduled".
func earliest(a, b time.Duration) time.Duration {
	switch {
	case a == 0:
		return b
	case b == 0:
		return a
	default:
		return min(a, b)
	}
}

// pruneAuth removes the credentials of factories that are no longer configured, or that no longer
// authenticate Omni at all. It reports whether every one of them is gone.
//
// Nothing puts a finalizer on ImageFactoryAuth today — ClusterMachineConfigController takes it as a
// plain mapped input — but destroying a resource outright would start failing the moment something
// did, so the removal goes through teardown and only destroys once the finalizers are clear.
func (ctrl *AuthController) pruneAuth(ctx context.Context, r controller.Runtime, authenticated []string) (bool, error) {
	existing, err := safe.ReaderListAll[*omni.ImageFactoryAuth](ctx, r)
	if err != nil {
		return false, err
	}

	allRemoved := true

	var errs []error

	for auth := range existing.All() {
		if slices.Contains(authenticated, auth.Metadata().ID()) {
			continue
		}

		// TeardownAndDestroy reports a resource that is already gone as removed, so a not-found
		// error never reaches here.
		destroyed, err := helpers.TeardownAndDestroy(ctx, r, auth.Metadata())
		if err != nil {
			// One factory's credentials failing to go must not strand the rest: a single resource
			// the controller cannot touch — one the ownership migration missed, say — would
			// otherwise wedge every removal behind it for good.
			errs = append(errs, fmt.Errorf("failed to remove the credentials of image factory %q: %w", auth.Metadata().ID(), err))
		}

		if err != nil || !destroyed {
			allRemoved = false
		}
	}

	return allRemoved, errors.Join(errs...)
}
