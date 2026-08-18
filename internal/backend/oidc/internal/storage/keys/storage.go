// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package keys

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	oidczitadel "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// Storage implements JWT key signing storage around resource state.
//
// The stored public key resources are the single source of truth for token verification:
// deleting a key resource invalidates all the tokens signed by it on their next use.
//
//nolint:govet
type Storage struct {
	st     state.State
	logger *zap.Logger

	mu         sync.Mutex
	currentKey op.SigningKey
}

// NewStorage creates a new Storage.
func NewStorage(st state.State, logger *zap.Logger) *Storage {
	return &Storage{
		st:     st,
		logger: logger,
	}
}

// KeySet returns the public keys of all stored valid signing keys.
//
// It will be called to get the current (public) keys, among others for the keys_endpoint or for validating access_tokens on the userinfo_endpoint, ...
func (s *Storage) KeySet(ctx context.Context) ([]op.Key, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	keys, err := safe.StateListAll[*oidc.JWTPublicKey](ctx, s.st)
	if err != nil {
		return nil, err
	}

	keySet := make([]op.Key, 0, keys.Len())

	for res := range keys.All() {
		key, valid, err := parseKey(res)
		if err != nil {
			// skip the key instead of failing the whole set, so a single broken key
			// does not take down the verification of the tokens signed by the others
			s.logger.Error("failed to parse a stored OIDC key", zap.String("key_id", res.Metadata().ID()), zap.Error(err))

			continue
		}

		if valid {
			keySet = append(keySet, key)
		}
	}

	return keySet, nil
}

// GetPublicKeyByID looks up the public key with the given ID.
func (s *Storage) GetPublicKeyByID(ctx context.Context, keyID string) (any, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	res, err := safe.StateGetByID[*oidc.JWTPublicKey](ctx, s.st, keyID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, fmt.Errorf("key not found, ID %q", keyID)
		}

		return nil, err
	}

	key, valid, err := parseKey(res)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the key with ID %q: %w", keyID, err)
	}

	if !valid {
		return nil, fmt.Errorf("key is no longer valid, ID %q", keyID)
	}

	return key.Key(), nil
}

// GetCurrentSigningKey returns the active and currently used signing key.
//
// If the key resource was deleted from the state, e.g., by an administrator to invalidate the tokens signed by it,
// a replacement key is generated and stored before it is returned.
func (s *Storage) GetCurrentSigningKey(ctx context.Context) (op.SigningKey, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentKey == nil {
		return nil, errors.New("no current key")
	}

	res, err := safe.StateGetByID[*oidc.JWTPublicKey](ctx, s.st, s.currentKey.ID())

	switch {
	case state.IsNotFoundError(err):
		// fall through to the replacement below
	case err != nil:
		return nil, err
	default:
		_, valid, parseErr := parseKey(res)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse the current key with ID %q: %w", s.currentKey.ID(), parseErr)
		}

		if valid {
			return s.currentKey, nil
		}
	}

	s.logger.Info("current OIDC signing key is no longer valid, replacing it", zap.String("key_id", s.currentKey.ID()))

	key, err := s.generateAndStoreKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failure to replace the signing key: %w", err)
	}

	s.currentKey = key

	return key, nil
}

// RunRefreshKey runs the key refresher in a loop.
func (s *Storage) RunRefreshKey(ctx context.Context) error {
	ctx = actor.MarkContextAsInternalActor(ctx)

	for ctx.Err() == nil {
		err := s.runRefreshKey(ctx)
		if err == nil {
			return nil
		}

		s.logger.Error("key refresher failed", zap.Error(err))

		// wait some time before restarting
		timer := time.NewTimer(10 * time.Second)

		select {
		case <-ctx.Done():
			timer.Stop()

			return nil
		case <-timer.C:
		}
	}

	return nil
}

