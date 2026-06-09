// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package keys_test

import (
	"crypto/rsa"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/go-jose/go-jose/v4"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage/keys"
)

func TestStorage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storage := keys.NewStorage(st, zaptest.NewLogger(t))

		errCh := make(chan error, 1)

		go func() { errCh <- storage.RunRefreshKey(ctx) }()

		t.Cleanup(func() { require.NoError(t, <-errCh) })

		// a key is generated on startup
		synctest.Wait()

		firstKey, err := storage.GetCurrentSigningKey()
		require.NoError(t, err)

		assert.EqualValues(t, jose.RS256, firstKey.SignatureAlgorithm())

		privateKey, ok := firstKey.Key().(*rsa.PrivateKey)
		require.True(t, ok)

		// the active key set has exactly one key, whose public part matches the signing key
		keySet, err := storage.KeySet()
		require.NoError(t, err)

		require.Len(t, keySet, 1)
		assert.Equal(t, firstKey.ID(), keySet[0].ID())
		assert.Equal(t, privateKey.PublicKey, *keySet[0].Key().(*rsa.PublicKey)) //nolint:forcetypeassert,errcheck

		// seed an already-expired key, it must be cleaned up on the next rotation
		expiredKey := oidc.NewJWTPublicKey("expired-key")
		expiredKey.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(-time.Hour))
		require.NoError(t, st.Create(ctx, expiredKey))

		// after one rotation interval, a second key is generated and the signing key changes
		time.Sleep(external.KeyRotationInterval)
		synctest.Wait()

		secondKey, err := storage.GetCurrentSigningKey()
		require.NoError(t, err)

		assert.NotEqual(t, firstKey.ID(), secondKey.ID())

		// the two generated keys are active, and the expired key has been pruned
		keySet, err = storage.KeySet()
		require.NoError(t, err)

		assert.Equal(t, sorted([]string{firstKey.ID(), secondKey.ID()}), getKeyIDs(keySet))

		_, err = safe.StateGet[*oidc.JWTPublicKey](ctx, st, expiredKey.Metadata())
		assert.True(t, state.IsNotFoundError(err), "the expired key should have been destroyed")
	})
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
