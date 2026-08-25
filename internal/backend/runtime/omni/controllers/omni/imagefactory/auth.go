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
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/imagefactory/tokens"
)

// AuthControllerName is the name of AuthController.
//
// It is exported because the migration that adopts the resources previously written by the Omni
// startup path has to stamp this exact owner on them.
const AuthControllerName = "ImageFactoryAuthController"

const (
	// DefaultAuthRetryInterval is how long AuthController waits before
	// retrying work it could not finish: a token request that failed, or a teardown still held up by
	// another controller's finalizer.
	//
	// A failed token request is not urgent: tokens are replaced halfway through their lifetime, so
	// the token in state is still valid for about as long as it has already lived.
	DefaultAuthRetryInterval = 30 * time.Second
)

// AuthController maintains the credentials Omni uses to authenticate against the
// configured image factories: the basic auth taken straight from the configuration, and the
// access tokens issued and rotated against the OAuth2 client.
//
// The set of factories comes from Omni's configuration rather than from a resource, so the
// controller has no inputs: it reconciles on its own schedule, derived from the lifetimes of the
// tokens it has issued.
type AuthController struct {
	issuers   *tokens.Issuers
	factories []config.Factory
}

// NewAuthController creates a new AuthController.
func NewAuthController(registries *config.Registries, issuers *tokens.Issuers) *AuthController {
	return &AuthController{
		issuers:   issuers,
		factories: registries.AllFactories(),
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
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-timer.C:
		}

		sleep, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			// A failing token endpoint must not take the controller down: the credentials already in
			// state stay usable, and the next attempt may well succeed.
			logger.Error("failed to reconcile image factory auth", zap.Error(err))

			return err
		}

		if sleep <= 0 {
			// Nothing on record expires and nothing is left unfinished. The factories come from
			// static configuration, so there is nothing left to wake up for: leaving the timer unset
			// parks the loop on the select above until the runtime shuts down. An installation with
			// no OAuth2 configured factory reconciles exactly once.
			logger.Debug("image factory auth is fully reconciled, no further work is scheduled")

			continue
		}

		timer.Reset(sleep)
	}
}

// reconcile brings the ImageFactoryAuth resources in line with the configured factories, and returns
// how long to wait before the next reconciliation.
func (ctrl *AuthController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (time.Duration, error) {
	authenticated := make([]string, 0, len(ctrl.factories))

	// A zero sleep means nothing needs scheduling; only a token that expires, or work left
	// unfinished, puts a wake-up on the clock.
	var (
		sleep time.Duration
		errs  []error
	)

	for _, factory := range ctrl.factories {
		factoryURL := tokens.NormalizeFactoryURL(factory.GetUrl())

		issuer := ctrl.issuers.ForURL(factoryURL)

		hasBasicAuth := factory.GetUsername() != "" && factory.GetPassword() != ""
		if !hasBasicAuth && issuer == nil {
			// The factory serves Omni anonymously, so it gets no resource at all.
			continue
		}

		authenticated = append(authenticated, factoryURL)

		refreshIn, err := ctrl.reconcileFactory(ctx, r, logger, factory, factoryURL, issuer)
		if err != nil {
			errs = append(errs, fmt.Errorf("image factory %q: %w", factoryURL, err))

			continue
		}

		sleep = ctrl.soonest(sleep, refreshIn)
	}

	// Pruning runs even when a factory failed above: an unconfigured factory's credentials should go
	// regardless of whether some other factory's token could be renewed.
	pruned, err := ctrl.pruneAuth(ctx, r, authenticated)
	if err != nil {
		errs = append(errs, err)
	}

	if !pruned {
		// A teardown is waiting on another controller to drop its finalizer, so come back for it.
		sleep = ctrl.soonest(sleep, DefaultAuthRetryInterval)
	}

	if err := errors.Join(errs...); err != nil {
		return 0, err
	}

	return sleep, nil
}

// reconcileFactory writes the factory's credentials, issuing a token first when the factory has an
// OAuth2 client and the token on record is missing, stale, or due for replacement.
//
// It returns how long the resulting token stays usable, or zero for a factory that has basic auth
// only and so has nothing that expires.
func (ctrl *AuthController) reconcileFactory(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	factory config.Factory,
	factoryURL string,
	issuer tokens.Issuer,
) (time.Duration, error) {
	existing, err := safe.ReaderGetByID[*omni.ImageFactoryAuth](ctx, r, factoryURL)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, err
	}

	var (
		token     *specs.ImageFactoryAuthSpec_AccessToken
		tokenErr  error
		refreshIn time.Duration
	)

	if existing != nil {
		token = existing.TypedSpec().Value.GetToken()
	}

	switch {
	case issuer == nil && token != nil:
		// The factory's OAuth2 client was removed from the configuration, so the token it issued
		// must not be left lying around.
		logger.Info("dropping image factory token, the factory is no longer configured with OAuth2 credentials", zap.String("factory_url", factoryURL))

		token = nil
	case issuer != nil:
		issuerChanged := token.GetClientId() != issuer.ClientID() || token.GetAudience() != issuer.Audience()

		if token != nil {
			issuedAt := token.GetIssuedAt().AsTime()
			expiresAt := token.GetExpiresAt().AsTime()

			refreshIn = time.Until(issuedAt.Add(expiresAt.Sub(issuedAt) / 2))
		}

		if token == nil || refreshIn <= 0 || issuerChanged {
			issued, issueErr := ctrl.issueToken(ctx, logger, issuer, factoryURL)
			if issueErr != nil {
				tokenErr = issueErr
			} else {
				// Only set the token when successful
				token = issued
			}
		}
	}

	// The write happens even when the token could not be issued. Basic auth is what the machines'
	// registry configuration is built from, so a factory whose authorization server is unreachable
	// must still get the credentials it does have.
	if err = safe.WriterModify(ctx, r, omni.NewImageFactoryAuth(factoryURL), func(res *omni.ImageFactoryAuth) error {
		res.TypedSpec().Value.Username = factory.GetUsername()
		res.TypedSpec().Value.Password = factory.GetPassword()
		res.TypedSpec().Value.Token = token

		return nil
	}); err != nil {
		return 0, err
	}

	return refreshIn, tokenErr
}

// issueToken obtains a fresh token from the issuer.
func (ctrl *AuthController) issueToken(ctx context.Context, logger *zap.Logger, issuer tokens.Issuer, factoryURL string) (*specs.ImageFactoryAuthSpec_AccessToken, error) {
	token, err := issuer.IssueToken(ctx)
	if err != nil {
		return nil, err
	}

	logger.Info(
		"issued image factory token",
		zap.String("factory_url", factoryURL),
		zap.Duration("lifetime", token.Lifetime()),
		zap.Time("expires_at", token.ExpiresAt),
	)

	return &specs.ImageFactoryAuthSpec_AccessToken{
		Token:     token.Token,
		TokenType: token.TokenType,
		IssuedAt:  timestamppb.New(token.IssuedAt),
		ExpiresAt: timestamppb.New(token.ExpiresAt),
		ClientId:  issuer.ClientID(),
		Audience:  issuer.Audience(),
	}, nil
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

// soonest folds a candidate wake-up into the earliest one scheduled so far, where a zero value on
// either side means "nothing scheduled".
func (ctrl *AuthController) soonest(current, candidate time.Duration) time.Duration {
	if candidate <= 0 {
		return current
	}

	if current <= 0 {
		return candidate
	}

	return min(current, candidate)
}
