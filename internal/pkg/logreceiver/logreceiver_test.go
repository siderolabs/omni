// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package logreceiver_test

import (
	"bytes"
	"context"
	"io"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/pkg/logreceiver"
)

//nolint:govet
type testLogHandler struct {
	t      *testing.T
	b      bytes.Buffer
	logger *zap.Logger
}

var addr = netip.MustParseAddr("1.2.3.4")

func (t *testLogHandler) HandleMessage(_ context.Context, srcAddress netip.Addr, rawData []byte) {
	assert.Equal(t.t, addr, srcAddress)
	t.b.Write(rawData)
}

func (t *testLogHandler) HandleError(srcAddress netip.Addr, err error) {
	assert.Equal(t.t, addr, srcAddress)
	t.t.Fatal(err)
}

func (t *testLogHandler) HasLink(netip.Addr) bool {
	return true
}

func TestConnHandler(t *testing.T) {
	logger := zaptest.NewLogger(t)
	handler := &testLogHandler{
		t:      t,
		logger: logger,
	}
	ch := logreceiver.NewConnHandler(handler, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch.HandleConn(ctx, addr, io.NopCloser(bytes.NewBufferString("{ hello: \"1\" }\n{ hello: \"2\" }\n")))
	assert.Equal(t, "{ hello: \"1\" }{ hello: \"2\" }", handler.b.String())
}
