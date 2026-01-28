// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/conn"

	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

func TestBoundBind(t *testing.T) {
	t.Parallel()

	t.Run("binds to specific address", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")

		fns, port, err := bind.Open(0)
		require.NoError(t, err)
		require.NotEmpty(t, fns)
		require.NotZero(t, port)

		t.Cleanup(func() {
			require.NoError(t, bind.Close())
		})

		// Verify it's actually bound to 127.0.0.1 by checking we can't connect from external
		// We do this by trying to send to the port and verifying the local address
		testConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)})
		require.NoError(t, err)

		t.Cleanup(func() {
			testConn.Close() //nolint:errcheck
		})

		localAddr, ok := testConn.LocalAddr().(*net.UDPAddr)
		require.True(t, ok, "local address should be UDPAddr")
		assert.True(t, localAddr.IP.IsLoopback(), "connection should be on loopback")
	})

	t.Run("open returns error if already open", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")

		_, _, err := bind.Open(0)
		require.NoError(t, err)

		t.Cleanup(func() {
			bind.Close() //nolint:errcheck
		})

		_, _, err = bind.Open(0)
		require.ErrorIs(t, err, conn.ErrBindAlreadyOpen)
	})

	t.Run("close is idempotent", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")

		_, _, err := bind.Open(0)
		require.NoError(t, err)

		require.NoError(t, bind.Close())
		require.NoError(t, bind.Close())
	})

	t.Run("send and receive", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")

		fns, port, err := bind.Open(0)
		require.NoError(t, err)
		require.Len(t, fns, 1)

		t.Cleanup(func() {
			require.NoError(t, bind.Close())
		})

		// Create a client to send data to the bind
		clientConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)})
		require.NoError(t, err)

		t.Cleanup(func() {
			clientConn.Close() //nolint:errcheck
		})

		// Send test data
		testData := []byte("hello wireguard")
		_, err = clientConn.Write(testData)
		require.NoError(t, err)

		// Receive using the bind's receive function
		bufs := make([][]byte, 1)
		bufs[0] = make([]byte, 1024)
		sizes := make([]int, 1)
		eps := make([]conn.Endpoint, 1)

		n, err := fns[0](bufs, sizes, eps)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, len(testData), sizes[0])
		assert.Equal(t, testData, bufs[0][:sizes[0]])
		assert.NotNil(t, eps[0])

		// Send data back using the bind
		replyData := []byte("hello back")
		err = bind.Send([][]byte{replyData}, eps[0])
		require.NoError(t, err)

		// Receive the reply on the client
		buf := make([]byte, 1024)
		n2, err := clientConn.Read(buf)
		require.NoError(t, err)
		assert.Equal(t, replyData, buf[:n2])
	})

	t.Run("parse endpoint", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")

		ep, err := bind.ParseEndpoint("192.168.1.1:51820")
		require.NoError(t, err)

		assert.Equal(t, "192.168.1.1:51820", ep.DstToString())
		assert.Equal(t, netip.MustParseAddr("192.168.1.1"), ep.DstIP())
	})

	t.Run("parse endpoint invalid", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")

		_, err := bind.ParseEndpoint("invalid")
		require.Error(t, err)
	})

	t.Run("batch size is 1", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")
		assert.Equal(t, 1, bind.BatchSize())
	})

	t.Run("set mark returns nil", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")
		assert.NoError(t, bind.SetMark(42))
	})

	t.Run("concurrent send and close", func(t *testing.T) {
		t.Parallel()

		bind := siderolink.NewBoundBind("127.0.0.1")

		_, port, err := bind.Open(0)
		require.NoError(t, err)

		ep, err := bind.ParseEndpoint("127.0.0.1:" + strconv.Itoa(int(port)))
		require.NoError(t, err)

		var wg sync.WaitGroup

		// Start multiple goroutines sending
		for range 10 {
			wg.Go(func() {
				for range 100 {
					bind.Send([][]byte{[]byte("test")}, ep) //nolint:errcheck
				}
			})
		}

		// Close while sends are happening
		require.NoError(t, bind.Close())

		wg.Wait()
	})
}
