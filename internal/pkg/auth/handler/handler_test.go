// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/handler"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func testHandler(t *testing.T, authEnabled bool) {
	ctxCh := make(chan context.Context, 1)

	coreHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctxCh <- r.Context()
	})

	logger := zaptest.NewLogger(t)

	authenticatorFunc := func(context.Context, string) (*auth.Authenticator, error) {
		return &auth.Authenticator{
			Verifier: mockSignerVerifier{},
			Identity: "user@example.com",
			UserID:   "user-id",
			Role:     role.Operator,
		}, nil
	}

	wrapWithAuth := func(h http.Handler) http.Handler {
		return handler.NewAuthConfig(handler.NewSignature(h, authenticatorFunc, logger), authEnabled, logger)
	}

	ts := httptest.NewServer(wrapWithAuth(coreHandler))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	type testCase struct { //nolint:govet
		name string
		uri  string

		signRequest     bool
		appendSignature []byte
		verifyContext   bool

		expectedCode int
	}

	var testCases []testCase

	if authEnabled {
		testCases = []testCase{
			{
				name:         "no signature",
				expectedCode: http.StatusOK,
				uri:          "/ok",
			},
			{
				name:         "correct signature",
				expectedCode: http.StatusOK,

				signRequest:   true,
				verifyContext: true,

				uri: "/ok",
			},
			{
				name:         "broken signature",
				expectedCode: http.StatusUnauthorized,

				signRequest:     true,
				appendSignature: []byte("broken"),

				uri: "/fail",
			},
		}
	} else {
		testCases = []testCase{
			{
				name:         "no signature",
				expectedCode: http.StatusOK,
				uri:          "/",
			},
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+tc.uri, nil)
			require.NoError(t, err)

			if tc.signRequest {
				var msg *message.HTTP

				msg, err = message.NewHTTP(req)
				require.NoError(t, err)

				require.NoError(t, msg.Sign("foo@example.com", mockSignerVerifier{tc.appendSignature}))
			}

			resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
			require.NoError(t, err)

			require.Equal(t, tc.expectedCode, resp.StatusCode)

			if tc.expectedCode != http.StatusOK {
				return
			}

			var reqCtx context.Context

			select {
			case reqCtx = <-ctxCh:
			case <-time.After(time.Second):
				t.Fatal("timeout")
			}

			ctxAuthEnabledVal, ok := ctxstore.Value[auth.EnabledAuthContextKey](reqCtx) //nolint:contextcheck
			require.True(t, ok)
			assert.Equal(t, authEnabled, ctxAuthEnabledVal.Enabled)

			if !tc.verifyContext {
				return
			}

			ctxUserIDVal, ok := ctxstore.Value[auth.UserIDContextKey](reqCtx) //nolint:contextcheck
			require.True(t, ok)
			assert.Equal(t, "user-id", ctxUserIDVal.UserID)

			ctxRoleVal, ok := ctxstore.Value[auth.RoleContextKey](reqCtx) //nolint:contextcheck
			require.True(t, ok)
			assert.Equal(t, role.Operator, ctxRoleVal.Role)

			ctxIdentityVal, ok := ctxstore.Value[auth.IdentityContextKey](reqCtx) //nolint:contextcheck
			require.True(t, ok)
			assert.Equal(t, "user@example.com", ctxIdentityVal.Identity)
		})
	}
}

func TestHandler(t *testing.T) {
	t.Run("AuthEnabled", func(t *testing.T) {
		testHandler(t, true)
	})
	t.Run("AuthDisabled", func(t *testing.T) {
		testHandler(t, false)
	})
}
