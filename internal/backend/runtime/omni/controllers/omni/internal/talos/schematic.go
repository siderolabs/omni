// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ErrInvalidSchematic means that the machine has extensions installed bypassing the image factory.
var ErrInvalidSchematic = fmt.Errorf("invalid schematic")

// GetSchematicID calculates schematic using Talos client: reads extensions list and looks up schematic meta
// extension, or calculates vanilla schematic ID if there's none.
func GetSchematicID(ctx context.Context, c *client.Client) (string, error) {
	extensions := map[resource.ID]*runtime.ExtensionStatus{}

	items, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, c.COSI)
	if err != nil {
		return "", err
	}

	items.ForEach(func(status *runtime.ExtensionStatus) {
		extensions[status.TypedSpec().Metadata.Name] = status
	})

	schematicExtension, ok := extensions[constants.SchematicIDExtensionName]

	if !ok && len(extensions) > 0 {
		return "", ErrInvalidSchematic
	}

	// default schematic
	if !ok {
		return (&schematic.Schematic{}).ID()
	}

	return schematicExtension.TypedSpec().Metadata.Version, nil
}
