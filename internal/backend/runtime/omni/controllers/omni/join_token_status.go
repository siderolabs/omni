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
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// JoinTokenStatusController generates status of each join token registered in the system.
type JoinTokenStatusController = qtransform.QController[*siderolink.JoinToken, *siderolink.JoinTokenStatus]

const joinTokenStatusControllerName = "JoinTokenStatusController"

// NewJoinTokenStatusController instanciates the join token status controller.
func NewJoinTokenStatusController() *JoinTokenStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*siderolink.JoinToken, *siderolink.JoinTokenStatus]{
			Name: joinTokenStatusControllerName,
			MapMetadataFunc: func(res *siderolink.JoinToken) *siderolink.JoinTokenStatus {
				return siderolink.NewJoinTokenStatus(resources.DefaultNamespace, res.Metadata().ID())
			},
			UnmapMetadataFunc: func(res *siderolink.JoinTokenStatus) *siderolink.JoinToken {
				return siderolink.NewJoinToken(resources.DefaultNamespace, res.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, joinToken *siderolink.JoinToken, joinTokenStatus *siderolink.JoinTokenStatus) error {
				defaultJoinToken, err := safe.ReaderGetByID[*siderolink.DefaultJoinToken](ctx, r, siderolink.DefaultJoinTokenID)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				joinTokenStatus.TypedSpec().Value.IsDefault = defaultJoinToken != nil && defaultJoinToken.TypedSpec().Value.TokenId == joinToken.Metadata().ID()

				usages, err := safe.ReaderListAll[*siderolink.JoinTokenUsage](ctx, r)
				if err != nil {
					return err
				}

				var useCount uint64

				usages.ForEach(func(r *siderolink.JoinTokenUsage) {
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

				var requeueAfter time.Duration

				if joinTokenStatus.TypedSpec().Value.ExpirationTime != nil {
					requeueAfter = time.Until(joinTokenStatus.TypedSpec().Value.ExpirationTime.AsTime())
				}

				if requeueAfter > 0 {
					return controller.NewRequeueInterval(requeueAfter)
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, usage *siderolink.JoinTokenUsage) ([]resource.Pointer, error) {
				return []resource.Pointer{siderolink.NewJoinToken(resources.DefaultNamespace, usage.TypedSpec().Value.TokenId).Metadata()}, nil
			},
		),
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ *siderolink.DefaultJoinToken) ([]resource.Pointer, error) {
				items, err := safe.ReaderListAll[*siderolink.JoinToken](ctx, r)
				if err != nil {
					return nil, err
				}

				return safe.Map(items, func(item *siderolink.JoinToken) (resource.Pointer, error) { return item.Metadata(), nil })
			},
		),
		qtransform.WithConcurrency(4),
	)
}
