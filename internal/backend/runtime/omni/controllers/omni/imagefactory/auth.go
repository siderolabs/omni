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
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// AuthControllerName is the name of AuthController.
//
// It is exported because the migration that adopts the resources previously written by the Omni
// startup path has to stamp this exact owner on them.
const AuthControllerName = "ImageFactoryAuthController"

// DefaultAuthRetryInterval is how long AuthController waits before retrying a teardown still held
// up by another controller's finalizer.
const DefaultAuthRetryInterval = 30 * time.Second

// AuthController maintains the credentials Omni uses to authenticate against the configured image
// factories.
//
// The set of factories comes from Omni's configuration rather than from a resource, so the
// controller has no inputs: it reconciles once, and comes back only when a removal is still
// pending.
type AuthController struct {
	factories []config.Factory
}

// NewAuthController creates a new AuthController.
func NewAuthController(registries *config.Registries) *AuthController {
	return &AuthController{
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

		retry, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			return err
		}

		if !retry {
			// The factories come from static configuration, so there is nothing left to wake up
			// for: leaving the timer unset parks the loop on the select above until the runtime
			// shuts down.
			logger.Debug("image factory auth is fully reconciled, no further work is scheduled")

			continue
		}

		timer.Reset(DefaultAuthRetryInterval)
	}
}

// reconcile brings the ImageFactoryAuth resources in line with the configured factories, and
// reports whether it has to be run again.
func (ctrl *AuthController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (bool, error) {
	authenticated := make([]string, 0, len(ctrl.factories))

	var errs []error

	for _, factory := range ctrl.factories {
		if factory.GetUsername() == "" || factory.GetPassword() == "" {
			// The factory serves Omni anonymously, so it gets no resource at all.
			continue
		}

		factoryURL := strings.TrimRight(factory.GetUrl(), "/")

		authenticated = append(authenticated, factoryURL)

		if err := safe.WriterModify(ctx, r, omni.NewImageFactoryAuth(factoryURL), func(res *omni.ImageFactoryAuth) error {
			res.TypedSpec().Value.Username = factory.GetUsername()
			res.TypedSpec().Value.Password = factory.GetPassword()

			return nil
		}); err != nil {
			errs = append(errs, fmt.Errorf("image factory %q: %w", factoryURL, err))
		}
	}

	// Pruning runs even when a factory failed above: an unconfigured factory's credentials should go
	// regardless of whether some other factory's credentials could be written.
	pruned, err := ctrl.pruneAuth(ctx, r, authenticated)
	if err != nil {
		errs = append(errs, err)
	}

	if err := errors.Join(errs...); err != nil {
		return false, err
	}

	if !pruned {
		// A teardown is waiting on another controller to drop its finalizer, so come back for it.
		logger.Debug("image factory auth removal is still pending, scheduling a retry")

		return true, nil
	}

	return false, nil
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
