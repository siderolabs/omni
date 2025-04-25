// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// InfraProviderCleanupController manages InfraProvider resource cleanup.
type InfraProviderCleanupController = cleanup.Controller[*infra.Provider]

// NewInfraProviderCleanupController returns a new InfraProviderCleanup controller.
// This controller removes infra.ProviderStatus and infra.ProviderHealthStatus resources reported by the provider.
func NewInfraProviderCleanupController() *InfraProviderCleanupController {
	return cleanup.NewController(
		cleanup.Settings[*infra.Provider]{
			Name: "InfraProviderCleanupController",
			Handler: cleanup.Combine(
				&helpers.SameIDHandler[*infra.Provider, *infra.ProviderStatus]{},
				&helpers.SameIDHandler[*infra.Provider, *infra.ProviderHealthStatus]{},
				helpers.NewCustomHandler[*infra.Provider, *auth.Identity](func(ctx context.Context, r controller.Runtime, input *infra.Provider, _ string) error {
					ready, err := deleteServiceAccount(ctx, r, access.InfraProviderServiceAccountPrefix+input.Metadata().ID())
					if err != nil {
						return err
					}

					if !ready {
						return xerrors.NewTagged[cleanup.SkipReconcileTag](errors.New("the service account is still being destroyed"))
					}

					return nil
				}, false,
					controller.Output{
						Type: auth.PublicKeyType,
						Kind: controller.OutputShared,
					},
					controller.Output{
						Type: auth.UserType,
						Kind: controller.OutputShared,
					},
				),
			),
		},
	)
}

func deleteServiceAccount(ctx context.Context, r controller.ReaderWriter, name string) (bool, error) {
	sa := access.ParseServiceAccountFromName(name)
	id := sa.FullID()

	identity, err := safe.ReaderGetByID[*auth.Identity](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return true, nil
		}

		return false, err
	}

	_, isServiceAccount := identity.Metadata().Labels().Get(auth.LabelIdentityTypeServiceAccount)
	if !isServiceAccount {
		return true, nil
	}

	pubKeys, err := safe.ReaderListAll[*auth.PublicKey](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(auth.LabelIdentityUserID, identity.TypedSpec().Value.UserId)),
	)
	if err != nil {
		return false, err
	}

	userMD := auth.NewUser(resources.DefaultNamespace, identity.TypedSpec().Value.UserId).Metadata()

	for _, f := range []func() (bool, error){
		func() (bool, error) {
			return helpers.TeardownAndDestroyAll(ctx, r, pubKeys.Pointers(), controller.WithOwner(""))
		},
		func() (bool, error) {
			return helpers.TeardownAndDestroy(ctx, r, userMD, controller.WithOwner(""))
		},
		func() (bool, error) {
			return helpers.TeardownAndDestroy(ctx, r, identity.Metadata(), controller.WithOwner(""))
		},
	} {
		destroyed, err := f()
		if err != nil {
			return false, err
		}

		if !destroyed {
			return false, nil
		}
	}

	return true, nil
}
