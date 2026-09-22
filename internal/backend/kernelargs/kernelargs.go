// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package kernelargs contains logic and utilities for managing extra kernel arguments.
package kernelargs

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// protectedKeys are set by Omni. They are taken from the machine's current schematic, never from the KernelArgs resource.
var protectedKeys = []string{
	constants.KernelParamSideroLink, constants.KernelParamEventsSink, constants.KernelParamLoggingKernel,
	constants.KernelParamConfig, constants.KernelParamConfigEarly, constants.KernelParamConfigInline,
}

// forbiddenKeys break the boot or the connection to Omni once baked into the installer image.
var forbiddenKeys = []string{
	constants.KernelParamPlatform,        // the imager replaces the platform with it
	constants.KernelParamWipe,            // wipes the disk on every boot
	constants.KernelParamHaltIfInstalled, // halts every boot from disk
	"talos.board",                        // Talos 1.12 and older refuse to install with a board set and no overlay, Omni supports 1.9 and later
}

// ksppKeys are the KSPP parameters Talos refuses to boot without, so their negation is rejected.
var ksppKeys = []string{"slab_nomerge", "pti"}

type Initializer struct {
	state  state.State
	logger *zap.Logger
}

func NewInitializer(state state.State, logger *zap.Logger) (*Initializer, error) {
	if state == nil {
		return nil, fmt.Errorf("state is nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Initializer{
		state:  state,
		logger: logger,
	}, nil
}

func (initializer *Initializer) Init(ctx context.Context, id resource.ID, args []string) error {
	extraArgs := FilterExtras(args)

	if len(extraArgs) == 0 {
		return nil
	}

	kernelArgs := omni.NewKernelArgs(id)
	kernelArgs.TypedSpec().Value.Args = extraArgs

	if err := initializer.state.Create(ctx, kernelArgs); err != nil && !state.IsConflictError(err) {
		return fmt.Errorf("error creating extra kernel args configuration: %w", err)
	}

	return nil
}

func UpdateSupported(machineStatus *omni.MachineStatus, getClusterMachineConfig func() (*omni.ClusterMachineConfig, error)) (bool, error) {
	securityState := machineStatus.TypedSpec().Value.SecurityState
	talosVersion := machineStatus.TypedSpec().Value.TalosVersion

	if securityState == nil && talosVersion == "" {
		return false, fmt.Errorf("missing security state and Talos version")
	}

	if securityState != nil && securityState.BootedWithUki {
		return true, nil
	}

	parsedTalosVersion, err := semver.ParseTolerant(talosVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse Talos version %q: %w", talosVersion, err)
	}

	// If the update is supported not because the machine is booted with UKI, but because its version is >= 1.12,
	// we need to additionally check that GrubUseUKICmdline is set to true if the machine is not in maintenance mode.
	//
	// This can happen with the machines which were allocated to a cluster that was created with an older version of Talos,
	// and their .machine.install.extraKernelArgs field was populated via a ConfigPatch.
	if parsedTalosVersion.Major == 1 && parsedTalosVersion.Minor < 12 {
		return false, nil
	}

	if getClusterMachineConfig == nil {
		return true, nil
	}

	clusterMachineConfig, err := getClusterMachineConfig()
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get cluster machine config: %w", err)
	}

	if clusterMachineConfig == nil {
		return true, nil
	}

	return clusterMachineConfig.TypedSpec().Value.GrubUseUkiCmdline, nil
}

// Calculate returns the protected args of the machine's current schematic followed by the user's extra args.
// When that is logically equal to the current args, the current args are returned as they are, so a cosmetic difference never upgrades a machine.
func Calculate(machineStatus *omni.MachineStatus, kernelArgs *omni.KernelArgs) (args []string, initialized bool, err error) {
	if !machineStatus.TypedSpec().Value.SchematicReady() {
		return nil, false, nil
	}

	if _, initialized = machineStatus.Metadata().Annotations().Get(omni.KernelArgsInitialized); !initialized {
		return nil, false, nil
	}

	var extraArgs []string

	if kernelArgs != nil {
		extraArgs = FilterExtras(kernelArgs.TypedSpec().Value.Args) // a protected arg in the resource is not an extra, otherwise it is added on every upgrade
	}

	currentArgs := machineStatus.TypedSpec().Value.Schematic.KernelArgs

	// Only the extra args are compared, as an ordered list. The protected args always come from the current args, so they cannot differ,
	// and their duplicates are cleaned up only when the extra args change and the machine is upgraded anyway.
	if slices.Equal(FilterExtras(currentArgs), extraArgs) {
		return currentArgs, true, nil
	}

	baseArgs := xslices.Deduplicate(FilterProtected(currentArgs), func(arg string) string { return arg })

	return slices.Concat(baseArgs, extraArgs), true, nil
}

// Validate rejects kernel args a user must not set: protected args, forbidden args and their negations, and entries holding more than one arg.
func Validate(args []string) error {
	for _, arg := range args {
		if strings.ContainsFunc(arg, unicode.IsSpace) {
			return fmt.Errorf("kernel arg %q must be a single argument", arg)
		}

		if isProtected(arg) || isForbidden(arg) {
			return fmt.Errorf("kernel arg %q is not allowed, remove it", arg)
		}
	}

	return nil
}

// FilterProtected filters out the "extra args" from the provided kernel args, leaving only the protected kernel arguments that cannot be modified.
func FilterProtected(args []string) []string {
	return xslices.Filter(args, isProtected)
}

// FilterExtras filters out the protected kernel arguments from the provided kernel args, leaving only the "extra args" that can be modified.
func FilterExtras(args []string) []string {
	return xslices.Filter(args, func(value string) bool {
		return !isProtected(value)
	})
}

// isProtected matches a protected key in any whitespace-separated token, since the imager re-splits entries.
// A negated protected key is not protected: it is an extra the user can still remove, only new ones are rejected by Validate.
func isProtected(arg string) bool {
	return slices.ContainsFunc(strings.Fields(arg), func(token string) bool {
		return hasKey(token, protectedKeys)
	})
}

func isForbidden(arg string) bool {
	if negated, ok := strings.CutPrefix(arg, "-"); ok {
		return hasKey(negated, protectedKeys) || hasKey(negated, forbiddenKeys) || hasKey(negated, ksppKeys)
	}

	return hasKey(arg, forbiddenKeys)
}

func hasKey(token string, keys []string) bool {
	return slices.ContainsFunc(keys, func(key string) bool {
		return token == key || strings.HasPrefix(token, key+"=")
	})
}
