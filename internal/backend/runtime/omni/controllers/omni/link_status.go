// Copyright (c) 2025 Sidero Labs, Inc.
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
		qtransform.WithOutputKind(controller.OutputShared),
		qtransform.WithConcurrency(32),
	)
}

type linkStatusHandler[T siderolinkSpecWrapper] struct {
	peers *siderolink.PeersPool
}

func (handler *linkStatusHandler[T]) reconcileRunning(ctx context.Context, _ controller.Reader, _ *zap.Logger, link T, linkStatus *siderolinkres.LinkStatus) error {
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
