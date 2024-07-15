// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpcutil

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/api/v1alpha1"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/pkg/constants"
)

const (
	// LogLevelOverrideMetadataKey is a metadata key that can be used to override the log level.
	//
	// Should only be used in debug mode.
	LogLevelOverrideMetadataKey = "log-level-override"
)

// Hook is a function that can be used to modify the request or response before it is logged.
type Hook func(req proto.Message) proto.Message

// NewHook returns a hook that will rewrite the request or response using the provided rewriter.
func NewHook(hooks ...Rewriter) Hook {
	return func(req proto.Message) proto.Message {
		for _, hook := range hooks {
			res, ok := hook(req)
			if ok {
				return res
			}
		}

		return req
	}
}

// Rewriter is a function that can be used to modify the request or response before it is logged.
type Rewriter func(req proto.Message) (proto.Message, bool)

// NewRewriter returns a rewriter that will rewrite the copy of request or response using the provided rewriter.
func NewRewriter[T proto.Message](f func(T) (T, bool)) Rewriter {
	return func(req proto.Message) (proto.Message, bool) {
		typedReq, ok := req.(T)
		if !ok {
			return req, false
		}

		return f(proto.Clone(typedReq).(T)) //nolint:forcetypeassert
	}
}

type jsonpbObjectMarshaler struct {
	pb        proto.Message
	bodyLimit int
}

func (j *jsonpbObjectMarshaler) MarshalLogObject(e zapcore.ObjectEncoder) error {
	// ZAP jsonEncoder deals with AddReflect by using json.MarshalObject. The same thing applies for consoleEncoder.
	return e.AddReflected("msg", j)
}

func (j *jsonpbObjectMarshaler) MarshalJSON() ([]byte, error) {
	res, err := marshaller.Marshal(j.pb)
	if err != nil {
		return nil, fmt.Errorf("json serializer failed: %w", err)
	}

	if j.bodyLimit > 0 && len(res) > j.bodyLimit {
		res = res[:j.bodyLimit]
	}

	return res, nil
}

var marshaller = protojson.MarshalOptions{}

// SetRealPeerAddress returns a new unary server interceptor that adds the real peer address for "peer.address" tag.
func SetRealPeerAddress() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		setRealIPAddress(ctx)

		return handler(ctx, req)
	}
}

// SetUserAgent returns a new unary server interceptor that adds user agent to the list of ctxtags.
func SetUserAgent() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		setUserAgent(ctx)

		return handler(ctx, req)
	}
}

// InterceptBodyToTags returns a new unary server interceptor that adds request and response body to the list ctxtags.
func InterceptBodyToTags(hook Hook, bodyLimit int) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if r, ok := req.(proto.Message); ok {
			grpc_ctxtags.Extract(ctx).Set("grpc.request.content", &jsonpbObjectMarshaler{pb: hook(r), bodyLimit: bodyLimit})
		}

		resp, err := handler(ctx, req)
		if r, ok := resp.(proto.Message); ok {
			grpc_ctxtags.Extract(ctx).Set("grpc.response.content", &jsonpbObjectMarshaler{pb: hook(r), bodyLimit: bodyLimit})
		}

		return resp, err
	}
}

func setRealIPAddress(ctx context.Context) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		for _, hdr := range []string{"x-forwarded-for", "x-real-ip"} {
			peer := md.Get(hdr)
			if len(peer) == 0 {
				continue
			}

			addr, err := netip.ParseAddr(peer[0])
			if err != nil {
				continue
			}

			grpc_ctxtags.Extract(ctx).Set("peer.address", addr.String())

			break
		}
	}
}

func setUserAgent(ctx context.Context) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		for _, hdr := range []string{"grpcgateway-user-agent", "user-agent"} {
			if ua := md.Get(hdr); len(ua) > 0 {
				grpc_ctxtags.Extract(ctx).Set("user_agent", ua[0])

				break
			}
		}
	}
}

// SetShouldLog marks the context to log the request.
func SetShouldLog(ctx context.Context, system string) {
	grpc_ctxtags.Extract(ctx).Set("request_log_initiator", system)
}

// AddLogPair adds a key-value pair to the context.
func AddLogPair(ctx context.Context, key string, val any) {
	grpc_ctxtags.Extract(ctx).Set(key, val)
}

// ShouldLog returns true if the request should be logged.
func ShouldLog(ctx context.Context) bool {
	return grpc_ctxtags.Extract(ctx).Has("request_log_initiator")
}

const successLogLevelTag = "grpc.success_log_level"

// LogLevelOverridingMessageProducer returns a new message producer
// that overrides the log level based on the incoming context.
func LogLevelOverridingMessageProducer(wrapped grpc_zap.MessageProducer) grpc_zap.MessageProducer {
	return func(ctx context.Context, msg string, level zapcore.Level, code codes.Code, err error, duration zapcore.Field) {
		if code == codes.OK {
			tags := grpc_ctxtags.Extract(ctx).Values()
			if logLevel, ok := tags[successLogLevelTag].(zapcore.Level); ok {
				delete(tags, successLogLevelTag)

				level = logLevel
			}
		}

		if !constants.IsDebugBuild {
			wrapped(ctx, msg, level, code, err, duration)

			return
		}

		// this is a debug build, so we additionally check the log level override header

		shouldLog := true

		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			logLevels := md.Get(LogLevelOverrideMetadataKey)
			if len(logLevels) > 0 {
				parsedLevel, parseErr := zapcore.ParseLevel(logLevels[0])
				if parseErr == nil {
					shouldLog = level >= parsedLevel
				}
			}
		}

		if shouldLog {
			wrapped(ctx, msg, level, code, err, duration)
		}
	}
}

// LogLevelInterceptors returns a pair of unary and stream server interceptors that control the log level based on the GRPC method.
//
// Currently, the log levels of the read operations on the COSI ResourceService are overridden to the debug level.
func LogLevelInterceptors() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	isReadMethod := func(method string) bool {
		switch method {
		case resources.ResourceService_Get_FullMethodName, resources.ResourceService_List_FullMethodName, resources.ResourceService_Watch_FullMethodName,
			v1alpha1.State_Get_FullMethodName, v1alpha1.State_List_FullMethodName, v1alpha1.State_Watch_FullMethodName:
			return true
		default:
			return false
		}
	}

	unary := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isReadMethod(info.FullMethod) {
			grpc_ctxtags.Extract(ctx).Set(successLogLevelTag, zapcore.DebugLevel)
		}

		return handler(ctx, req)
	}

	stream := func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isReadMethod(info.FullMethod) {
			grpc_ctxtags.Extract(stream.Context()).Set(successLogLevelTag, zapcore.DebugLevel)
		}

		return handler(srv, stream)
	}

	return unary, stream
}

// SetAuditData returns a new unary server interceptor that adds audit data to the context.
func SetAuditData() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(SetAuditInCtx(ctx), req)
	}
}
