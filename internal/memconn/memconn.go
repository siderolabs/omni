// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package memconn provides a memory-based net.Conn implementation.
package memconn

import (
	"context"
	"errors"
	"net"

	"github.com/akutz/memconn"
)

// NewTransport creates a new transport.
func NewTransport(address string) *Transport {
	return &Transport{address: address}
}

// Transport is transport for in-memory connection.
type Transport struct {
	address string
}

// Listener creates new listener.
func (l *Transport) Listener() (net.Listener, error) {
	if l.address == "" {
		return nil, errors.New("address is not set")
	}

	return memconn.Listen("memu", l.address)
}

// DialContext creates a new connection.
func (l *Transport) DialContext(ctx context.Context) (net.Conn, error) {
	if l.address == "" {
		return nil, errors.New("address is not set")
	}

	return memconn.DialContext(ctx, "memu", l.address)
}

// Address returns the address. Since this is a memory-based connection, the address is always "passthrough:" + address,
// because the address is not a real network address and gRPC tries to resolve it otherwise.
func (l *Transport) Address() string {
	return "passthrough:" + l.address
}
