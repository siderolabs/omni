// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

// PeerID describes the ID which is used to uniquely identify the peers in the pool.
type PeerID struct {
	key             string
	virtualAddrport string
}

type peer struct {
	link   *specs.SiderolinkSpec
	owners map[ownerID]struct{}
}

type ownerID struct {
	id           string
	resourceType string
	namespace    string
}

func getOwnerID(md *resource.Metadata) ownerID {
	return ownerID{
		id:           md.ID(),
		resourceType: md.Type(),
		namespace:    md.Namespace(),
	}
}

// NewPeersPool creates a new PeersPool.
func NewPeersPool(logger *zap.Logger, wgHandler WireguardHandler) *PeersPool {
	return &PeersPool{
		peers:     map[PeerID]peer{},
		wgHandler: wgHandler,
		logger:    logger,
	}
}

// PeersPool keeps track of the wireguard peers
// it has reference counter and deduplicates peer creation.
type PeersPool struct {
	logger    *zap.Logger
	wgHandler WireguardHandler
	peers     map[PeerID]peer
	peersMu   sync.Mutex
}

// GetPeerID returns the peer id.
func GetPeerID(spec interface {
	GetVirtualAddrport() string
	GetNodePublicKey() string
},
) PeerID {
	return PeerID{
		virtualAddrport: spec.GetVirtualAddrport(),
		key:             spec.GetNodePublicKey(),
	}
}

// Add a wireguard peer.
// if the peer exists, only the reference counter is increased.
func (pool *PeersPool) Add(ctx context.Context, spec *specs.SiderolinkSpec, owner *resource.Metadata) error {
	pool.peersMu.Lock()
	defer pool.peersMu.Unlock()

	oid := getOwnerID(owner)

	if existing, ok := pool.peers[GetPeerID(spec)]; ok {
		existing.owners[oid] = struct{}{}

		pool.logger.Info("reference existing wireguard peer", zap.String("public_key", spec.NodePublicKey), zap.String("owner", owner.String()))

		return nil
	}

	if err := pool.wgHandler.PeerEvent(ctx, spec, false); err != nil {
		return err
	}

	pool.peers[GetPeerID(spec)] = peer{
		link:   spec,
		owners: map[ownerID]struct{}{oid: {}},
	}

	pool.logger.Info("added wireguard peer", zap.String("public_key", spec.NodePublicKey), zap.String("owner", owner.String()))

	return nil
}

// Remove wireguard peer if the ref counter is 0.
func (pool *PeersPool) Remove(ctx context.Context, peerID PeerID, owner *resource.Metadata) error {
	pool.peersMu.Lock()
	defer pool.peersMu.Unlock()

	existing, ok := pool.peers[peerID]
	if !ok {
		return nil
	}

	delete(existing.owners, getOwnerID(owner))

	if len(existing.owners) > 0 {
		return nil
	}

	if err := pool.wgHandler.PeerEvent(ctx, existing.link, true); err != nil {
		return err
	}

	delete(pool.peers, peerID)

	pool.logger.Info("removing wireguard peer", zap.String("public_key", peerID.key), zap.String("owner", owner.String()))

	return nil
}
