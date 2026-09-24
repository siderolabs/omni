// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory

import (
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"

	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
)

// WithJoinConfig returns the schematic with its SideroLink join configuration replaced by the given one, in the form the Talos version supports:
// embedded machine configuration from 1.12 on, kernel args below. Omni-managed join args and documents (e.g., SideroLink) are dropped first, everything else is kept.
func WithJoinConfig(base schematic.Schematic, joinKernelArgs []string, joinDocuments []byte, talosVersion string) (schematic.Schematic, error) {
	if len(joinKernelArgs) == 0 || len(joinDocuments) == 0 {
		return schematic.Schematic{}, errors.New("join config is empty")
	}

	result := base

	result.Customization.ExtraKernelArgs = xslices.Filter(base.Customization.ExtraKernelArgs, func(arg string) bool { return !kernelargs.IsJoinArg(arg) })
	result.Customization.EmbeddedMachineConfiguration = dropJoinDocuments(base.Customization.EmbeddedMachineConfiguration)

	if !quirks.New(talosVersion).SupportsEmbeddedConfig() {
		result.Customization.ExtraKernelArgs = slices.Concat(joinKernelArgs, result.Customization.ExtraKernelArgs)

		return result, nil
	}

	result.Customization.EmbeddedMachineConfiguration = appendDocuments(result.Customization.EmbeddedMachineConfiguration, string(joinDocuments))

	return result, nil
}

func appendDocuments(config, documents string) string {
	if config == "" {
		return documents
	}

	if !strings.HasSuffix(config, "\n") {
		config += "\n"
	}

	return config + "---\n" + documents
}

var documentSeparator = regexp.MustCompile(`(?m)^---[ \t]*(#.*)?\r?\n`)

// dropJoinDocuments drops the Omni-managed documents (e.g., SideroLink) from the config as text, so the other documents stay as they are written (only the separators between them are normalized).
// A chunk that does not parse, like a comment or a document kind this Omni does not know, is kept.
func dropJoinDocuments(config string) string {
	chunks := documentSeparator.Split(config, -1)

	kept := xslices.Filter(chunks, func(chunk string) bool {
		provider, err := configloader.NewFromBytes([]byte(chunk))
		if err != nil {
			return true
		}

		return !slices.ContainsFunc(provider.Documents(), siderolink.IsJoinConfigDocument)
	})

	if len(kept) == len(chunks) {
		return config
	}

	return strings.Join(kept, "---\n")
}
