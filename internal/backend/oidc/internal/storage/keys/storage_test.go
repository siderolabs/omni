// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package keys_test

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
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
		storage := runStorage(ctx, t, st)

		// a key is generated on startup
		synctest.Wait()

		firstKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)

		assert.EqualValues(t, jose.RS256, firstKey.SignatureAlgorithm())

		privateKey, ok := firstKey.Key().(*rsa.PrivateKey)
		require.True(t, ok)

		// the key set has exactly one key, whose public part matches the signing key
		keySet, err := storage.KeySet(ctx)
		require.NoError(t, err)

		require.Len(t, keySet, 1)
		assert.Equal(t, firstKey.ID(), keySet[0].ID())
		assert.Equal(t, privateKey.PublicKey, *keySet[0].Key().(*rsa.PublicKey)) //nolint:forcetypeassert,errcheck

		// seed an already-expired key: it is not trusted even before the rotation prunes it
		expiredKey := oidc.NewJWTPublicKey("expired-key")
		expiredKey.TypedSpec().Value.PublicKey = x509PublicKey(t, privateKey)
		expiredKey.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(-time.Hour))
		require.NoError(t, st.Create(ctx, expiredKey))

		_, err = storage.GetPublicKeyByID(ctx, "expired-key")
		assert.ErrorContains(t, err, "no longer valid")

		keySet, err = storage.KeySet(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{firstKey.ID()}, getKeyIDs(keySet))

		// after one rotation interval, a second key is generated, the signing key changes and the expired key is pruned
		time.Sleep(external.KeyRotationInterval)
		synctest.Wait()

		secondKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)

		assert.NotEqual(t, firstKey.ID(), secondKey.ID())

		keySet, err = storage.KeySet(ctx)
		require.NoError(t, err)

		assert.Equal(t, sorted([]string{firstKey.ID(), secondKey.ID()}), getKeyIDs(keySet))

		_, err = safe.StateGet[*oidc.JWTPublicKey](ctx, st, expiredKey.Metadata())
		assert.True(t, state.IsNotFoundError(err), "the expired key should have been destroyed")
	})
}

func TestDeletedKeyIsNotTrusted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storage := runStorage(ctx, t, st)

		synctest.Wait()

		// build a second key by advancing past one rotation, so a non-current key can be deleted
		firstKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)

		time.Sleep(external.KeyRotationInterval)
		synctest.Wait()

		secondKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)
		require.NotEqual(t, firstKey.ID(), secondKey.ID())

		// the old key still verifies
		_, err = storage.GetPublicKeyByID(ctx, firstKey.ID())
		require.NoError(t, err)

		// delete the old key: it stops verifying on the next lookup, the current key is unaffected
		require.NoError(t, st.Destroy(ctx, oidc.NewJWTPublicKey(firstKey.ID()).Metadata()))

		_, err = storage.GetPublicKeyByID(ctx, firstKey.ID())
		assert.ErrorContains(t, err, "key not found")

		_, err = storage.GetPublicKeyByID(ctx, secondKey.ID())
		assert.NoError(t, err)

		keySet, err := storage.KeySet(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{secondKey.ID()}, getKeyIDs(keySet))

		// deleting it again is a plain not found error
		err = st.Destroy(ctx, oidc.NewJWTPublicKey(firstKey.ID()).Metadata())
		assert.True(t, state.IsNotFoundError(err))
	})
}

func TestCurrentKeyReplacement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storage := runStorage(ctx, t, st)

		synctest.Wait()

		firstKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)

		// delete the current key: concurrent signing key requests must produce exactly one replacement
		require.NoError(t, st.Destroy(ctx, oidc.NewJWTPublicKey(firstKey.ID()).Metadata()))

		results := make([]op.SigningKey, 2)
		errCh := make(chan error, 2)

		for i := range results {
			go func() {
				var keyErr error

				results[i], keyErr = storage.GetCurrentSigningKey(ctx)

				errCh <- keyErr
			}()
		}

		require.NoError(t, <-errCh)
		require.NoError(t, <-errCh)

		// both requests returned the same replacement key, and it is immediately trusted
		assert.Equal(t, results[0].ID(), results[1].ID())
		assert.NotEqual(t, firstKey.ID(), results[0].ID())

		_, err = storage.GetPublicKeyByID(ctx, results[0].ID())
		assert.NoError(t, err)

		keySet, err := storage.KeySet(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{results[0].ID()}, getKeyIDs(keySet))
	})
}

