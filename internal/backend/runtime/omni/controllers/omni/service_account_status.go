// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

// ServiceAccountStatusController creates omni.ServiceAccountStatus for each identity.
//
// ServiceAccountStatusController generates information about Talos extensions available for a Talos version.
type ServiceAccountStatusController = qtransform.QController[*auth.Identity, *auth.ServiceAccountStatus]

const serviceAccountStatusControllerName = "ServiceAccountStatusController"

// NewServiceAccountStatusController instantiates the ServiceAccountStatus controller.
//
//nolint:gocognit
func NewServiceAccountStatusController() *ServiceAccountStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*auth.Identity, *auth.ServiceAccountStatus]{
			Name: serviceAccountStatusControllerName,
			MapMetadataOptionalFunc: func(identity *auth.Identity) optional.Optional[*auth.ServiceAccountStatus] {
				if _, isServiceAccount := identity.Metadata().Labels().Get(auth.LabelIdentityTypeServiceAccount); !isServiceAccount {
					return optional.None[*auth.ServiceAccountStatus]()
				}

				if strings.HasSuffix(identity.Metadata().ID(), access.InfraProviderServiceAccountNameSuffix) {
					return optional.None[*auth.ServiceAccountStatus]()
				}

				return optional.Some(auth.NewServiceAccountStatus(identity.Metadata().ID()))
			},
			UnmapMetadataFunc: func(r *auth.ServiceAccountStatus) *auth.Identity {
				return auth.NewIdentity(r.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, identity *auth.Identity, status *auth.ServiceAccountStatus) error {
				publicKeyList, err := safe.ReaderListAll[*auth.PublicKey](
					ctx,
					r,
					state.WithLabelQuery(resource.LabelEqual(auth.LabelPublicKeyUserID, identity.TypedSpec().Value.UserId)),
				)
				if err != nil {
					return err
				}

				status.TypedSpec().Value.PublicKeys = nil

				for key := range publicKeyList.All() {
					if key.Metadata().Phase() == resource.PhaseRunning {
						if !key.Metadata().Finalizers().Has(serviceAccountStatusControllerName) {
							if err = r.AddFinalizer(ctx, key.Metadata(), serviceAccountStatusControllerName); err != nil {
								return err
							}
						}
					} else {
						if err = r.RemoveFinalizer(ctx, key.Metadata(), serviceAccountStatusControllerName); err != nil {
							return err
						}

						continue
					}

					status.TypedSpec().Value.PublicKeys = append(status.TypedSpec().Value.PublicKeys,
						&specs.ServiceAccountStatusSpec_PgpPublicKey{
							Id:         key.Metadata().ID(),
							Armored:    string(key.TypedSpec().Value.GetPublicKey()),
							Expiration: key.TypedSpec().Value.GetExpiration(),
						},
					)
				}

				user, err := safe.ReaderGetByID[*auth.User](ctx, r, identity.TypedSpec().Value.UserId)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return err
				}

				status.TypedSpec().Value.Role = user.TypedSpec().Value.Role

				return nil
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, identity *auth.Identity) error {
				publicKeyList, err := safe.ReaderListAll[*auth.PublicKey](
					ctx,
					r,
					state.WithLabelQuery(resource.LabelEqual(auth.LabelPublicKeyUserID, identity.TypedSpec().Value.UserId)),
				)
				if err != nil {
					return err
				}

				for key := range publicKeyList.All() {
					if err = r.RemoveFinalizer(ctx, key.Metadata(), serviceAccountStatusControllerName); err != nil {
						return err
					}
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*auth.User](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, user controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				identities, err := safe.ReaderListAll[*auth.Identity](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(auth.LabelIdentityUserID, user.ID())),
				)
				if err != nil {
					return nil, err
				}

				return safe.Map(identities, func(i *auth.Identity) (resource.Pointer, error) {
					return i.Metadata(), nil
				})
			},
		),
		qtransform.WithExtraMappedInput[*auth.PublicKey](
			qtransform.MapperFuncFromTyped[*auth.PublicKey](
				func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, key *auth.PublicKey) ([]resource.Pointer, error) {
					return []resource.Pointer{
						auth.NewIdentity(key.TypedSpec().Value.Identity.Email).Metadata(),
					}, nil
				},
			),
		),
	)
}
