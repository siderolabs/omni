// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package ratelimit_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.zx2c4.com/wireguard/conn"

	"github.com/siderolabs/omni/internal/pkg/siderolink/ratelimit"
)

var _ conn.Endpoint = (*mockEndpoint)(nil)

type mockBind struct {
	conn.Bind
	sentBufs [][]byte
}

func (m *mockBind) Send(bufs [][]byte, _ conn.Endpoint) error {
	m.sentBufs = append(m.sentBufs, bufs...)

	return nil
}

type mockEndpoint struct {
	dst string
}

func (e *mockEndpoint) ClearSrc()           {}
func (e *mockEndpoint) SrcToString() string { return "" }
func (e *mockEndpoint) DstToString() string { return e.dst }
func (e *mockEndpoint) DstToBytes() []byte  { return nil }
func (e *mockEndpoint) DstIP() netip.Addr   { return netip.MustParseAddrPort(e.dst).Addr() }
func (e *mockEndpoint) SrcIP() netip.Addr   { return netip.Addr{} }

func TestWrapBind(t *testing.T) {
	t.Parallel()

	// 1 Mbps, burst = 1500 bytes
	limiter := ratelimit.NewLimiter(1, 1500, zaptest.NewLogger(t))
	require.NotNil(t, limiter)

	mock := &mockBind{}
	wrapped := limiter.WrapBind(mock)

	ep := &mockEndpoint{dst: "[fd00::1]:51820"}

	// First packet (1200 bytes) should pass.
	err := wrapped.Send([][]byte{make([]byte, 1200)}, ep)
	require.NoError(t, err)
	assert.Len(t, mock.sentBufs, 1)

	// Second packet should be dropped (burst exceeded).
	err = wrapped.Send([][]byte{make([]byte, 1200)}, ep)
	require.NoError(t, err)
	assert.Len(t, mock.sentBufs, 1, "second packet should be dropped")
}

func TestWrapBindPartialDrop(t *testing.T) {
	t.Parallel()

	// 1 Mbps, burst = 2000 bytes — allows ~1 packet of 1200 bytes but not 2
	limiter := ratelimit.NewLimiter(1, 2000, zaptest.NewLogger(t))
	require.NotNil(t, limiter)

	mock := &mockBind{}
	wrapped := limiter.WrapBind(mock)

	ep := &mockEndpoint{dst: "[fd00::1]:51820"}

	// Send batch of 3 packets — only first should pass
	bufs := [][]byte{
		make([]byte, 1200),
		make([]byte, 1200),
		make([]byte, 1200),
	}

	err := wrapped.Send(bufs, ep)
	require.NoError(t, err)

	assert.Len(t, mock.sentBufs, 1, "only first packet should pass within burst")
}

func TestWrapBindSharedBudget(t *testing.T) {
	t.Parallel()

	// 1 Mbps, burst = 2000 bytes — shared across all peers
	limiter := ratelimit.NewLimiter(1, 2000, zaptest.NewLogger(t))
	require.NotNil(t, limiter)

	mock := &mockBind{}
	wrapped := limiter.WrapBind(mock)

	ep1 := &mockEndpoint{dst: "[fd00::1]:51820"}
	ep2 := &mockEndpoint{dst: "[fd00::2]:51820"}

	// First peer's packet passes (1200 bytes consumed from shared bucket).
	err := wrapped.Send([][]byte{make([]byte, 1200)}, ep1)
	require.NoError(t, err)
	assert.Len(t, mock.sentBufs, 1)

	// Second peer's packet dropped — shared burst exhausted.
	err = wrapped.Send([][]byte{make([]byte, 1200)}, ep2)
	require.NoError(t, err)
	assert.Len(t, mock.sentBufs, 1, "second peer should be limited by shared budget")
}
