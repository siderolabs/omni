// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpczap_test

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/omni/internal/pkg/grpcutil/grpczap"
)

func TestDefaultMessageProducerSkipsDisabledLevel(t *testing.T) {
	grpczap.DefaultMessageProducer(context.Background(), "test", zapcore.InfoLevel, codes.OK, nil, zap.Duration("grpc.duration", 0))
}
