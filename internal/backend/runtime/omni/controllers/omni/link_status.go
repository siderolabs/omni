// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

type siderolinkSpecWrapper interface {
	generic.ResourceWithRD
	TypedSpec() *protobuf.ResourceSpec[specs.SiderolinkSpec, *specs.SiderolinkSpec]
}

// NewLinkStatusController initializes LinkStatusController.
func NewLinkStatusController[T siderolinkSpecWrapper](peers *siderolink.PeersPool) *qtransform.QController[T, *siderolinkres.LinkStatus] {
	var r T

	handler := &linkStatusHandler[T]{
		peers: peers,
	}

	return qtransform.NewQController(
		qtransform.Settings[T, *siderolinkres.LinkStatus]{
			Name: fmt.Sprintf("LinkStatusController[%s]", r.ResourceDefinition().Type),
			MapMetadataFunc: func(res T) *siderolinkres.LinkStatus {
				return siderolinkres.NewLinkStatus(res)
			},
			UnmapMetadataFunc: func(res *siderolinkres.LinkStatus) T {
				link, err := protobuf.CreateResource(r.ResourceDefinition().Type)
				if err != nil {
					panic(err)
				}

				*link.Metadata() = resource.NewMetadata(
					r.ResourceDefinition().DefaultNamespace,
					r.ResourceDefinition().Type,
					res.TypedSpec().Value.LinkId,
					resource.VersionUndefined,
				)

				return link.(T) //nolint:forcetypeassert,errcheck
			},
			TransformFunc:        handler.reconcileRunning,
			FinalizerRemovalFunc: handler.reconcileTearingDown,
		},
		qtransform.WithExtraMappedInput[*siderolinkres.GRPCTunnelConfig](
			qtransform.MapperSameID[T](),
		),
		qtransform.WithOutputKind(controller.OutputShared),
		qtransform.WithConcurrency(32),
	)
}

type linkStatusHandler[T siderolinkSpecWrapper] struct {
	peers *siderolink.PeersPool
}

func (handler *linkStatusHandler[T]) reconcileRunning(ctx context.Context, r controller.Reader, _ *zap.Logger, link T, linkStatus *siderolinkres.LinkStatus) error {
	grpcTunnelConfig, err := safe.ReaderGetByID[*siderolinkres.GRPCTunnelConfig](ctx, r, link.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if grpcTunnelConfig != nil {
		// If link.TypedSpec().Value.VirtualAddrport != "" then the machine is expected to use
		// a WireGuard over gRPC tunnel.
		//
		// If the existing link's tunnel mode does not match grpcTunnelConfig, we remove the
		// peer without recreating it. This forces the machine to reconnect after 4 minutes
		// and 35 seconds. During that reconnect, the machine will call the provision API again,
		// this time sending the correct gRPC tunnel mode, which updates the link resource.
		// The controller will then add the new peer.
		//
		// If grpcTunnelConfig is reverted within that 4 minute 35 second window, the controller
		// will recreate the peer using the old mode, and the machine will become reachable again.
		if err := handler.peers.Remove(ctx, siderolink.GetPeerID(linkStatus.TypedSpec().Value), link.Metadata()); err != nil {
			return err
		}

		if (link.TypedSpec().Value.VirtualAddrport != "") != grpcTunnelConfig.TypedSpec().Value.Enabled {
			return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("removed peer")
		}
	}

	if handler.needsPeerUpdate(linkStatus.TypedSpec().Value, link.TypedSpec().Value) {
		// remove the old peer to re-create it
		if err := handler.peers.Remove(ctx, siderolink.GetPeerID(linkStatus.TypedSpec().Value), link.Metadata()); err != nil {
			return err
		}
	}

	if err := handler.peers.Add(ctx, link.TypedSpec().Value, link.Metadata()); err != nil {
		return err
	}

	linkStatus.TypedSpec().Value.NodePublicKey = link.TypedSpec().Value.NodePublicKey
	linkStatus.TypedSpec().Value.NodeSubnet = link.TypedSpec().Value.NodeSubnet
	linkStatus.TypedSpec().Value.VirtualAddrport = link.TypedSpec().Value.VirtualAddrport

	linkStatus.TypedSpec().Value.LinkId = link.Metadata().ID()

	helpers.CopyAllAnnotations(link, linkStatus)

	return nil
}

func (handler *linkStatusHandler[T]) reconcileTearingDown(ctx context.Context, _ controller.Reader, _ *zap.Logger, link T) error {
	return handler.peers.Remove(ctx, siderolink.GetPeerID(link.TypedSpec().Value), link.Metadata())
}

func (handler *linkStatusHandler[T]) needsPeerUpdate(oldSpec, newSpec linkSpec) bool {
	if oldSpec.GetVirtualAddrport() != "" && oldSpec.GetVirtualAddrport() != newSpec.GetVirtualAddrport() {
		return true
	}

	if oldSpec.GetNodePublicKey() != "" && oldSpec.GetNodePublicKey() != newSpec.GetNodePublicKey() {
		return true
	}

	return false
}

type linkSpec interface {
	GetVirtualAddrport() string
	GetNodePublicKey() string
}
