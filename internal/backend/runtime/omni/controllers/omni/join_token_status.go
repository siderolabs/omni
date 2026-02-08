// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// JoinTokenStatusController generates status of each join token registered in the system.
type JoinTokenStatusController = qtransform.QController[*siderolink.JoinToken, *siderolink.JoinTokenStatus]

const joinTokenStatusControllerName = "JoinTokenStatusController"

// NewJoinTokenStatusController instantiates the join token status controller.
//
//nolint:gocognit,gocyclo,cyclop
func NewJoinTokenStatusController() *JoinTokenStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*siderolink.JoinToken, *siderolink.JoinTokenStatus]{
			Name: joinTokenStatusControllerName,
			MapMetadataFunc: func(res *siderolink.JoinToken) *siderolink.JoinTokenStatus {
				return siderolink.NewJoinTokenStatus(res.Metadata().ID())
			},
			UnmapMetadataFunc: func(res *siderolink.JoinTokenStatus) *siderolink.JoinToken {
				return siderolink.NewJoinToken(res.Metadata().ID())
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

				joinTokenStatus.TypedSpec().Value.Warnings = nil

				var useCount uint64

				usages.ForEach(func(res *siderolink.JoinTokenUsage) {
					if res.TypedSpec().Value.TokenId != joinToken.Metadata().ID() {
						return
					}

					useCount++

					var nodeUniqueTokenStatus *siderolink.NodeUniqueTokenStatus

					nodeUniqueTokenStatus, err = safe.ReaderGetByID[*siderolink.NodeUniqueTokenStatus](ctx, r, res.Metadata().ID())
					if err != nil && !state.IsNotFoundError(err) {
						return
					}

					if nodeUniqueTokenStatus == nil {
						joinTokenStatus.TypedSpec().Value.Warnings = append(
							joinTokenStatus.TypedSpec().Value.Warnings,
							&specs.JoinTokenStatusSpec_Warning{
								Machine: res.Metadata().ID(),
								Message: "Does not have the node unique token",
							},
						)

						return
					}

					switch nodeUniqueTokenStatus.TypedSpec().Value.State {
					case specs.NodeUniqueTokenStatusSpec_EPHEMERAL:
						joinTokenStatus.TypedSpec().Value.Warnings = append(
							joinTokenStatus.TypedSpec().Value.Warnings,
							&specs.JoinTokenStatusSpec_Warning{
								Machine: res.Metadata().ID(),
								Message: "Talos is not installed so the generated node unique token is ephemeral",
							},
						)
					case specs.NodeUniqueTokenStatusSpec_NONE:
						joinTokenStatus.TypedSpec().Value.Warnings = append(
							joinTokenStatus.TypedSpec().Value.Warnings,
							&specs.JoinTokenStatusSpec_Warning{
								Machine: res.Metadata().ID(),
								Message: "Does not have the node unique token",
							},
						)
					case specs.NodeUniqueTokenStatusSpec_UNSUPPORTED:
						joinTokenStatus.TypedSpec().Value.Warnings = append(
							joinTokenStatus.TypedSpec().Value.Warnings,
							&specs.JoinTokenStatusSpec_Warning{
								Machine: res.Metadata().ID(),
								Message: "Installed Talos version does not support unique node tokens",
							},
						)
					case specs.NodeUniqueTokenStatusSpec_UNKNOWN:
						joinTokenStatus.TypedSpec().Value.Warnings = append(
							joinTokenStatus.TypedSpec().Value.Warnings,
							&specs.JoinTokenStatusSpec_Warning{
								Machine: res.Metadata().ID(),
								Message: "The machine node unique token status is not determined",
							},
						)
					case specs.NodeUniqueTokenStatusSpec_PERSISTENT:
						// all good, can be rotated
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
		qtransform.WithExtraMappedInput[*siderolink.JoinTokenUsage](
			qtransform.MapperFuncFromTyped[*siderolink.JoinTokenUsage](
				func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, usage *siderolink.JoinTokenUsage) ([]resource.Pointer, error) {
					return []resource.Pointer{siderolink.NewJoinToken(usage.TypedSpec().Value.TokenId).Metadata()}, nil
				},
			),
		),
		qtransform.WithExtraMappedInput[*siderolink.DefaultJoinToken](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				items, err := safe.ReaderListAll[*siderolink.JoinToken](ctx, r)
				if err != nil {
					return nil, err
				}

				return safe.Map(items, func(item *siderolink.JoinToken) (resource.Pointer, error) { return item.Metadata(), nil })
			},
		),
		qtransform.WithExtraMappedInput[*siderolink.NodeUniqueTokenStatus](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, status controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				usage, err := safe.ReaderGetByID[*siderolink.JoinTokenUsage](ctx, r, status.ID())
				if err != nil {
					if state.IsNotFoundError(err) {
						return nil, nil
					}

					return nil, err
				}

				return []resource.Pointer{siderolink.NewJoinToken(usage.TypedSpec().Value.TokenId).Metadata()}, nil
			},
		),
		qtransform.WithConcurrency(4),
	)
}
