// Copyright (c) 2025 Sidero Labs, Inc.
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
	"slices"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
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
//nolint:govet
type Storage struct {
	st     state.State
	clock  clock.Clock
	logger *zap.Logger

	mu           sync.Mutex
	currentKey   op.SigningKey
	activeKeySet []op.Key
}

// NewStorage creates a new Storage.
func NewStorage(st state.State, clock clock.Clock, logger *zap.Logger) *Storage {
	result := &Storage{
		st:     st,
		clock:  clock,
		logger: logger,
	}

	return result
}

// KeySet implements the op.Storage interface.
//
// It will be called to get the current (public) keys, among others for the keys_endpoint or for validating access_tokens on the userinfo_endpoint, ...
func (s *Storage) KeySet() ([]op.Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeKeySet != nil {
		return s.activeKeySet, nil
	}

	return nil, errors.New("no active key set")
}

// GetPublicKeyByID looks up the public key with the given ID.
func (s *Storage) GetPublicKeyByID(keyID string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeKeySet == nil {
		return nil, errors.New("no active key set")
	}

	idx := slices.IndexFunc(s.activeKeySet, func(key op.Key) bool { return key.ID() == keyID })
	if idx == -1 {
		return nil, fmt.Errorf("key not found, ID %q", keyID)
	}

	return s.activeKeySet[idx].Key(), nil
}

// GetCurrentSigningKey returns the active and currently used signing key.
func (s *Storage) GetCurrentSigningKey() (op.SigningKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentKey == nil {
		return nil, errors.New("no current key")
	}

	return s.currentKey, nil
}

// Options is a set of options for the key refresher.
type Options struct {
	keyCh chan<- op.SigningKey
}

// Opts is a functional option for the key refresher.
type Opts func(*Options)

// WithKeyCh sets the channel to send the generated key to.
func WithKeyCh(keyCh chan<- op.SigningKey) Opts {
	return func(o *Options) {
		o.keyCh = keyCh
	}
}

// RunRefreshKey runs the key refresher in a loop.
func (s *Storage) RunRefreshKey(ctx context.Context, opts ...Opts) error {
	ctx = actor.MarkContextAsInternalActor(ctx)

	var options Options

	for _, opt := range opts {
		opt(&options)
	}

	for ctx.Err() == nil {
		err := s.runRefreshKey(ctx, options.keyCh)
		if err == nil {
			return nil
		}

		s.logger.Error("key refresher failed", zap.Error(err))

		// wait some time before restarting
		select {
		case <-ctx.Done():
			return nil
		case <-s.clock.After(10 * time.Second):
		}
	}

	return nil
}

func (s *Storage) runRefreshKey(ctx context.Context, ch chan<- op.SigningKey) error {
	ticker := s.clock.Ticker(external.KeyRotationInterval)
	defer ticker.Stop()

	for ctx.Err() == nil {
		// renew the key
		privateKey, err := s.generateKey()
		if err != nil {
			return fmt.Errorf("failure to generate the key: %w", err)
		}

		keyID := uuid.NewString()

		if err = s.storeKey(ctx, keyID, privateKey); err != nil {
			return fmt.Errorf("failure to store the key: %w", err)
		}

		if err = s.cleanupOldKeys(ctx); err != nil {
			return fmt.Errorf("failure to cleanup old keys: %w", err)
		}

		key := &signingKey{
			id:        keyID,
			key:       privateKey,
			algorithm: jose.RS256,
		}

		s.mu.Lock()
		s.currentKey = key
		s.mu.Unlock()

		s.logger.Info("new OIDC signing key generated", zap.String("key_id", keyID))

		if ch != nil {
			select {
			case ch <- key:
			case <-ctx.Done():
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}

	return nil
}

func (s *Storage) generateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

func (s *Storage) storeKey(ctx context.Context, keyID string, privateKey *rsa.PrivateKey) error {
	key := oidc.NewJWTPublicKey(oidc.NamespaceName, keyID)
	key.TypedSpec().Value.PublicKey = x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)

	maxTokenFiletime := s.MaxTokenLifetime()

	key.TypedSpec().Value.Expiration = timestamppb.New(s.clock.Now().Add(2*external.KeyRotationInterval + maxTokenFiletime))

	s.logger.Info("generating new OIDC key",
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

	newKeySet := make([]op.Key, 0, keys.Len())

	for key := range keys.All() {
		if s.clock.Now().After(key.TypedSpec().Value.Expiration.AsTime()) {
			s.logger.Info("destroying expired OIDC key",
				zap.String("key_id", key.Metadata().ID()),
				zap.Stringer("expiration", key.TypedSpec().Value.Expiration.AsTime()),
			)

			if err = s.st.Destroy(ctx, key.Metadata()); err != nil {
				return err
			}
		} else {
			pKey, err := x509.ParsePKCS1PublicKey(key.TypedSpec().Value.PublicKey)
			if err != nil {
				return err
			}

			newKeySet = append(newKeySet, &publicKey{
				id:        key.Metadata().ID(),
				algorithm: jose.RS256,
				publicKey: pKey,
			})
		}
	}

	s.logger.Info("active OIDC public signing keys",
		zap.Strings("key_ids", xslices.Map(newKeySet, func(key op.Key) string { return key.ID() })),
	)

	s.mu.Lock()
	s.activeKeySet = newKeySet
	s.mu.Unlock()

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
