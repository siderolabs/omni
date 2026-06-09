// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	authctrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

const keyPrunerInterval = 10 * time.Minute

// newExpiringKey builds a confirmed public key that expires after the given duration relative to the (virtual) now.
func newExpiringKey(id string, expiresIn time.Duration) *authres.PublicKey {
	publicKey := authres.NewPublicKey(id)
	publicKey.TypedSpec().Value.Confirmed = true
	publicKey.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(expiresIn))

	return publicKey
}

// TestRemoveExpiredKey verifies that the key pruner removes keys past their expiration while leaving the rest alone.
func TestRemoveExpiredKey(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				require.NoError(t, tc.Runtime.RegisterController(omnictrl.NewKeyPrunerController(keyPrunerInterval)))
			},
			func(ctx context.Context, tc testutils.TestContext) {
				owner := new(omnictrl.KeyPrunerController{}).Name()

				expiredKey := newExpiringKey("expired", time.Hour)
				liveKey := newExpiringKey("live", 24*time.Hour)

				require.NoError(t, tc.State.Create(ctx, expiredKey, state.WithCreateOwner(owner)))
				require.NoError(t, tc.State.Create(ctx, liveKey, state.WithCreateOwner(owner)))

				// advance past the first key's expiration but not the second's
				time.Sleep(2 * time.Hour)
				synctest.Wait()

				// the expired key must be pruned
				_, err := tc.State.Get(ctx, expiredKey.Metadata())
				require.Truef(t, state.IsNotFoundError(err), "expired key should have been pruned, got err: %v", err)

				// the non-expired key must be left untouched (still present and running, not torn down)
				liveRes, err := tc.State.Get(ctx, liveKey.Metadata())
				require.NoError(t, err)
				require.Equal(t, resource.PhaseRunning, liveRes.Metadata().Phase())
			},
		)
	})
}

// TestRemoveExpiredKeyWithCleanupFinalizer verifies that the key pruner removes an expired public key even when
// PublicKeyCleanupController has put a finalizer on it, by tearing the key down so the finalizer is removed before destroying it.
func TestRemoveExpiredKeyWithCleanupFinalizer(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				require.NoError(t, tc.Runtime.RegisterController(omnictrl.NewKeyPrunerController(keyPrunerInterval)))
				require.NoError(t, tc.Runtime.RegisterController(authctrl.NewPublicKeyCleanupController()))
			},
			func(ctx context.Context, tc testutils.TestContext) {
				owner := new(omnictrl.KeyPrunerController{}).Name()
				finalizer := authctrl.NewPublicKeyCleanupController().Name()

				publicKey := newExpiringKey("expired", time.Hour)
				require.NoError(t, tc.State.Create(ctx, publicKey, state.WithCreateOwner(owner)))

				// wait until the cleanup controller puts its finalizer on the key
				rtestutils.AssertResource(ctx, t, tc.State, publicKey.Metadata().ID(), func(r *authres.PublicKey, a *assert.Assertions) {
					a.True(r.Metadata().Finalizers().Has(finalizer))
				})

				// advance past expiration, allowing the teardown and the subsequent destroy to happen
				time.Sleep(2 * time.Hour)
				synctest.Wait()

				// the expired key must be pruned
				_, err := tc.State.Get(ctx, publicKey.Metadata())
				require.Truef(t, state.IsNotFoundError(err), "expired key should have been pruned, got err: %v", err)
			},
		)
	})
}

// TestRemoveExpiredKeyWithEmptyOwner verifies that the key pruner removes an expired public key that is not owned by
// the pruner and carries a cleanup finalizer, as is the case for service account keys. The pruner cannot assume itself
// to be the owner, and must tear the key down to drop the finalizer before destroying it.
func TestRemoveExpiredKeyWithEmptyOwner(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				require.NoError(t, tc.Runtime.RegisterController(omnictrl.NewKeyPrunerController(keyPrunerInterval)))
				require.NoError(t, tc.Runtime.RegisterController(authctrl.NewPublicKeyCleanupController()))
			},
			func(ctx context.Context, tc testutils.TestContext) {
				finalizer := authctrl.NewPublicKeyCleanupController().Name()

				// no WithCreateOwner, mirroring how service account keys are created
				publicKey := newExpiringKey("expired", time.Hour)
				require.NoError(t, tc.State.Create(ctx, publicKey))

				// wait until the cleanup controller puts its finalizer on the key
				rtestutils.AssertResource(ctx, t, tc.State, publicKey.Metadata().ID(), func(r *authres.PublicKey, a *assert.Assertions) {
					a.True(r.Metadata().Finalizers().Has(finalizer))
				})

				// advance past expiration
				time.Sleep(2 * time.Hour)
				synctest.Wait()

				// the expired key must be pruned
				_, err := tc.State.Get(ctx, publicKey.Metadata())
				require.Truef(t, state.IsNotFoundError(err), "expired key should have been pruned, got err: %v", err)
			},
		)
	})
}
