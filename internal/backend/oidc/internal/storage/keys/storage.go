// Copyright (c) 2024 Sidero Labs, Inc.
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

	"github.com/benbjohnson/clock"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	oidczitadel "github.com/zitadel/oidc/pkg/oidc"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/square/go-jose.v2"

	"github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
)

// Storage implements JWT key signing storage around resource state.
type Storage struct {
	currentKey   *jose.JSONWebKey
	activeKeySet *jose.JSONWebKeySet
	logger       *zap.Logger
	st           state.State
	clock        clock.Clock
	lock         sync.Mutex
}

// NewStorage creates a new Storage.
func NewStorage(logger *zap.Logger, st state.State, clock clock.Clock) *Storage {
	return &Storage{
		logger: logger,
		st:     st,
		clock:  clock,
	}
}

// GetSigningKey implements the op.Storage interface.
//
// It will be called when creating the OpenID Provider.
func (s *Storage) GetSigningKey(ctx context.Context, keyCh chan<- jose.SigningKey) {
	for ctx.Err() == nil {
		err := s.keyRefresher(ctx, keyCh)
		if err == nil {
			return
		}

		s.logger.Error("key refresher failed", zap.Error(err))

		// wait some time before restarting
		select {
		case <-ctx.Done():
			return
		case <-s.clock.After(10 * time.Second):
		}
	}
}

// GetKeySet implements the op.Storage interface.
//
// It will be called to get the current (public) keys, among others for the keys_endpoint or for validating access_tokens on the userinfo_endpoint, ...
func (s *Storage) GetKeySet(context.Context) (*jose.JSONWebKeySet, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.activeKeySet != nil {
		return s.activeKeySet, nil
	}

	return nil, errors.New("no active key set")
}

// GetPublicKeyByID looks up the public key with the given ID.
func (s *Storage) GetPublicKeyByID(keyID string) (any, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.activeKeySet == nil {
		return nil, errors.New("no active key set")
	}

	for _, key := range s.activeKeySet.Keys {
		if key.KeyID == keyID {
			return key.Key, nil
		}
	}

	return nil, fmt.Errorf("key not found, ID %q", keyID)
}

// GetCurrentSigningKey returns the active and currently used signing key.
func (s *Storage) GetCurrentSigningKey() (*jose.JSONWebKey, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.currentKey != nil {
		return s.currentKey, nil
	}

	return nil, errors.New("no current key")
}

func (s *Storage) keyRefresher(ctx context.Context, keyCh chan<- jose.SigningKey) error {
	ticker := s.clock.Ticker(external.KeyRotationInterval)
	defer ticker.Stop()

	for {
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

		jsonWebKey := jose.JSONWebKey{
			KeyID:     keyID,
			Key:       privateKey,
			Algorithm: string(jose.RS256),
		}

		s.lock.Lock()
		s.currentKey = &jsonWebKey
		s.lock.Unlock()

		// send the new signing key to be used by OIDC
		select {
		case <-ctx.Done():
			return nil
		case keyCh <- jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       jsonWebKey,
		}:
		}

		s.logger.Info("new OIDC signing key generated", zap.String("key_id", keyID))

		// wait for key rotation interval
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
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

	newKeySet := &jose.JSONWebKeySet{}

	for iter := keys.Iterator(); iter.Next(); {
		key := iter.Value()

		if s.clock.Now().After(key.TypedSpec().Value.Expiration.AsTime()) {
			s.logger.Info("destroying expired OIDC key",
				zap.String("key_id", key.Metadata().ID()),
				zap.Stringer("expiration", key.TypedSpec().Value.Expiration.AsTime()),
			)

			if err = s.st.Destroy(ctx, key.Metadata()); err != nil {
				return err
			}
		} else {
			publicKey, err := x509.ParsePKCS1PublicKey(key.TypedSpec().Value.PublicKey)
			if err != nil {
				return err
			}

			newKeySet.Keys = append(newKeySet.Keys, jose.JSONWebKey{
				KeyID:     key.Metadata().ID(),
				Algorithm: string(jose.RS256),
				Use:       oidczitadel.KeyUseSignature,
				Key:       publicKey,
			})
		}
	}

	s.logger.Info("active OIDC public signing keys",
		zap.Strings("key_ids",
			xslices.Map(newKeySet.Keys, func(key jose.JSONWebKey) string {
				return key.KeyID
			}),
		),
	)

	s.lock.Lock()
	defer s.lock.Unlock()

	s.activeKeySet = newKeySet

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
