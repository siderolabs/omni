// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpczap

import (
	"context"
	"path"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/internal/pkg/grpcutil/grpczap/ctxzap"
)

var (
	// SystemField is used in every log statement made through this package.
	SystemField = zap.String("system", "grpc")

	// ServerField is used in every server-side log statement made through this package.
	ServerField = zap.String("span.kind", "server")
)

// UnaryServerInterceptor returns a unary server interceptor that adds zap.Logger to the context.
func UnaryServerInterceptor(logger *zap.Logger, opts ...Option) grpc.UnaryServerInterceptor {
	o := evaluateServerOpt(opts)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		startTime := time.Now()
		newCtx := newLoggerForCall(ctx, logger, info.FullMethod, startTime, o.timestampFormat)

		resp, err := handler(newCtx, req)
		if !o.shouldLog(info.FullMethod, err) {
			return resp, err
		}

		code := o.codeFunc(err)
		level := o.levelFunc(code)
		duration := o.durationFunc(time.Since(startTime))

		o.messageFunc(newCtx, "finished unary call with code "+code.String(), level, code, err, duration)

		return resp, err
	}
}

// StreamServerInterceptor returns a stream server interceptor that adds zap.Logger to the context.
func StreamServerInterceptor(logger *zap.Logger, opts ...Option) grpc.StreamServerInterceptor {
	o := evaluateServerOpt(opts)

	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		newCtx := newLoggerForCall(stream.Context(), logger, info.FullMethod, startTime, o.timestampFormat)

		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx

		err := handler(srv, wrapped)
		if !o.shouldLog(info.FullMethod, err) {
			return err
		}

		code := o.codeFunc(err)
		level := o.levelFunc(code)
		duration := o.durationFunc(time.Since(startTime))

		o.messageFunc(newCtx, "finished streaming call with code "+code.String(), level, code, err, duration)

		return err
	}
}

func serverCallFields(fullMethodString string) []zapcore.Field {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)

	return []zapcore.Field{
		SystemField,
		ServerField,
		zap.String("grpc.service", service),
		zap.String("grpc.method", method),
	}
}

func newLoggerForCall(ctx context.Context, logger *zap.Logger, fullMethodString string, start time.Time, timestampFormat string) context.Context {
	fields := []zapcore.Field{zap.String("grpc.start_time", start.Format(timestampFormat))}
	if deadline, ok := ctx.Deadline(); ok {
		fields = append(fields, zap.String("grpc.request.deadline", deadline.Format(timestampFormat)))
	}

	callLog := logger.With(append(fields, serverCallFields(fullMethodString)...)...)

	return ctxzap.ToContext(ctx, callLog)
}
