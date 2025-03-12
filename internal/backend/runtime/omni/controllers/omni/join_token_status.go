// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

// JoinTokenStatusController generates status of each join token registered in the system.
type JoinTokenStatusController = qtransform.QController[*auth.JoinToken, *auth.JoinTokenStatus]

const joinTokenStatusControllerName = "JoinTokenStatusController"

// NewJoinTokenStatusController instanciates the join token status controller.
func NewJoinTokenStatusController() *JoinTokenStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*auth.JoinToken, *auth.JoinTokenStatus]{
			Name: joinTokenStatusControllerName,
			MapMetadataFunc: func(res *auth.JoinToken) *auth.JoinTokenStatus {
				return auth.NewJoinTokenStatus(resources.DefaultNamespace, res.Metadata().ID())
			},
			UnmapMetadataFunc: func(res *auth.JoinTokenStatus) *auth.JoinToken {
				return auth.NewJoinToken(resources.DefaultNamespace, res.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, joinToken *auth.JoinToken, joinTokenStatus *auth.JoinTokenStatus) error {
				defaultJoinToken, err := safe.ReaderGetByID[*auth.DefaultJoinToken](ctx, r, auth.DefaultJoinTokenID)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				joinTokenStatus.TypedSpec().Value.IsDefault = defaultJoinToken != nil && defaultJoinToken.TypedSpec().Value.TokenId == joinToken.Metadata().ID()

				links, err := safe.ReaderListAll[*auth.JoinTokenUsage](ctx, r)
				if err != nil {
					return err
				}

				var useCount uint64

				links.ForEach(func(r *auth.JoinTokenUsage) {
					if r.TypedSpec().Value.TokenId == joinToken.Metadata().ID() {
						useCount++
					}
				})

				switch {
				case joinToken.TypedSpec().Value.Revoked:
					joinTokenStatus.TypedSpec().Value.State = specs.JoinTokenStatusSpec_REVOKED

					joinTokenStatus.Metadata().Labels().Delete(auth.LabelTokenActive)
				case joinToken.TypedSpec().Value.ExpirationTime != nil &&
					time.Now().After(joinToken.TypedSpec().Value.ExpirationTime.AsTime()):
					joinTokenStatus.TypedSpec().Value.State = specs.JoinTokenStatusSpec_EXPIRED

					joinTokenStatus.Metadata().Labels().Delete(auth.LabelTokenActive)
				default:
					joinTokenStatus.TypedSpec().Value.State = specs.JoinTokenStatusSpec_ACTIVE

					joinTokenStatus.Metadata().Labels().Set(auth.LabelTokenActive, "")
				}

				joinTokenStatus.TypedSpec().Value.UseCount = useCount
				joinTokenStatus.TypedSpec().Value.ExpirationTime = joinToken.TypedSpec().Value.ExpirationTime
				joinTokenStatus.TypedSpec().Value.Name = joinToken.TypedSpec().Value.Name

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, usage *auth.JoinTokenUsage) ([]resource.Pointer, error) {
				return []resource.Pointer{auth.NewJoinToken(resources.DefaultNamespace, usage.TypedSpec().Value.TokenId).Metadata()}, nil
			},
		),
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ *auth.DefaultJoinToken) ([]resource.Pointer, error) {
				items, err := safe.ReaderListAll[*auth.JoinToken](ctx, r)
				if err != nil {
					return nil, err
				}

				return safe.Map(items, func(item *auth.JoinToken) (resource.Pointer, error) { return item.Metadata(), nil })
			},
		),
		qtransform.WithConcurrency(4),
	)
}
