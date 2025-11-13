// Copyright (c) 2025 Sidero Labs, Inc.
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

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

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
	extraArgs := xslices.Filter(args, func(value string) bool { return !isProtected(value) })

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

func UpdateSupported(machineStatus *omni.MachineStatus) (bool, error) {
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

	return parsedTalosVersion.GTE(semver.MustParse("1.12.0")), nil
}

func Calculate(machineStatus *omni.MachineStatus, kernelArgs *omni.KernelArgs) (args []string, initialized bool, err error) {
	schematicConfig := machineStatus.TypedSpec().Value.Schematic
	if schematicConfig == nil {
		return nil, false, nil
	}

	if _, initialized = machineStatus.Metadata().Annotations().Get(omni.KernelArgsInitialized); !initialized {
		return nil, false, nil
	}

	var extraArgs []string

	if kernelArgs != nil {
		extraArgs = kernelArgs.TypedSpec().Value.Args
	}

	baseArgs := xslices.Filter(schematicConfig.KernelArgs, isProtected)

	return slices.Concat(baseArgs, extraArgs), true, nil
}

// FilterExtras filters out the protected kernel arguments from the provided kernel args, leaving only the "extra args" that can be modified.
func FilterExtras(args []string) []string {
	return xslices.Filter(args, func(value string) bool {
		return !isProtected(value)
	})
}

func isProtected(arg string) bool {
	for _, prefix := range []string{
		constants.KernelParamSideroLink, constants.KernelParamEventsSink, constants.KernelParamLoggingKernel,
		constants.KernelParamConfig, constants.KernelParamConfigEarly, constants.KernelParamConfigInline,
	} {
		if strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}

	return false
}

// GetUncached reads the KernelArgs resource with the given ID, bypassing the resource cache.
//
// We need this to avoid stale reads of the KernelArgs resource: there can be cases where the omni.KernelArgsInitialized annotation is present in the MachineStatus,
// but the KernelArgs resource is not yet visible due to the resource cache, which can cause unwanted Talos upgrades through a schematic id update.
func GetUncached(ctx context.Context, r controller.Reader, id resource.ID) (*omni.KernelArgs, error) {
	uncachedReader, ok := r.(controller.UncachedReader)
	if !ok {
		return nil, fmt.Errorf("reader does not support uncached reads")
	}

	kernelArgs, err := uncachedReader.GetUncached(ctx, omni.NewKernelArgs(id).Metadata())
	if err != nil {
		return nil, fmt.Errorf("error getting extra kernel args configuration: %w", err)
	}

	kernelArgsTyped, ok := kernelArgs.(*omni.KernelArgs)
	if !ok {
		return nil, fmt.Errorf("unexpected resource type: %T", kernelArgs)
	}

	return kernelArgsTyped, nil
}