func TestExpiredCurrentKeyIsReplaced(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storage := runStorage(ctx, t, st)

		synctest.Wait()

		firstKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)

		// expire the current key resource in place: signing must not continue with it
		_, err = safe.StateUpdateWithConflicts(ctx, st, oidc.NewJWTPublicKey(firstKey.ID()).Metadata(), func(res *oidc.JWTPublicKey) error {
			res.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(-time.Hour))

			return nil
		})
		require.NoError(t, err)

		replacement, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, firstKey.ID(), replacement.ID())
	})
}

func TestTearingDownKeyIsNotTrusted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storage := runStorage(ctx, t, st)

		synctest.Wait()

		currentKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)

		privateKey, ok := currentKey.Key().(*rsa.PrivateKey)
		require.True(t, ok)

		// seed a valid extra key, then tear it down without destroying it, e.g. an interrupted omnictl delete
		tearingDown := oidc.NewJWTPublicKey("tearing-down-key")
		tearingDown.TypedSpec().Value.PublicKey = x509PublicKey(t, privateKey)
		tearingDown.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(time.Hour))
		require.NoError(t, st.Create(ctx, tearingDown))

		_, err = storage.GetPublicKeyByID(ctx, "tearing-down-key")
		require.NoError(t, err)

		_, err = st.Teardown(ctx, tearingDown.Metadata())
		require.NoError(t, err)

		_, err = storage.GetPublicKeyByID(ctx, "tearing-down-key")
		assert.ErrorContains(t, err, "no longer valid")

		keySet, err := storage.KeySet(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{currentKey.ID()}, getKeyIDs(keySet))
	})
}

func TestMalformedKeyIsSkipped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storage := runStorage(ctx, t, st)

		synctest.Wait()

		currentKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)

		// seed a key whose stored public key bytes cannot be parsed
		malformed := oidc.NewJWTPublicKey("malformed-key")
		malformed.TypedSpec().Value.PublicKey = []byte("not a valid public key")
		malformed.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(time.Hour))
		require.NoError(t, st.Create(ctx, malformed))

		// the key set skips the malformed key instead of failing as a whole
		keySet, err := storage.KeySet(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{currentKey.ID()}, getKeyIDs(keySet))

		// the point lookup for the malformed key fails, naming it
		_, err = storage.GetPublicKeyByID(ctx, "malformed-key")
		assert.ErrorContains(t, err, "malformed-key")
	})
}

func TestCurrentKeyReplacementFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		failing := &failingState{CoreState: namespaced.NewState(inmem.Build)}
		st := state.WrapCore(failing)
		storage := runStorage(ctx, t, st)

		synctest.Wait()

		firstKey, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)

		require.NoError(t, st.Destroy(ctx, oidc.NewJWTPublicKey(firstKey.ID()).Metadata()))

		// storing the replacement fails: signing must fail instead of falling back to the deleted key
		failing.failCreate.Store(true)

		_, err = storage.GetCurrentSigningKey(ctx)
		assert.ErrorContains(t, err, "failure to replace the signing key")

		// once the state recovers, signing recovers with a fresh key
		failing.failCreate.Store(false)

		replacement, err := storage.GetCurrentSigningKey(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, firstKey.ID(), replacement.ID())
	})
}

func TestNoCurrentKey(t *testing.T) {
	ctx := t.Context()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	storage := keys.NewStorage(st, zaptest.NewLogger(t))

	// the refresher has not generated the startup key yet: signing fails and never generates lazily
	_, err := storage.GetCurrentSigningKey(ctx)
	assert.ErrorContains(t, err, "no current key")

	keys, err := safe.StateListAll[*oidc.JWTPublicKey](ctx, st)
	require.NoError(t, err)
	assert.Zero(t, keys.Len())
}

func runStorage(ctx context.Context, t *testing.T, st state.State) *keys.Storage {
	storage := keys.NewStorage(st, zaptest.NewLogger(t))

	errCh := make(chan error, 1)

	go func() { errCh <- storage.RunRefreshKey(ctx) }()

	t.Cleanup(func() { require.NoError(t, <-errCh) })

	return storage
}

func x509PublicKey(t *testing.T, privateKey *rsa.PrivateKey) []byte {
	t.Helper()

	return x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)
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

type failingState struct {
	state.CoreState

	failCreate atomic.Bool
}

func (s *failingState) Create(ctx context.Context, res resource.Resource, opts ...state.CreateOption) error {
	if s.failCreate.Load() {
		return errors.New("create failure")
	}

	return s.CoreState.Create(ctx, res, opts...)
}
