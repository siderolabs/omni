// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package auth contains auth-related controllers.
package auth

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// IdentityStatusController creates an IdentityStatus for each Identity, aggregating data from Identity, User, and IdentityLastActive resources.
type IdentityStatusController = qtransform.QController[*authres.Identity, *authres.IdentityStatus]

// NewIdentityStatusController instantiates the IdentityStatus controller.
func NewIdentityStatusController() *IdentityStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*authres.Identity, *authres.IdentityStatus]{
			Name: "IdentityStatusController",
			MapMetadataFunc: func(identity *authres.Identity) *authres.IdentityStatus {
				return authres.NewIdentityStatus(identity.Metadata().ID())
			},
			UnmapMetadataFunc: func(identityStatus *authres.IdentityStatus) *authres.Identity {
				return authres.NewIdentity(identityStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, identity *authres.Identity, status *authres.IdentityStatus) error {
				status.TypedSpec().Value.UserId = identity.TypedSpec().Value.UserId

				helpers.SyncAllLabels(identity, status)

				user, err := safe.ReaderGetByID[*authres.User](ctx, r, identity.TypedSpec().Value.UserId)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return fmt.Errorf("failed to get user: %w", err)
				}

				status.TypedSpec().Value.Role = user.TypedSpec().Value.Role

				lastActive, err := safe.ReaderGetByID[*authres.IdentityLastActive](ctx, r, identity.Metadata().ID())
				if err != nil {
					if state.IsNotFoundError(err) {
						status.TypedSpec().Value.LastActive = ""

						return nil
					}

					return fmt.Errorf("failed to get identity last active: %w", err)
				}

				if ts := lastActive.TypedSpec().Value.GetLastActive(); ts != nil {
					status.TypedSpec().Value.LastActive = ts.AsTime().UTC().Format(time.RFC3339)
				} else {
					status.TypedSpec().Value.LastActive = ""
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*authres.User](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, user controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				identities, err := safe.ReaderListAll[*authres.Identity](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(authres.LabelIdentityUserID, user.ID()),
				))
				if err != nil {
					return nil, err
				}

				return slices.Collect(identities.Pointers()), nil
			},
		),
		qtransform.WithExtraMappedInput[*authres.IdentityLastActive](
			qtransform.MapperSameID[*authres.Identity](),
		),
	)
}
