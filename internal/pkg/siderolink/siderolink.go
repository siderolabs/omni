// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package siderolink implements siderolink manager.
package siderolink

import (
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// ListenHost is the host SideroLink should listen.
var ListenHost string

func init() {
	ListenHost = siderolink.GetListenHost()
}
