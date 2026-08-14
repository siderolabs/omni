// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package httpauth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/httpauth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// rejection records what the middleware rendered for a request that failed auth.
type rejection struct {
	status string
	code   int
	called bool
}

func (r *rejection) reject(_ http.ResponseWriter, status string, code int) {
	r.status, r.code, r.called = status, code, true
}

// build wraps next in the middleware, prepending the interceptor that stands in for Omni's auth
// config one - every chain has it in production, and the role check the middleware runs needs what
// it puts on the context. Returns the handler plus the recorded rejection.
func build(t *testing.T, interceptors []grpc.UnaryServerInterceptor, next http.Handler) (http.Handler, *rejection) {
	t.Helper()

	return buildWithRole(t, role.None, append([]grpc.UnaryServerInterceptor{authenticated(role.Admin)}, interceptors...), next)
}

func buildWithRole(t *testing.T, requiredRole role.Role, interceptors []grpc.UnaryServerInterceptor, next http.Handler) (http.Handler, *rejection) {
	t.Helper()

	rej := &rejection{}

	return httpauth.Middleware(interceptors, rej.reject, zaptest.NewLogger(t))(next, requiredRole), rej
}

// authenticated returns an interceptor standing in for the chain that establishes an identity: the
// auth config interceptor marks auth as enabled, and the signature interceptor puts the verified
// caller's role on the context.
func authenticated(callerRole role.Role) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: true})
		ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: callerRole})

		return next(ctx, req)
	}
}

func okHandler(served *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if served != nil {
			*served = true
		}

		w.WriteHeader(http.StatusOK)
	})
}

// failing returns an interceptor that rejects every request with the given code.
func failing(code codes.Code) grpc.UnaryServerInterceptor {
	return func(context.Context, any, *grpc.UnaryServerInfo, grpc.UnaryHandler) (any, error) {
		return nil, status.Error(code, "denied")
	}
}

func request(t *testing.T, path string, header http.Header) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)

	for name, values := range header {
		for _, value := range values {
			r.Header.Add(name, value)
		}
	}

	return r
}

func TestMetadataIsRecoveredFromHeaders(t *testing.T) {
	var got metadata.MD

	capture := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		got, _ = metadata.FromIncomingContext(ctx)

		return next(ctx, req)
	}

	handler, rej := build(t, []grpc.UnaryServerInterceptor{capture}, okHandler(nil))

	handler.ServeHTTP(httptest.NewRecorder(), request(t, "/api/vex/v1.2.3", http.Header{
		// The frontend sets these with mixed case; gRPC metadata keys are lowercase.
		"Grpc-Metadata-Signature": {"sig"},
		"Grpc-metadata-Timestamp": {"12345"},
		// Repeated headers become a multi-valued metadata entry.
		"Grpc-Metadata-Scope": {"first", "second"},
		// Anything without the prefix is not metadata.
		"Authorization": {"Bearer token"},
		"Content-Type":  {"application/json"},
	}))

	require.False(t, rej.called)
	require.Equal(t, metadata.MD{
		"signature": {"sig"},
		"timestamp": {"12345"},
		"scope":     {"first", "second"},
	}, got)
}

func TestMethodDropsAPIPrefix(t *testing.T) {
	for _, test := range []struct{ path, expected string }{
		{"/api/vex/v1.2.3", "/vex/v1.2.3"},
		{"/api/sbom/abc/v1.2.3/amd64", "/sbom/abc/v1.2.3/amd64"},
		// Only the leading "/api" is dropped, not a later occurrence of it.
		{"/api/vulns/api/v1.2.3/amd64/report.json", "/vulns/api/v1.2.3/amd64/report.json"},
		// A route outside "/api" is passed through unchanged.
		{"/healthz", "/healthz"},
	} {
		t.Run(test.path, func(t *testing.T) {
			var method string

			capture := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
				method = info.FullMethod

				return next(ctx, req)
			}

			handler, _ := build(t, []grpc.UnaryServerInterceptor{capture}, okHandler(nil))

			handler.ServeHTTP(httptest.NewRecorder(), request(t, test.path, nil))

			require.Equal(t, test.expected, method)
		})
	}
}

func TestRejectionStatusMapping(t *testing.T) {
	for _, test := range []struct {
		code     codes.Code
		expected int
	}{
		// No usable identity: the caller may retry with credentials.
		{codes.Unauthenticated, http.StatusUnauthorized},
		// A valid identity that is not allowed to do this.
		{codes.PermissionDenied, http.StatusForbidden},
		// Anything else must not read as "retry with credentials".
		{codes.Internal, http.StatusForbidden},
		{codes.InvalidArgument, http.StatusForbidden},
	} {
		t.Run(test.code.String(), func(t *testing.T) {
			served := false

			handler, rej := build(t, []grpc.UnaryServerInterceptor{failing(test.code)}, okHandler(&served))

			handler.ServeHTTP(httptest.NewRecorder(), request(t, "/api/vex/v1.2.3", nil))

			require.False(t, served, "wrapped handler must not run when auth fails")
			require.True(t, rej.called)
			require.Equal(t, test.expected, rej.code)
			// The caller is told what a gRPC caller would be told for the same failure.
			require.Equal(t, "denied", rej.status)
		})
	}
}

