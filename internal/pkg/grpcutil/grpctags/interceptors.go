// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpctags

import (
	"context"
	"net"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// UnaryServerInterceptor initializes request-scoped tags for unary calls.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(newTagsForCtx(ctx), req)
	}
}

// StreamServerInterceptor initializes request-scoped tags for streaming calls.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = newTagsForCtx(stream.Context())

		return handler(srv, wrapped)
	}
}

func newTagsForCtx(ctx context.Context) context.Context {
	tags := NewTags()
	if p, ok := peer.FromContext(ctx); ok {
		tags.Set("peer.address", peerAddress(p.Addr))
	}

	return SetInContext(ctx, tags)
}

func peerAddress(addr net.Addr) string {
	if addr == nil {
		return ""
	}

	host, _, err := net.SplitHostPort(addr.String())
	if err == nil {
		return host
	}

	return addr.String()
}
