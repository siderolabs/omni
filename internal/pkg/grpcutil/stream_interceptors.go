// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpcutil

import (
	"context"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// StreamSetRealPeerAddress returns a new stream server interceptor that adds the real peer address for "peer.address" tag.
func StreamSetRealPeerAddress() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		setRealIPAddress(ss.Context())

		return handler(srv, ss)
	}
}

// StreamSetUserAgent returns a new stream server interceptor that adds user agent to the list of ctxtags.
func StreamSetUserAgent() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		setUserAgent(ss.Context())

		return handler(srv, ss)
	}
}

// StreamIntercept returns a new stream server interceptor that calls the given hook functions.
func StreamIntercept(hooks StreamHooks) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &serverStreamHook{
			ServerStream: ss,
			info:         info,
			hooks:        hooks,
		})
	}
}

// RecvMsgHandler is a function that handles received message.
type RecvMsgHandler func(msg any) error

// RecvMsgHook is a hook function for RecvMsgHandler.
type RecvMsgHook func(ctx context.Context, msg any, info *grpc.StreamServerInfo, handler RecvMsgHandler) error

// StreamHooks is a set of hooks that are called when a stream is intercepted.
type StreamHooks struct {
	RecvMsg RecvMsgHook
}

type serverStreamHook struct {
	grpc.ServerStream

	info  *grpc.StreamServerInfo
	hooks StreamHooks
}

func (s *serverStreamHook) RecvMsg(msg any) error {
	if s.hooks.RecvMsg != nil {
		return s.hooks.RecvMsg(s.Context(), msg, s.info, s.ServerStream.RecvMsg)
	}

	return s.ServerStream.RecvMsg(msg)
}

// StreamInterceptRequestBodyToTags returns a new stream server interceptor that adds request body to the list of ctxtags.
// It uses the given hook function to override proto message.
func StreamInterceptRequestBodyToTags(hook Hook, bodyLimit int) RecvMsgHook {
	return func(ctx context.Context, msg any, info *grpc.StreamServerInfo, handler RecvMsgHandler) error {
		if info.IsClientStream {
			return handler(msg)
		}

		result := handler(msg)

		if actualMsg, ok := msg.(proto.Message); ok {
			grpc_ctxtags.Extract(ctx).Set("grpc.request.content", &jsonpbObjectMarshaler{
				pb:        hook(actualMsg),
				bodyLimit: bodyLimit,
			})
		}

		return result
	}
}

// StreamSetAuditData returns a new stream server interceptor that adds audit data to the context.
func StreamSetAuditData() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: SetAuditInCtx(ss.Context()),
		})
	}
}
