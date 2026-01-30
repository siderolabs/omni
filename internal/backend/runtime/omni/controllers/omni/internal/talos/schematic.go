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
	"github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/internal/backend/extensions"
)

// ErrInvalidSchematic means that the machine has extensions installed bypassing the image factory.
var ErrInvalidSchematic = fmt.Errorf("invalid schematic")

// SchematicInfo contains the information about the schematic - the plain schematic ID and the extensions.
type SchematicInfo struct {
	ID          string
	FullID      string
	Raw         string
	Overlay     schematic.Overlay
	Extensions  []string
	KernelArgs  []string
	MetaValues  []schematic.MetaValue
	InAgentMode bool
}

// Equal compares schematic id with both extensions ID and Full ID.
func (si SchematicInfo) Equal(id string) bool {
	return si.ID == id || si.FullID == id
}

// GetSchematicInfo uses Talos API to list all the schematics, and computes the plain schematic ID,
// taking only the extensions into account - ignoring everything else, e.g., the kernel command line args or meta values.
func GetSchematicInfo(ctx context.Context, c *client.Client, defaultKernelArgs []string) (SchematicInfo, error) {
	items, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, c.COSI)
	if err != nil {
		return SchematicInfo{}, fmt.Errorf("failed to list extensions: %w", err)
	}

	var (
		exts         []string
		fullID       string
		rawSchematic = &schematic.Schematic{}
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

				if err = yaml.Unmarshal([]byte(manifest), rawSchematic); err != nil {
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

	extensionsSchematic := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: exts,
			},
		},
	}

	id, err := extensionsSchematic.ID()
	if err != nil {
		return SchematicInfo{}, fmt.Errorf("failed to calculate extensions schematic ID: %w", err)
	}

	var (
		kernelArgs []string
		metaValues []schematic.MetaValue
		overlay    schematic.Overlay
	)

	if rawSchematic != nil {
		kernelArgs = rawSchematic.Customization.ExtraKernelArgs
		metaValues = rawSchematic.Customization.Meta
		overlay = rawSchematic.Overlay
	}

	if fullID == "" { // we could not find the full ID, so we fall back to synthesizing it using the default args
		kernelArgs = defaultKernelArgs
		extensionsSchematic.Customization.ExtraKernelArgs = defaultKernelArgs

		fullID, err = extensionsSchematic.ID()
		if err != nil {
			return SchematicInfo{}, fmt.Errorf("failed to calculate full schematic ID: %w", err)
		}
	}

	return SchematicInfo{
		ID:         id,
		FullID:     fullID,
		Extensions: exts,
		KernelArgs: kernelArgs,
		MetaValues: metaValues,
		Overlay:    overlay,
		Raw:        manifest,
	}, nil
}
