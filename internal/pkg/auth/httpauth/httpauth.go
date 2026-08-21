// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package httpauth applies Omni's gRPC auth interceptors to plain HTTP requests, so an HTTP route
// can be protected by exactly the same checks as a gRPC method.
package httpauth

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

// MetadataHeaderPrefix is the header prefix the frontend uses to carry gRPC metadata on the plain
// HTTP requests it signs.
const MetadataHeaderPrefix = "grpc-metadata-"

// RejectFunc renders the response body for a request that failed auth. The middleware does not
// know the error shape a given route family uses, so the caller supplies it.
type RejectFunc func(w http.ResponseWriter, status string, code int)

// Middleware runs the given auth interceptor chain against a plain HTTP request, by reconstructing
// the gRPC metadata and signed message the interceptors expect from the request's
// "Grpc-Metadata-*" headers (set by the frontend for every "/api/..." request it signs).
//
// The chain is configured once and applied per route, each route naming the role it requires. The
// interceptors run in the order they are given, and a route's handler is called only if all of them
// pass and the caller holds that role; otherwise reject renders the failure.
func Middleware(interceptors []grpc.UnaryServerInterceptor, reject RejectFunc, logger *zap.Logger) func(http.Handler, role.Role) http.Handler {
	return func(next http.Handler, requiredRole role.Role) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := grpcutil.SetAuditInCtx(metadata.NewIncomingContext(r.Context(), metadataFromHeader(r.Header)))

			// The interceptors dispatch on the gRPC method name, which for a signed HTTP request is
			// the route path without the "/api" prefix the frontend signs it under.
			info := &grpc.UnaryServerInfo{FullMethod: strings.TrimPrefix(r.URL.Path, "/api")}

			served := false

			// Each interceptor hands the context it enriched to the handler it wraps, so the request is served
			// from the innermost handler of the chain, where every enrichment is visible.
			handler := grpc.UnaryHandler(func(handlerCtx context.Context, _ any) (any, error) {
				// The role check belongs here rather than around the chain, for the same reason: the
				// interceptors are what establish the caller's role, and this is the first point at
				// which it can be read. A gRPC method checks it in its own handler, equally late.
				if _, err := auth.CheckGRPC(handlerCtx, auth.WithRole(requiredRole)); err != nil {
					return nil, err
				}

				served = true

				next.ServeHTTP(w, r.WithContext(handlerCtx))

				return nil, nil
			})

			// build the chain from the inside out, so that the interceptors run in the order they were given.
			for _, interceptor := range slices.Backward(interceptors) {
				unary, inner := interceptor, handler

				handler = func(handlerCtx context.Context, req any) (any, error) {
					return unary(handlerCtx, req, info, inner)
				}
			}

			// served guards against writing an error over a response the wrapped handler already
			// started: an interceptor may fail on the way out, after the handler ran.
			if _, err := handler(ctx, nil); err != nil && !served {
				logger.Info("http request failed auth", zap.String("path", r.URL.Path), zap.Error(err))

				// Report the same message a gRPC caller gets for this failure, e.g. "invalid
				// signature" or "unauthorized: insufficient role".
				reject(w, status.Convert(err).Message(), statusFromGRPCErr(err))
			}
		})
	}
}

// metadataFromHeader recovers the gRPC metadata the frontend encoded into the request headers.
func metadataFromHeader(header http.Header) metadata.MD {
	md := metadata.MD{}

	for name, values := range header {
		lowerName := strings.ToLower(name)
		if !strings.HasPrefix(lowerName, MetadataHeaderPrefix) {
			continue
		}

		key := strings.TrimPrefix(lowerName, MetadataHeaderPrefix)

		md[key] = append(md[key], values...)
	}

	return md
}

// statusFromGRPCErr maps an interceptor failure to an HTTP status: a missing or invalid identity is
// unauthorized, anything else is a valid identity lacking permission.
func statusFromGRPCErr(err error) int {
	if status.Code(err) == codes.Unauthenticated {
		return http.StatusUnauthorized
	}

	return http.StatusForbidden
}
