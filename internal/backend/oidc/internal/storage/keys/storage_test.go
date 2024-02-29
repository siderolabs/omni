// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package keys_test

import (
	"context"
	"crypto/rsa"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"gopkg.in/square/go-jose.v2"

	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage/keys"
)

func TestStorage(t *testing.T) {
	var wg sync.WaitGroup

	t.Cleanup(wg.Wait)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	clock := clock.NewMock()

	storage := keys.NewStorage(zaptest.NewLogger(t), st, clock)

	keyCh := make(chan jose.SigningKey, 1)

	// run the key rotation loop
	wg.Add(1)

	go func() {
		defer wg.Done()

		storage.GetSigningKey(ctx, keyCh)
	}()

	// the first key should be generated immediately
	var key jose.SigningKey

	select {
	case key = <-keyCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for key")
	}

	assert.Equal(t, jose.RS256, key.Algorithm)

	privateKey, ok := key.Key.(jose.JSONWebKey)
	require.True(t, ok)

	// get the active set of public keys, it should have exactly one key
	// public key should match a private key
	keySet, err := storage.GetKeySet(ctx)
	require.NoError(t, err)

	assert.Len(t, keySet.Keys, 1)
	assert.Equal(t, keySet.Keys[0].KeyID, privateKey.KeyID)
	assert.Equal(t, *keySet.Keys[0].Key.(*rsa.PublicKey), privateKey.Key.(*rsa.PrivateKey).PublicKey) //nolint:forcetypeassert

	expectedPublicKeyIDs := []string{privateKey.KeyID}

	// advance time and generate one more key - will trigger one timer tick
	clock.Add(external.KeyRotationInterval)

	select {
	case key = <-keyCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for key")
	}

	assert.Equal(t, jose.RS256, key.Algorithm)

	privateKey, ok = key.Key.(jose.JSONWebKey)
	require.True(t, ok)

	expectedPublicKeyIDs = append(expectedPublicKeyIDs, privateKey.KeyID)

	// get the active set of public keys, it should have two keys now
	keySet, err = storage.GetKeySet(ctx)
	require.NoError(t, err)

	assert.Len(t, keySet.Keys, 2)
	assert.Equal(t, sorted(expectedPublicKeyIDs), getKeyIDs(keySet))

	// advance the time even more so that the first key expires
	// this triggers at least one timer tick, but the exact number varies due to the mocked clock
	clock.Add(external.KeyRotationInterval + storage.MaxTokenLifetime() + time.Second)

	expectedKeyToExpire := expectedPublicKeyIDs[0]
	expectedPublicKeyIDs = expectedPublicKeyIDs[1:]

	// consume the newly generated keys until we reach the last one,
	// which will cause the first (oldest) key to be removed due to expiration
	for {
		select {
		case key = <-keyCh:
		case <-time.After(10 * time.Second):
			t.Fatal("timeout waiting for key")
		}

		assert.Equal(t, jose.RS256, key.Algorithm)

		privateKey, ok = key.Key.(jose.JSONWebKey)
		require.True(t, ok)

		expectedPublicKeyIDs = append(expectedPublicKeyIDs, privateKey.KeyID)

		// get the active set of public keys
		keySet, err = storage.GetKeySet(ctx)
		require.NoError(t, err)

		keyIDs := getKeyIDs(keySet)

		if !slices.Contains(keyIDs, expectedKeyToExpire) {
			t.Logf("first key is removed, stop receiving")

			break
		}
	}

	// get the active set of public keys - it should have at least two keys:
	// - the second key from the previous set, because it shouldn't be expired yet.
	// - the newly generated keys due to the tick(s) happened above.
	keySet, err = storage.GetKeySet(ctx)
	require.NoError(t, err)

	keyIDs := getKeyIDs(keySet)

	assert.GreaterOrEqual(t, len(keyIDs), 2, "at least two keys should be present")
	assert.Equal(t, sorted(expectedPublicKeyIDs), keyIDs)
}

func getKeyIDs(keySet *jose.JSONWebKeySet) []string {
	ids := xslices.Map(keySet.Keys, func(k jose.JSONWebKey) string { return k.KeyID })

	return sorted(ids)
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)

	return s
}
