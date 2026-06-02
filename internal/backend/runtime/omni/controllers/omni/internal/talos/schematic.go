// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"

	"github.com/siderolabs/omni/internal/backend/extensions"
)

// ErrInvalidSchematic means that the machine has extensions installed bypassing the image factory.
var ErrInvalidSchematic = fmt.Errorf("invalid schematic")

// SchematicInfo contains the information about the schematic observed on a machine.
type SchematicInfo struct {
	FullID      string
	Raw         string
	Extensions  []string
	KernelArgs  []string
	InAgentMode bool
}

// GetSchematicInfo lists the extension status resources on the given Talos COSI state, and computes the schematic ID
// from the extensions found on the machine, ignoring everything else (e.g., the kernel command line args).
//
// The argument fallbackKernelArgs is only used if the machine doesn't have the schematic meta extension, i.e., its installation media was created bypassing image factory -
// in that case, we synthesize the schematic ID in a best-effort way (only if it doesn't have any extensions), and use the provided fallback kernel args as the current args of the machine.
func GetSchematicInfo(ctx context.Context, talosState state.CoreState, fallbackKernelArgs []string) (SchematicInfo, error) {
	items, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, talosState)
	if err != nil {
		return SchematicInfo{}, fmt.Errorf("failed to list extensions: %w", err)
	}

	var (
		exts         []string
		fullID       string
		rawSchematic *schematic.Schematic
		manifest     string
	)

	for status := range items.All() {
		name := status.TypedSpec().Metadata.Name
		if name == extensions.MetalAgentExtensionName {
			return SchematicInfo{InAgentMode: true}, nil
		}

		if name == constants.SchematicIDExtensionName { // skip the meta extension
			fullID = status.TypedSpec().Metadata.Version

			if status.TypedSpec().Metadata.ExtraInfo != "" {
				manifest = status.TypedSpec().Metadata.ExtraInfo

				if rawSchematic, err = schematic.Unmarshal([]byte(manifest)); err != nil {
					return SchematicInfo{}, fmt.Errorf("failed to unmarshal schematic manifest: %w", err)
				}
			}

			continue
		}

		if name == "modules.dep" { // ignore the virtual extension used for kernel modules dependencies
			continue
		}

		if !strings.HasPrefix(name, extensions.OfficialPrefix) {
			name = extensions.OfficialPrefix + name
		}

		exts = append(exts, name)
	}

	exts = extensions.MapNames(exts)

	if fullID == "" && len(exts) > 0 {
		return SchematicInfo{}, ErrInvalidSchematic
	}

	var kernelArgs []string

	if rawSchematic != nil {
		kernelArgs = rawSchematic.Customization.ExtraKernelArgs
	}

	if fullID == "" { // we could not find the full ID, so we fall back to synthesizing it (and the raw YAML) using the default args
		kernelArgs = fallbackKernelArgs

		synthesized := schematic.Schematic{
			Customization: schematic.Customization{
				SystemExtensions: schematic.SystemExtensions{
					OfficialExtensions: exts,
				},
				ExtraKernelArgs: fallbackKernelArgs,
			},
		}

		var err error

		fullID, err = synthesized.ID()
		if err != nil {
			return SchematicInfo{}, fmt.Errorf("failed to calculate full schematic ID: %w", err)
		}

		raw, err := synthesized.Marshal()
		if err != nil {
			return SchematicInfo{}, fmt.Errorf("failed to marshal synthesized schematic: %w", err)
		}

		manifest = string(raw)
	}

	return SchematicInfo{
		FullID:     fullID,
		Extensions: exts,
		KernelArgs: kernelArgs,
		Raw:        manifest,
	}, nil
}
