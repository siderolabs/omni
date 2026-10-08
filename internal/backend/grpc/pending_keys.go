// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"sync"
	"time"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

const pendingKeysSweepInterval = time.Minute

// pendingKeys holds the registered public keys until they are confirmed.
type pendingKeys struct {
	keys      map[string]*authres.PublicKey
	lastSweep time.Time
	mu        sync.Mutex
}

func (p *pendingKeys) add(key *authres.PublicKey) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	if now.Sub(p.lastSweep) > pendingKeysSweepInterval {
		for id, pending := range p.keys {
			if expired(pending, now) {
				delete(p.keys, id)
			}
		}

		p.lastSweep = now
	}

	if _, ok := p.keys[key.Metadata().ID()]; !ok {
		p.keys[key.Metadata().ID()] = key
	}
}

func (p *pendingKeys) get(id string) (*authres.PublicKey, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key, ok := p.keys[id]
	if ok && expired(key, time.Now()) {
		delete(p.keys, id)

		return nil, false
	}

	return key, ok
}

func (p *pendingKeys) remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.keys, id)
}

func expired(key *authres.PublicKey, now time.Time) bool {
	return key.TypedSpec().Value.GetExpiration().AsTime().Before(now)
}
