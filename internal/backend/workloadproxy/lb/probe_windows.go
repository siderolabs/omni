// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build windows

package lb

import (
	"net"
	"syscall"
)

func probeDialer() *net.Dialer {
	return &net.Dialer{
		// The dialer reduces the TIME-WAIT period to 1 seconds instead of the OS default of 60 seconds.
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptLinger(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{Onoff: 1, Linger: 1}) //nolint: errcheck
			})
		},
	}
}
