// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpczap

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/omni/internal/pkg/grpcutil/grpczap/ctxzap"
)

// DefaultMessageProducer writes the default final gRPC log entry.
func DefaultMessageProducer(ctx context.Context, msg string, level zapcore.Level, code codes.Code, err error, duration zapcore.Field) {
	if checkedEntry := ctxzap.Extract(ctx).Check(level, msg); checkedEntry != nil {
		checkedEntry.Write(
			zap.Error(err),
			zap.String("grpc.code", code.String()),
			duration,
		)
	}
}
