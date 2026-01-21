// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package services

import (
	"context"

	"github.com/siderolabs/go-debug"
	"go.uber.org/zap"
)

// RunDebugServer is the Go debug server.
func RunDebugServer(ctx context.Context, logger *zap.Logger, bindEndpoint string) error {
	debugLogFunc := func(msg string) {
		logger.Info(msg)
	}

	return debug.ListenAndServe(ctx, bindEndpoint, debugLogFunc)
}
