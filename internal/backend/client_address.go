// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package backend

import (
	"net/http"
	"net/netip"
	"strings"
)

const forwardedForHeader = "X-Forwarded-For"

// Used by pod networks on some clouds (e.g., EKS with custom networking), and IsPrivate does not cover it.
var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

// setClientAddress rewrites X-Forwarded-For to the single address returned by clientAddress.
//
// It also removes Grpc-Metadata-X-Forwarded-For, as the gateway would put its value before the rewritten one.
func setClientAddress(req *http.Request) {
	req.Header.Del("Grpc-Metadata-" + forwardedForHeader)

	if client := clientAddress(req.RemoteAddr, req.Header.Values(forwardedForHeader)); client != "" {
		req.Header.Set(forwardedForHeader, client)
	} else {
		req.Header.Del(forwardedForHeader)
	}
}

// clientAddress returns the client address the same way Rails does by default. It starts with the connection
// address and moves through the X-Forwarded-For entries from right to left while the address is private, i.e., a
// proxy in front of Omni. The first public address is the client.
//
// Example: a connection from 10.0.0.5 with "X-Forwarded-For: 1.2.3.4, 203.0.113.9, 10.20.0.9" returns 203.0.113.9.
// The client-sent 1.2.3.4 is never read, as only proxies write to the right of it.
//
// A client on a private network that reaches Omni directly can still set any address.
func clientAddress(remoteAddr string, forwardedFor []string) string {
	addr, ok := parseAddress(remoteAddr)
	if !ok {
		return ""
	}

	entries := strings.Split(strings.Join(forwardedFor, ","), ",")

	for i := len(entries) - 1; i >= 0 && isProxyAddress(addr); i-- {
		next, ok := parseAddress(strings.TrimSpace(entries[i]))
		if !ok {
			break
		}

		addr = next
	}

	return addr.String()
}

// parseAddress parses a bare address or host:port, as RemoteAddr and some proxies (e.g., Azure Application Gateway)
// write it.
func parseAddress(s string) (netip.Addr, bool) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr, true
	}

	addrPort, err := netip.ParseAddrPort(s)

	return addrPort.Addr(), err == nil
}

func isProxyAddress(addr netip.Addr) bool {
	return addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || sharedAddressSpace.Contains(addr.Unmap())
}
