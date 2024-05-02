// Copyright (c) 2024 Sidero Labs, Inc.
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
	"gopkg.in/yaml.v3"
)

// ErrInvalidSchematic means that the machine has extensions installed bypassing the image factory.
var ErrInvalidSchematic = fmt.Errorf("invalid schematic")

// SchematicInfo contains the information about the schematic - the plain schematic ID and the extensions.
type SchematicInfo struct {
	ID         string
	FullID     string
	Extensions []string
	Schematic  schematic.Schematic
}

// Equal compares schematic id with both extensions ID and Full ID.
func (si SchematicInfo) Equal(id string) bool {
	return si.ID == id || si.FullID == id
}

// GetSchematicInfo uses Talos API to list all the schematics, and computes the plain schematic ID,
// taking only the extensions into account - ignoring everything else, e.g., the kernel command line args or meta values.
func GetSchematicInfo(ctx context.Context, c *client.Client) (SchematicInfo, error) {
	const officialExtensionPrefix = "siderolabs/"

	items, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, c.COSI)
	if err != nil {
		return SchematicInfo{}, fmt.Errorf("failed to list extensions: %w", err)
	}

	var (
		extensions             []string
		schematicID            string
		schematicConfiguration schematic.Schematic
	)

	err = items.ForEachErr(func(status *runtime.ExtensionStatus) error {
		name := status.TypedSpec().Metadata.Name
		if name == constants.SchematicIDExtensionName { // skip the meta extension
			schematicID = status.TypedSpec().Metadata.Version

			if status.TypedSpec().Metadata.ExtraInfo != "" {
				return yaml.Unmarshal([]byte(status.TypedSpec().Metadata.ExtraInfo), &schematicConfiguration)
			}

			return nil
		}

		if name == "modules.dep" { // ignore the virtual extension used for kernel modules dependencies
			return nil
		}

		switch name {
		case "v4l-uvc":
			fallthrough
		case "usb-modem":
			name += "-drivers"
		case "gasket":
			name += "-driver"
		case "talos-vmtoolsd":
			name = "vmtoolsd-guest-agent"
		}

		if !strings.HasPrefix(name, officialExtensionPrefix) {
			name = officialExtensionPrefix + name
		}

		extensions = append(extensions, name)

		return nil
	})
	if err != nil {
		return SchematicInfo{}, err
	}

	if schematicID == "" && len(extensions) > 0 {
		return SchematicInfo{}, ErrInvalidSchematic
	}

	extensionsSchematic := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: extensions,
			},
		},
	}

	id, err := extensionsSchematic.ID()
	if err != nil {
		return SchematicInfo{}, fmt.Errorf("failed to calculate extensions schematic ID: %w", err)
	}

	return SchematicInfo{
		ID:         id,
		FullID:     schematicID,
		Extensions: extensions,
		Schematic:  schematicConfiguration,
	}, nil
}
