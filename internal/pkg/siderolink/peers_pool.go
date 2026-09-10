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
//
// PeerID matches the public key of the Wireguard peer, the same key Wireguard uses
// to identify the peer.
//
// This is used to deduplicate peers in the pool and to remove them when they are no longer needed.
type PeerID string

type peer struct {
	link   *specs.SiderolinkSpec
	owners map[ownerID]struct{}
}

type ownerID struct {
	id           string
	resourceType string
	namespace    string
}

func (o ownerID) String() string {
	return o.resourceType + "/" + o.namespace + "/" + o.id
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
	GetNodePublicKey() string
},
) PeerID {
	return PeerID(spec.GetNodePublicKey())
}

// Add a wireguard peer.
// if the peer exists, only the reference counter is increased.
func (pool *PeersPool) Add(ctx context.Context, spec *specs.SiderolinkSpec, owner *resource.Metadata) error {
	pool.peersMu.Lock()
	defer pool.peersMu.Unlock()

	oid := getOwnerID(owner)

	if err := pool.wgHandler.PeerEvent(ctx, spec, false); err != nil {
		return err
	}

	if existing, ok := pool.peers[GetPeerID(spec)]; ok {
		_, alreadyExists := existing.owners[oid]

		if !alreadyExists {
			pool.logger.Info(
				"reference existing wireguard peer",
				zap.String("public_key", spec.NodePublicKey),
				zap.String("new_owner", oid.String()),
			)
		}

		existing.owners[oid] = struct{}{}

		return nil
	}

	pool.peers[GetPeerID(spec)] = peer{
		link:   spec,
		owners: map[ownerID]struct{}{oid: {}},
	}

	pool.logger.Info("added wireguard peer", zap.String("public_key", spec.NodePublicKey), zap.String("owner", oid.String()))

	return nil
}

// Remove wireguard peer if the ref counter is 0.
func (pool *PeersPool) Remove(ctx context.Context, peerID PeerID, owner *resource.Metadata) error {
	pool.peersMu.Lock()
	defer pool.peersMu.Unlock()

	oid := getOwnerID(owner)

	existing, ok := pool.peers[peerID]
	if !ok {
		return nil
	}

	_, ownerExists := existing.owners[oid]
	if !ownerExists {
		pool.logger.Warn("owner does not exist for wireguard peer", zap.String("public_key", string(peerID)), zap.String("owner", oid.String()))
	}

	delete(existing.owners, oid)

	if len(existing.owners) > 0 {
		return nil
	}

	if err := pool.wgHandler.PeerEvent(ctx, existing.link, true); err != nil {
		return err
	}

	delete(pool.peers, peerID)

	pool.logger.Info("removing wireguard peer", zap.String("public_key", string(peerID)), zap.String("owner", oid.String()))

	return nil
}
