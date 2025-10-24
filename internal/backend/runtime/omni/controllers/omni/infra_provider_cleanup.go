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
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// InfraProviderCleanupController manages InfraProvider resource cleanup.
type InfraProviderCleanupController = cleanup.Controller[*infra.Provider]

// NewInfraProviderCleanupController returns a new InfraProviderCleanup controller.
// This controller removes infra.ProviderStatus and infra.ProviderHealthStatus resources reported by the provider.
// nolint:gocognit,gocyclo,cyclop
func NewInfraProviderCleanupController() *InfraProviderCleanupController {
	return cleanup.NewController(
		cleanup.Settings[*infra.Provider]{
			Name: "InfraProviderCleanupController",
			Handler: cleanup.Combine(
				&helpers.SameIDHandler[*infra.Provider, *infra.ProviderStatus]{},
				&helpers.SameIDHandler[*infra.Provider, *infra.ProviderHealthStatus]{},
				helpers.NewCustomHandler[*infra.Provider, *auth.Identity](func(ctx context.Context, r controller.Runtime, _ *zap.Logger, input *infra.Provider, _ string) error {
					ready, err := deleteServiceAccount(ctx, r, access.InfraProviderServiceAccountPrefix+input.Metadata().ID())
					if err != nil {
						return err
					}

					if !ready {
						return xerrors.NewTagged[cleanup.SkipReconcileTag](errors.New("the service account is still being destroyed"))
					}

					return nil
				}, helpers.WithExtraOutputs([]controller.Output{
					{
						Type: auth.PublicKeyType,
						Kind: controller.OutputShared,
					},
					{
						Type: auth.UserType,
						Kind: controller.OutputShared,
					},
				})),
				helpers.NewCustomHandler[*infra.Provider, *infra.Machine](func(ctx context.Context, r controller.Runtime, _ *zap.Logger, input *infra.Provider, _ string) error {
					machines, err := safe.ReaderListAll[*infra.Machine](ctx, r, state.WithLabelQuery(
						resource.LabelEqual(omni.LabelInfraProviderID, input.Metadata().ID()),
					))
					if err != nil {
						return err
					}

					for machine := range machines.All() {
						if err = r.RemoveFinalizer(ctx, machine.Metadata(), *machine.Metadata().Finalizers()...); err != nil {
							return err
						}
					}

					return nil
				}, helpers.WithExtraInputs(
					[]controller.Input{
						{
							Namespace: resources.InfraProviderNamespace,
							Type:      infra.InfraMachineType,
							Kind:      controller.InputStrong,
						},
					},
				), helpers.WithInputKind(controller.InputStrong), helpers.WithNoOutputs()),
				helpers.NewCustomHandler[*infra.Provider, *siderolink.Link](func(ctx context.Context, r controller.Runtime, _ *zap.Logger, input *infra.Provider, _ string) error {
					links, err := safe.ReaderListAll[*siderolink.Link](ctx, r)
					if err != nil {
						return err
					}

					for link := range links.All() {
						if infraProviderId, ok := link.Metadata().Annotations().Get(omni.LabelInfraProviderID); ok && infraProviderId == input.Metadata().ID() {
							_, teardownErr := r.Teardown(ctx, link.Metadata(), controller.WithOwner(""))
							if teardownErr != nil {
								return teardownErr
							}
						}
					}

					return nil
				}),
				helpers.NewCustomHandler[*infra.Provider, *infra.MachineStatus](func(ctx context.Context, r controller.Runtime, _ *zap.Logger, input *infra.Provider, _ string) error {
					machineStatuses, err := safe.ReaderListAll[*infra.MachineStatus](ctx, r, state.WithLabelQuery(
						resource.LabelEqual(omni.LabelInfraProviderID, input.Metadata().ID()),
					))
					if err != nil {
						return err
					}

					for machineStatus := range machineStatuses.All() {
						teardown, teardownErr := r.Teardown(ctx, machineStatus.Metadata(), controller.WithOwner(machineStatus.Metadata().Owner()))
						if teardownErr != nil {
							return teardownErr
						}

						if !teardown {
							return xerrors.NewTagged[cleanup.SkipReconcileTag](errors.New("the machine status is still being destroyed"))
						}

						if err = r.Destroy(ctx, machineStatus.Metadata(), controller.WithOwner(machineStatus.Metadata().Owner())); err != nil {
							return err
						}
					}

					return nil
				}),
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
