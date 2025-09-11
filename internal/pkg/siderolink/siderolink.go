// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package siderolink implements siderolink manager.
package siderolink

import (
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/constants"

	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// ListenHost is the host SideroLink should listen.
var ListenHost string

func init() {
	ListenHost = siderolink.GetListenHost()
}

// IsSiderolinkKernelArg checks if the given kernel argument is related to SideroLink.
func IsSiderolinkKernelArg(arg string) bool {
	for _, prefix := range []string{constants.KernelParamSideroLink, constants.KernelParamEventsSink, constants.KernelParamLoggingKernel} {
		if strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}

	return false
}
