// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package logging_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/omni/internal/backend/logging"
)

func TestDropMessagesContaining(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := logging.DropMessagesContaining(zap.New(core), "no known endpoint for peer", "MTU not updated")

	logger.Warn("peer(abcd…wxyz) - Failed to send handshake initiation: no known endpoint for peer")
	logger.With(zap.String("component", "siderolink")).
		Warn("peer(abcd…wxyz) - Failed to send data packets: no known endpoint for peer")
	logger.Warn("MTU not updated to negative value: -1")
	logger.Warn("peer(abcd…wxyz) - Failed to send handshake initiation: write udp: connection refused")
	logger.Info("wireguard device set up")

	entries := logs.All()

	require.Len(t, entries, 2)
	require.Equal(t, "peer(abcd…wxyz) - Failed to send handshake initiation: write udp: connection refused", entries[0].Message)
	require.Equal(t, "wireguard device set up", entries[1].Message)

	// the filter must leave the reported level alone
	require.Equal(t, zapcore.DebugLevel, logger.Level())
}

func TestDropMessagesContainingWithoutSubstrings(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	require.Same(t, logger, logging.DropMessagesContaining(logger))

	logger.Warn("nothing is filtered")

	require.Equal(t, 1, logs.Len())
}
