// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory

import (
	"errors"
	"fmt"

	"github.com/siderolabs/image-factory/pkg/schematic"
)

// ErrEmptyRawSchematic is returned by PatchSchematic when the input raw YAML is empty.
var ErrEmptyRawSchematic = errors.New("raw schematic YAML is empty")

// PatchSchematic parses the given raw schematic YAML and overwrites only the
// two fields that Omni manages on behalf of the user: the official extensions
// list and the extra kernel args.
//
// All other fields (owner, secureboot, bootloader, meta, overlay incl.
// options, and any field added by future image-factory versions) are
// preserved by going through the same schematic.Schematic struct the factory
// uses for its canonical representation.
//
// The unmarshal is strict: any field the bundled image-factory library does
// not know about causes an error. When that happens, the image-factory module
// version in this repo must be bumped to catch up with the factory.
func PatchSchematic(rawYAML string, extensions, kernelArgs []string) (schematic.Schematic, error) {
	if rawYAML == "" {
		return schematic.Schematic{}, ErrEmptyRawSchematic
	}

	parsed, err := schematic.Unmarshal([]byte(rawYAML))
	if err != nil {
		return schematic.Schematic{}, fmt.Errorf("failed to unmarshal raw schematic: %w", err)
	}

	parsed.Customization.SystemExtensions.OfficialExtensions = extensions
	parsed.Customization.ExtraKernelArgs = kernelArgs

	return *parsed, nil
}
