// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package siderolink implements siderolink manager.
package siderolink

import (
	"net/netip"

	"github.com/siderolabs/siderolink/pkg/wireguard"
)

// ListenHost is the host SideroLink should listen.
var ListenHost string

func init() {
	siderolinkNetworkPrefix := wireguard.NetworkPrefix("")

	ListenHost = netip.PrefixFrom(siderolinkNetworkPrefix.Addr().Next(), siderolinkNetworkPrefix.Bits()).Addr().String()
}
