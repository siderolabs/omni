// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/interceptor"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

var noopHandler = func(_ context.Context, _ any) (any, error) {
	return nil, nil //nolint:nilnil
}

// mockServerStream is a minimal grpc.ServerStream implementation for testing.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context //nolint:containedctx
}

func (m *mockServerStream) Context() context.Context     { return m.ctx }
func (m *mockServerStream) SendMsg(any) error            { return nil }
func (m *mockServerStream) RecvMsg(any) error            { return nil }
func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}

//nolint:maintidx
func TestActivity(t *testing.T) {
	t.Parallel()

	t.Run("no identity in context", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)
		activity := interceptor.NewActivity(st, logger)

		ctx := t.Context()

		_, err := activity.Unary()(ctx, nil, nil, noopHandler)
		require.NoError(t, err)

		list, err := st.List(ctx, authres.NewIdentityLastActive("").Metadata())
		require.NoError(t, err)
		assert.Empty(t, list.Items)
	})

	for _, tc := range []struct {
		ctxSetup func(ctx context.Context, identity string) context.Context
		name     string
		identity string
	}{
		{
			name:     "signature identity",
			identity: "user@example.com",
			ctxSetup: func(ctx context.Context, identity string) context.Context {
				return ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: identity})
			},
		},
		{
			name:     "verified email identity",
			identity: "saml-user@example.com",
			ctxSetup: func(ctx context.Context, identity string) context.Context {
				return ctxstore.WithValue(ctx, auth.VerifiedEmailContextKey{Email: identity})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			st := state.WrapCore(namespaced.NewState(inmem.Build))
			logger := zaptest.NewLogger(t)
			activity := interceptor.NewActivity(st, logger)

			require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive(tc.identity)))

			ctx := tc.ctxSetup(t.Context(), tc.identity)

			_, err := activity.Unary()(ctx, nil, nil, noopHandler)
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, st, tc.identity, func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
				asrt.WithinDuration(time.Now(), res.TypedSpec().Value.LastActive.AsTime(), 5*time.Second)
			})
		})
	}

	t.Run("signature identity takes precedence over verified email", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)
		activity := interceptor.NewActivity(st, logger)

		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("sig-user@example.com")))
		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("email-user@example.com")))

		ctx := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "sig-user@example.com"})
		ctx = ctxstore.WithValue(ctx, auth.VerifiedEmailContextKey{Email: "email-user@example.com"})

		_, err := activity.Unary()(ctx, nil, nil, noopHandler)
		require.NoError(t, err)

		emailUser, err := safe.StateGetByID[*authres.IdentityLastActive](ctx, st, "email-user@example.com")
		require.NoError(t, err)

		emailUserVersion := emailUser.Metadata().Version()

		// Wait for the async write to sig-user to complete; once it lands we know the goroutine has finished,
		// so if email-user's version is unchanged it proves only sig-user was tracked.
		rtestutils.AssertResource(ctx, t, st, "sig-user@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.WithinDuration(time.Now(), res.TypedSpec().Value.LastActive.AsTime(), 5*time.Second)
		})

		rtestutils.AssertResource(ctx, t, st, "email-user@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.True(res.Metadata().Version().Equal(emailUserVersion), "email-user should not have been updated")
		})
	})

	t.Run("debounce prevents repeated writes", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)
		activity := interceptor.NewActivity(st, logger)

		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("debounce-user@example.com")))

		ctx := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "debounce-user@example.com"})

		_, err := activity.Unary()(ctx, nil, nil, noopHandler)
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, st, "debounce-user@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.NotNil(res.TypedSpec().Value.LastActive)
		})

		res, err := safe.StateGetByID[*authres.IdentityLastActive](ctx, st, "debounce-user@example.com")
		require.NoError(t, err)

		firstVersion := res.Metadata().Version()

		// Second call within the debounce interval should not update.
		_, err = activity.Unary()(ctx, nil, nil, noopHandler)
		require.NoError(t, err)

		// Trigger a write for a different identity to use as a synchronization point:
		// once this write lands, any debounce-user write (if it were fired) would also have landed.
		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("sync-user@example.com")))

		syncCtx := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "sync-user@example.com"})

		_, err = activity.Unary()(syncCtx, nil, nil, noopHandler)
		require.NoError(t, err)

		rtestutils.AssertResource(syncCtx, t, st, "sync-user@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.NotNil(res.TypedSpec().Value.LastActive)
		})

		rtestutils.AssertResource(ctx, t, st, "debounce-user@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.True(res.Metadata().Version().Equal(firstVersion), "debounced user should not have been updated")
		})
	})

	t.Run("different identities tracked independently", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)
		activity := interceptor.NewActivity(st, logger)

		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("alice@example.com")))
		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("bob@example.com")))

		ctxA := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "alice@example.com"})
		ctxB := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "bob@example.com"})

		_, err := activity.Unary()(ctxA, nil, nil, noopHandler)
		require.NoError(t, err)

		_, err = activity.Unary()(ctxB, nil, nil, noopHandler)
		require.NoError(t, err)

		rtestutils.AssertResource(t.Context(), t, st, "alice@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.NotNil(res.TypedSpec().Value.LastActive)
		})
		rtestutils.AssertResource(t.Context(), t, st, "bob@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.NotNil(res.TypedSpec().Value.LastActive)
		})
	})

	t.Run("stream interceptor tracks activity", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)
		activity := interceptor.NewActivity(st, logger)

		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("stream-user@example.com")))

		ctx := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "stream-user@example.com"})
		stream := &mockServerStream{ctx: ctx}

		err := activity.Stream()(nil, stream, nil, func(_ any, _ grpc.ServerStream) error {
			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, st, "stream-user@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.WithinDuration(time.Now(), res.TypedSpec().Value.LastActive.AsTime(), 5*time.Second)
		})
	})

	t.Run("fingerprint tracking creates PublicKeyLastActive", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)
		activity := interceptor.NewActivity(st, logger)

		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("fp-user@example.com")))

		ctx := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "fp-user@example.com"})
		ctx = ctxstore.WithValue(ctx, auth.FingerprintContextKey{Fingerprint: "abc123fingerprint"})

		_, err := activity.Unary()(ctx, nil, nil, noopHandler)
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, st, "abc123fingerprint", func(res *authres.PublicKeyLastActive, asrt *assert.Assertions) {
			asrt.WithinDuration(time.Now(), res.TypedSpec().Value.LastUsed.AsTime(), 5*time.Second)

			identity, ok := res.Metadata().Labels().Get(authres.LabelIdentity)
			asrt.True(ok)
			asrt.Equal("fp-user@example.com", identity)
		})
	})

	t.Run("fingerprint debounced independently from identity", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)
		activity := interceptor.NewActivity(st, logger)

		require.NoError(t, st.Create(t.Context(), authres.NewIdentityLastActive("ind-user@example.com")))

		// First call with identity + fingerprint A.
		ctx := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "ind-user@example.com"})
		ctx = ctxstore.WithValue(ctx, auth.FingerprintContextKey{Fingerprint: "fingerprintA"})

		_, err := activity.Unary()(ctx, nil, nil, noopHandler)
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, st, "fingerprintA", func(res *authres.PublicKeyLastActive, asrt *assert.Assertions) {
			asrt.NotNil(res.TypedSpec().Value.LastUsed)
		})

		// Second call with same identity but different fingerprint B. Identity should be debounced, but fingerprint B should be tracked.
		ctx2 := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "ind-user@example.com"})
		ctx2 = ctxstore.WithValue(ctx2, auth.FingerprintContextKey{Fingerprint: "fingerprintB"})

		_, err = activity.Unary()(ctx2, nil, nil, noopHandler)
		require.NoError(t, err)

		rtestutils.AssertResource(ctx2, t, st, "fingerprintB", func(res *authres.PublicKeyLastActive, asrt *assert.Assertions) {
			asrt.NotNil(res.TypedSpec().Value.LastUsed)
		})
	})

	t.Run("updates existing resource", func(t *testing.T) {
		t.Parallel()

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		logger := zaptest.NewLogger(t)

		// Pre-create an old activity record.
		oldActivity := authres.NewIdentityLastActive("existing-user@example.com")
		oldActivity.TypedSpec().Value.LastActive = timestamppb.New(time.Now().Add(-2 * time.Hour))

		require.NoError(t, st.Create(t.Context(), oldActivity))

		activity := interceptor.NewActivity(st, logger)
		ctx := ctxstore.WithValue(t.Context(), auth.IdentityContextKey{Identity: "existing-user@example.com"})

		_, err := activity.Unary()(ctx, nil, nil, noopHandler)
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, st, "existing-user@example.com", func(res *authres.IdentityLastActive, asrt *assert.Assertions) {
			asrt.WithinDuration(time.Now(), res.TypedSpec().Value.LastActive.AsTime(), 5*time.Second)
		})
	})
}
