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
// If the schematic meta extension carries the raw manifest (Talos v1.7+),
// we take the extension list from it directly instead of reconstructing it from the extension status resources, so virtual and meta extensions cannot leak into it.
//
// The argument fallbackKernelArgs is only used if the machine doesn't have the schematic meta extension, i.e., its installation media was created bypassing image factory -
// in that case, we synthesize the schematic ID in a best-effort way (only if it doesn't have any extensions), and use the provided fallback kernel args as the current args of the machine.
func GetSchematicInfo(ctx context.Context, talosState state.CoreState, fallbackKernelArgs []string) (SchematicInfo, error) {
	items, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, talosState)
	if err != nil {
		return SchematicInfo{}, fmt.Errorf("failed to list extensions: %w", err)
	}

	var (
		observedExts []string
		fullID       string
		rawSchematic *schematic.Schematic
		manifest     string
	)

	for status := range items.All() {
		name := status.TypedSpec().Metadata.Name

		switch name {
		case extensions.MetalAgentExtensionName:
			return SchematicInfo{InAgentMode: true}, nil
		case constants.SchematicIDExtensionName: // skip the meta extension
			meta := status.TypedSpec().Metadata
			fullID = meta.Version

			if meta.ExtraInfo != "" {
				manifest = meta.ExtraInfo

				if rawSchematic, err = schematic.Unmarshal([]byte(manifest)); err != nil {
					return SchematicInfo{}, fmt.Errorf("failed to unmarshal schematic manifest: %w", err)
				}
			}
		case "modules.dep": // ignore the virtual extension used for kernel modules dependencies
		case "embedded-config": // ignore the meta extension reported by machines carrying an embedded machine configuration
		default:
			if !strings.HasPrefix(name, extensions.OfficialPrefix) {
				name = extensions.OfficialPrefix + name
			}

			observedExts = append(observedExts, name)
		}
	}

	// Prefer the raw schematic manifest as the definitive source of the extension list, since it records exactly the
	// user-requested extensions. Otherwise reconstruct the list from the observed extension status resources. Either
	// way, normalize the known wrong manifest names.
	exts := observedExts
	if rawSchematic != nil {
		exts = rawSchematic.Customization.SystemExtensions.OfficialExtensions
	}

	exts = extensions.MapNames(exts)

	if rawSchematic != nil { // the manifest also carries the definitive kernel args and raw YAML
		return SchematicInfo{
			FullID:     fullID,
			Extensions: exts,
			KernelArgs: rawSchematic.Customization.ExtraKernelArgs,
			Raw:        manifest,
		}, nil
	}

	if fullID == "" && len(exts) > 0 {
		return SchematicInfo{}, ErrInvalidSchematic
	}

	if fullID == "" { // we could not find the full ID, so we fall back to synthesizing it (and the raw YAML) using the default args
		synthesized := schematic.Schematic{
			Customization: schematic.Customization{
				SystemExtensions: schematic.SystemExtensions{
					OfficialExtensions: exts,
				},
				ExtraKernelArgs: fallbackKernelArgs,
			},
		}

		fullID, err = synthesized.ID()
		if err != nil {
			return SchematicInfo{}, fmt.Errorf("failed to calculate full schematic ID: %w", err)
		}

		raw, err := synthesized.Marshal()
		if err != nil {
			return SchematicInfo{}, fmt.Errorf("failed to marshal synthesized schematic: %w", err)
		}

		return SchematicInfo{
			FullID:     fullID,
			Extensions: exts,
			KernelArgs: fallbackKernelArgs,
			Raw:        string(raw),
		}, nil
	}

	// The meta extension is present (so the full ID is known) but it carries no raw manifest: report the extension list
	// reconstructed from the observed extension status resources, with no kernel args nor raw manifest available.
	return SchematicInfo{
		FullID:     fullID,
		Extensions: exts,
	}, nil
}
