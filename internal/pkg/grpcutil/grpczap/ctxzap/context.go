// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package ctxzap provides a call-scoped zap logger stored in the context.
//
// It is a trimmed-down version of the "logging/zap/ctxzap"
// package from github.com/grpc-ecosystem/go-grpc-middleware v1,
// which was removed in v2.
package ctxzap

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/omni/internal/pkg/grpcutil/grpctags"
)

type ctxMarker struct{}

type ctxLogger struct {
	logger *zap.Logger
	fields []zapcore.Field
}

var (
	ctxMarkerKey = &ctxMarker{}
	nullLogger   = zap.NewNop()
)

// Extract returns the call-scoped logger stored in ctx.
func Extract(ctx context.Context) *zap.Logger {
	l, ok := ctx.Value(ctxMarkerKey).(*ctxLogger)
	if !ok || l == nil {
		return nullLogger
	}

	fields := TagsToFields(ctx)
	fields = append(fields, l.fields...)

	return l.logger.With(fields...)
}

// TagsToFields transforms request-scoped tags into zap fields.
func TagsToFields(ctx context.Context) []zapcore.Field {
	values := grpctags.Extract(ctx).Values()
	fields := make([]zapcore.Field, 0, len(values))

	for k, v := range values {
		fields = append(fields, zap.Any(k, v))
	}

	return fields
}

// ToContext stores logger in ctx.
func ToContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxMarkerKey, &ctxLogger{logger: logger})
}

// Error logs an error message with the logger stored in ctx.
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	Extract(ctx).WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
}
