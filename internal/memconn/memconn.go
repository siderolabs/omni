// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package memconn provides a memory-based net.Conn implementation.
package memconn

import (
	"errors"
	"net"

	"github.com/akutz/memconn"
)

// Transport is transport for in-memory connection.
type Transport struct {
	Address string
}

// Listener creates new listener.
func (l *Transport) Listener() (net.Listener, error) {
	if l.Address == "" {
		return nil, errors.New("address is not set")
	}

	return memconn.Listen("memu", l.Address)
}

// Dial creates a new connection.
func (l *Transport) Dial() (net.Conn, error) {
	if l.Address == "" {
		return nil, errors.New("address is not set")
	}

	return memconn.Dial("memu", l.Address)
}