func (s *Storage) runRefreshKey(ctx context.Context) error {
	ticker := time.NewTicker(external.KeyRotationInterval)
	defer ticker.Stop()

	for ctx.Err() == nil {
		// renew the key
		key, err := s.generateAndStoreKey(ctx)
		if err != nil {
			return err
		}

		if err = s.cleanupOldKeys(ctx); err != nil {
			return fmt.Errorf("failure to cleanup old keys: %w", err)
		}

		s.mu.Lock()
		s.currentKey = key
		s.mu.Unlock()

		s.logger.Info("new OIDC signing key generated", zap.String("key_id", key.ID()))

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}

	return nil
}

func (s *Storage) generateAndStoreKey(ctx context.Context) (*signingKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failure to generate the key: %w", err)
	}

	keyID := uuid.NewString()

	if err = s.storeKey(ctx, keyID, privateKey); err != nil {
		return nil, fmt.Errorf("failure to store the key: %w", err)
	}

	return &signingKey{
		id:        keyID,
		key:       privateKey,
		algorithm: jose.RS256,
	}, nil
}

func (s *Storage) storeKey(ctx context.Context, keyID string, privateKey *rsa.PrivateKey) error {
	key := oidc.NewJWTPublicKey(keyID)
	key.TypedSpec().Value.PublicKey = x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)

	maxTokenFiletime := s.MaxTokenLifetime()

	key.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(2*external.KeyRotationInterval + maxTokenFiletime))

	s.logger.Info(
		"generating new OIDC key",
		zap.String("key_id", key.Metadata().ID()),
		zap.Stringer("expiration", key.TypedSpec().Value.Expiration.AsTime()),
	)

	return s.st.Create(ctx, key)
}

func (s *Storage) cleanupOldKeys(ctx context.Context) error {
	keys, err := safe.StateListAll[*oidc.JWTPublicKey](ctx, s.st)
	if err != nil {
		return err
	}

	for key := range keys.All() {
		if time.Now().After(key.TypedSpec().Value.Expiration.AsTime()) {
			s.logger.Info(
				"destroying expired OIDC key",
				zap.String("key_id", key.Metadata().ID()),
				zap.Stringer("expiration", key.TypedSpec().Value.Expiration.AsTime()),
			)

			if err = s.st.Destroy(ctx, key.Metadata()); err != nil {
				return err
			}
		}
	}

	return nil
}

// MaxTokenLifetime returns the maximum lifetime of an access token.
func (s *Storage) MaxTokenLifetime() time.Duration {
	//goland:noinspection GoBoolExpressions
	if external.ServiceAccountTokenLifetime > external.OIDCTokenLifetime {
		return external.ServiceAccountTokenLifetime
	}

	return external.OIDCTokenLifetime
}

// parseKey converts a stored public key resource into a key set entry.
//
// The second return value reports whether the key is still valid for verifying tokens:
// an expired key or a key which is being destroyed must not be trusted.
func parseKey(res *oidc.JWTPublicKey) (op.Key, bool, error) {
	pKey, err := x509.ParsePKCS1PublicKey(res.TypedSpec().Value.PublicKey)
	if err != nil {
		return nil, false, err
	}

	valid := res.Metadata().Phase() == resource.PhaseRunning && time.Now().Before(res.TypedSpec().Value.Expiration.AsTime())

	return &publicKey{
		id:        res.Metadata().ID(),
		algorithm: jose.RS256,
		publicKey: pKey,
	}, valid, nil
}

//nolint:govet
type signingKey struct {
	id        string
	algorithm jose.SignatureAlgorithm
	key       *rsa.PrivateKey
}

func (s *signingKey) ID() string                                  { return s.id }
func (s *signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return s.algorithm }
func (s *signingKey) Key() any                                    { return s.key }

//nolint:govet
type publicKey struct {
	id        string
	algorithm jose.SignatureAlgorithm
	publicKey *rsa.PublicKey
}

func (s *publicKey) ID() string                         { return s.id }
func (s *publicKey) Algorithm() jose.SignatureAlgorithm { return s.algorithm }
func (s *publicKey) Use() string                        { return oidczitadel.KeyUseSignature }
func (s *publicKey) Key() any                           { return s.publicKey }
