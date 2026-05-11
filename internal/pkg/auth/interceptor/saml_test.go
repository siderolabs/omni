// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/crewjam/saml"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/interceptor"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestSAMLSessionCanOnlyBeUsedOnceConcurrently(t *testing.T) {
	t.Parallel()

	const (
		sessionID    = "test-session"
		requestCount = 32
		email        = "user@example.com"
	)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	assertionData, err := json.Marshal(saml.Assertion{IssueInstant: time.Now().UTC()})
	require.NoError(t, err)

	assertionResource := authres.NewSAMLAssertion(sessionID)
	assertionResource.TypedSpec().Value.Data = assertionData
	assertionResource.TypedSpec().Value.Email = email

	require.NoError(t, st.Create(actor.MarkContextAsInternalActor(ctx), assertionResource))

	samlInterceptor := interceptor.NewSAML(st, zap.NewNop())

	var successCount, unauthenticatedCount atomic.Int32

	eg, ctx := errgroup.WithContext(ctx)

	for range requestCount {
		eg.Go(func() error {
			_, callErr := samlInterceptor.Unary()(samlRequestContext(ctx, sessionID), nil, nil, noopHandler)

			switch {
			case callErr == nil:
				successCount.Add(1)
			case status.Code(callErr) == codes.Unauthenticated:
				unauthenticatedCount.Add(1)
			default:
				return callErr
			}

			return nil
		})
	}

	require.NoError(t, eg.Wait())
	assert.Equal(t, int32(1), successCount.Load())
	assert.Equal(t, int32(requestCount-1), unauthenticatedCount.Load())

	usedAssertion, err := safe.StateGetByID[*authres.SAMLAssertion](ctx, st, sessionID)
	require.NoError(t, err)
	assert.True(t, usedAssertion.TypedSpec().Value.Used)
}

func samlRequestContext(ctx context.Context, sessionID string) context.Context {
	md := metadata.Pairs(auth.SamlSessionHeaderKey, sessionID)

	ctx = metadata.NewIncomingContext(ctx, md)
	ctx = ctxstore.WithValue(ctx, auth.GRPCMessageContextKey{Message: message.NewGRPC(md, "/omni.test/SAML")})
	ctx = ctxstore.WithValue(ctx, &auditlog.Data{})

	return ctx
}
