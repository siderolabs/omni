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
	"github.com/go-jose/go-jose/v4"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage/keys"
)

func TestStorage(t *testing.T) {
	errCh := make(chan error, 1)

	t.Cleanup(func() { require.NoError(t, <-errCh) })

	var wg sync.WaitGroup

	t.Cleanup(wg.Wait)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	clck := clock.NewMock()

	storage := keys.NewStorage(st, clck, zaptest.NewLogger(t))

	wg.Add(1)

	keyCh := make(chan op.SigningKey, 1)

	go func() {
		defer wg.Done()

		errCh <- storage.RunRefreshKey(ctx, keys.WithKeyCh(keyCh))
	}()

	var signingKey op.SigningKey

	select {
	case signingKey = <-keyCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for key")
	}

	gotKey, err := storage.GetCurrentSigningKey()
	require.NoError(t, err)
	assert.Equal(t, signingKey.ID(), gotKey.ID())
	assert.EqualValues(t, jose.RS256, signingKey.SignatureAlgorithm())

	privateKey, ok := signingKey.Key().(*rsa.PrivateKey)
	require.True(t, ok)

	// get the active set of public keys, it should have exactly one key
	// public key should match a private key
	keySet, err := storage.KeySet()
	require.NoError(t, err)

	assert.Len(t, keySet, 1)
	assert.Equal(t, keySet[0].ID(), signingKey.ID())
	assert.Equal(t, *keySet[0].Key().(*rsa.PublicKey), privateKey.PublicKey) //nolint:forcetypeassert

	expectedPublicKeyIDs := []string{signingKey.ID()}

	// advance time and generate one more key - will trigger one timer tick
	clck.Add(external.KeyRotationInterval)

	select {
	case signingKey = <-keyCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for key")
	}

	assert.NotEqual(t, signingKey.ID(), gotKey.ID())
	assert.EqualValues(t, jose.RS256, signingKey.SignatureAlgorithm())

	_, ok = signingKey.Key().(*rsa.PrivateKey)
	require.True(t, ok)

	expectedPublicKeyIDs = append(expectedPublicKeyIDs, signingKey.ID())

	// get the active set of public keys, it should have two keys now
	keySet, err = storage.KeySet()
	require.NoError(t, err)

	assert.Len(t, keySet, 2)
	assert.Equal(t, sorted(expectedPublicKeyIDs), getKeyIDs(keySet))

	// advance the time even more so that the first key expires
	// this triggers at least one timer tick, but the exact number varies due to the mocked clock
	clck.Add(external.KeyRotationInterval + storage.MaxTokenLifetime() + time.Second)

	expectedKeyToExpire := expectedPublicKeyIDs[0]
	expectedPublicKeyIDs = expectedPublicKeyIDs[1:]

	// consume the newly generated keys until we reach the last one,
	// which will cause the first (oldest) key to be removed due to expiration
	for {
		select {
		case signingKey = <-keyCh:
		case <-time.After(10 * time.Second):
			t.Fatal("timeout waiting for key")
		}

		assert.EqualValues(t, jose.RS256, signingKey.SignatureAlgorithm())

		_, ok = signingKey.Key().(*rsa.PrivateKey)
		require.True(t, ok)

		expectedPublicKeyIDs = append(expectedPublicKeyIDs, signingKey.ID())

		// get the active set of public keys
		keySet, err = storage.KeySet()
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
	keySet, err = storage.KeySet()
	require.NoError(t, err)

	keyIDs := getKeyIDs(keySet)

	assert.GreaterOrEqual(t, len(keyIDs), 2, "at least two keys should be present")
	assert.Equal(t, sorted(expectedPublicKeyIDs), keyIDs)
}

func getKeyIDs(keySet []op.Key) []string {
	ids := xslices.Map(keySet, func(k op.Key) string { return k.ID() })

	return sorted(ids)
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)

	return s
}
