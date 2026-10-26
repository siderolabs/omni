// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package metrics

import (
	"slices"
	"strconv"
	"strings"

	siderolinkconfig "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
)

const (
	joinConfigKernelArgs    = "kernel_args"
	joinConfigEmbedded      = "embedded_config"
	joinConfigMachineConfig = "machine_config"
	joinConfigUnknown       = "unknown"
)

// joinConfigLocation returns where the SideroLink join config of the machine is: in the embedded machine config of its schematic, in its kernel args, or in its machine config.
//
// The embedded machine config is checked first, as a machine booting with GRUB without `grubUseUKICmdline` keeps the join args on its kernel command line across upgrades,
// also after its schematic carries the join config.
//
// The kernel args are read from the kernel command line, whatever put them there (the schematic, the bootloader, PXE).
// Below Talos 1.11 it is read from /proc/cmdline, which the maintenance API does not serve, so it is empty for those machines
// in maintenance: the kernel args of the schematic are checked instead, and without join args in them the location is unknown,
// as the bootloader may still have added them.
//
// The embedded machine config is checked as text, so the schematic is not parsed for every machine each time the metrics are gathered.
func joinConfigLocation(ms *omni.MachineStatus) string {
	spec := ms.TypedSpec().Value

	switch {
	case strings.Contains(spec.GetSchematic().GetRaw(), "kind: "+siderolinkconfig.Kind):
		return joinConfigEmbedded
	case kernelargs.IsJoinArg(spec.KernelCmdline):
		return joinConfigKernelArgs
	case !spec.SchematicReady():
		return joinConfigUnknown
	case spec.GetSchematic().GetRaw() == "": // the schematic ID is known, but not its content
		return joinConfigUnknown
	case spec.KernelCmdline == "" && slices.ContainsFunc(spec.GetSchematic().GetKernelArgs(), kernelargs.IsJoinArg):
		return joinConfigKernelArgs
	case spec.KernelCmdline == "":
		return joinConfigUnknown
	default:
		return joinConfigMachineConfig
	}
}

// trustedRootsPresent returns whether the schematic the machine runs carries the trusted roots document Omni generates.
// A machine without it loses its connection to Omni after a reset when Omni's machine API certificate is signed by those roots.
func trustedRootsPresent(ms *omni.MachineStatus) string {
	spec := ms.TypedSpec().Value
	if !spec.SchematicReady() || spec.GetSchematic().GetRaw() == "" {
		return joinConfigUnknown
	}

	return strconv.FormatBool(strings.Contains(spec.GetSchematic().GetRaw(), "name: "+siderolink.TrustedRootsConfigName))
}
