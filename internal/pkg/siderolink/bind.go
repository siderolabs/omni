// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"syscall"

	"golang.zx2c4.com/wireguard/conn"
)

// NewBoundBind creates a Bind that listens on a specific IP address.
//
// This is a simplified version of wireguard-go's conn.StdNetBind. The standard
// StdNetBind always binds to all interfaces (0.0.0.0), ignoring any specific host
// in the endpoint configuration. This implementation respects the configured host.
//
// As a side benefit, this also fixes connectivity issues on macOS when multiple
// interfaces share the same subnet (wireguard-go lacks sticky socket support on macOS).
func NewBoundBind(bindAddr string) conn.Bind {
	return &boundBind{bindAddr: bindAddr}
}

type boundBind struct {
	conn     *net.UDPConn
	bindAddr string
	mu       sync.Mutex
}

func (b *boundBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn != nil {
		return nil, 0, conn.ErrBindAlreadyOpen
	}

	addr := net.JoinHostPort(b.bindAddr, strconv.Itoa(int(port)))

	c, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "udp", addr)
	if err != nil {
		return nil, 0, err
	}

	udpConn, ok := c.(*net.UDPConn)
	if !ok {
		c.Close() //nolint:errcheck

		return nil, 0, errors.New("expected *net.UDPConn")
	}

	b.conn = udpConn

	laddr, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok {
		b.conn.Close() //nolint:errcheck
		b.conn = nil

		return nil, 0, errors.New("expected *net.UDPAddr")
	}

	// Capture udpConn in closure to avoid race with Close setting b.conn = nil.
	// This matches how StdNetBind.makeReceiveIPv4/6 works.
	recvFunc := func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		n, addr, err := udpConn.ReadFromUDP(bufs[0])
		if err != nil {
			return 0, err
		}

		sizes[0] = n
		eps[0] = &conn.StdNetEndpoint{AddrPort: addr.AddrPort()}

		return 1, nil
	}

	return []conn.ReceiveFunc{recvFunc}, uint16(laddr.Port), nil
}

func (b *boundBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	// Copy conn under lock to avoid race with Close.
	// This matches how StdNetBind.Send works.
	b.mu.Lock()
	c := b.conn
	b.mu.Unlock()

	if c == nil {
		return syscall.EAFNOSUPPORT
	}

	dst, ok := ep.(*conn.StdNetEndpoint)
	if !ok {
		return conn.ErrWrongEndpointType
	}

	for _, buf := range bufs {
		_, err := c.WriteToUDP(buf, net.UDPAddrFromAddrPort(dst.AddrPort))
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *boundBind) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn == nil {
		return nil
	}

	err := b.conn.Close()
	b.conn = nil

	return err
}

func (b *boundBind) SetMark(uint32) error { return nil }

func (b *boundBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}

	return &conn.StdNetEndpoint{AddrPort: ap}, nil
}

func (b *boundBind) BatchSize() int { return 1 }