func TestInterceptorsRunInOrder(t *testing.T) {
	var order []string

	record := func(name string) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
			order = append(order, name)

			return next(ctx, req)
		}
	}

	handler, rej := build(t, []grpc.UnaryServerInterceptor{record("first"), record("second"), record("third")},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			order = append(order, "handler")

			w.WriteHeader(http.StatusOK)
		}))

	handler.ServeHTTP(httptest.NewRecorder(), request(t, "/api/vex/v1.2.3", nil))

	require.False(t, rej.called)
	require.Equal(t, []string{"first", "second", "third", "handler"}, order)
}

// A later interceptor must see the context enrichment of the ones before it, which only holds if
// the request is served from the innermost handler of the chain.
func TestContextEnrichmentReachesTheHandler(t *testing.T) {
	type key struct{ n int }

	enrich := func(n int) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
			return next(context.WithValue(ctx, key{n}, n), req)
		}
	}

	var seen []any

	handler, _ := build(t, []grpc.UnaryServerInterceptor{enrich(1), enrich(2)},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = []any{r.Context().Value(key{1}), r.Context().Value(key{2})}

			w.WriteHeader(http.StatusOK)
		}))

	handler.ServeHTTP(httptest.NewRecorder(), request(t, "/api/vex/v1.2.3", nil))

	require.Equal(t, []any{1, 2}, seen)
}

// An interceptor can fail on the way out, after the wrapped handler already wrote a response. The
// middleware must not write an error over it.
func TestNoRejectionAfterTheHandlerRan(t *testing.T) {
	failOnExit := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if _, err := next(ctx, req); err != nil {
			return nil, err
		}

		return nil, status.Error(codes.Unauthenticated, "too late")
	}

	served := false

	handler, rej := build(t, []grpc.UnaryServerInterceptor{failOnExit}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served = true

		w.WriteHeader(http.StatusOK)

		_, err := fmt.Fprint(w, "body")
		require.NoError(t, err)
	}))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, request(t, "/api/vex/v1.2.3", nil))

	require.True(t, served)
	require.False(t, rej.called, "must not write an error over a response the handler already sent")
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "body", resp.Body.String())
}

func TestRoleIsEnforced(t *testing.T) {
	for _, test := range []struct {
		name       string
		callerRole role.Role
		required   role.Role
		rejected   bool
	}{
		{name: "exact role", callerRole: role.Reader, required: role.Reader},
		{name: "role above the requirement", callerRole: role.Admin, required: role.Reader},
		{name: "role below the requirement", callerRole: role.None, required: role.Reader, rejected: true},
		{name: "reader asked for operator", callerRole: role.Reader, required: role.Operator, rejected: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			served := false

			handler, rej := buildWithRole(t, test.required,
				[]grpc.UnaryServerInterceptor{authenticated(test.callerRole)}, okHandler(&served))

			handler.ServeHTTP(httptest.NewRecorder(), request(t, "/api/vex/v1.2.3", nil))

			require.Equal(t, !test.rejected, served)
			require.Equal(t, test.rejected, rej.called)

			if test.rejected {
				require.Equal(t, http.StatusForbidden, rej.code)
				require.Contains(t, rej.status, "unauthorized")
			}
		})
	}
}

// An Omni running with auth disabled serves these routes to everyone: the auth config interceptor
// says so, and the role check honors it.
func TestAuthDisabledPassesThrough(t *testing.T) {
	authDisabled := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		return next(ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: false}), req)
	}

	served := false

	handler, rej := buildWithRole(t, role.Admin, []grpc.UnaryServerInterceptor{authDisabled}, okHandler(&served))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, request(t, "/api/vex/v1.2.3", nil))

	require.True(t, served)
	require.False(t, rej.called)
	require.Equal(t, http.StatusOK, resp.Code)
}

// Without the interceptor that reports whether auth is enabled there is nothing to authorize
// against, so the middleware fails closed rather than serving the route.
func TestNoInterceptorsFailsClosed(t *testing.T) {
	served := false

	handler, rej := buildWithRole(t, role.Reader, nil, okHandler(&served))

	handler.ServeHTTP(httptest.NewRecorder(), request(t, "/api/vex/v1.2.3", nil))

	require.False(t, served)
	require.True(t, rej.called)
	require.Equal(t, http.StatusUnauthorized, rej.code)
}
