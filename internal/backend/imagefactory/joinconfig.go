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
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	siderolinkconfig "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
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
	result.Customization.EmbeddedMachineConfiguration = dropDocuments(base.Customization.EmbeddedMachineConfiguration, siderolink.IsJoinConfigDocument)

	if !quirks.New(talosVersion).SupportsEmbeddedConfig() {
		result.Customization.ExtraKernelArgs = slices.Concat(joinKernelArgs, result.Customization.ExtraKernelArgs)

		return result, nil
	}

	result.Customization.EmbeddedMachineConfiguration = appendDocuments(result.Customization.EmbeddedMachineConfiguration, string(joinDocuments))

	return result, nil
}

// WithTrustedRoots returns the schematic with Omni's trusted roots document replaced by one carrying the given CA certificates (PEM),
// or dropped when there are none, when the schematic carries a SideroLink config, from Talos 1.12 on: the CA travels with the join config.
// Nothing else changes, so the documents an infra provider embeds itself, its join documents among them, stay as they are.
func WithTrustedRoots(base schematic.Schematic, trustedRoots, talosVersion string) (schematic.Schematic, error) {
	if !quirks.New(talosVersion).SupportsEmbeddedConfig() || !hasSideroLinkConfig(base) {
		return base, nil
	}

	result := base
	result.Customization.EmbeddedMachineConfiguration = dropDocuments(base.Customization.EmbeddedMachineConfiguration, isTrustedRootsDocument)

	if trustedRoots == "" {
		return result, nil
	}

	cfg, err := container.New(siderolink.TrustedRootsDocument(trustedRoots))
	if err != nil {
		return schematic.Schematic{}, err
	}

	document, err := cfg.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return schematic.Schematic{}, err
	}

	result.Customization.EmbeddedMachineConfiguration = appendDocuments(result.Customization.EmbeddedMachineConfiguration, string(document))

	return result, nil
}

// hasSideroLinkConfig reports whether the schematic carries a SideroLink config, as join kernel args or as an embedded document.
func hasSideroLinkConfig(base schematic.Schematic) bool {
	if slices.ContainsFunc(base.Customization.ExtraKernelArgs, kernelargs.IsJoinArg) {
		return true
	}

	return slices.ContainsFunc(documentSeparator.Split(base.Customization.EmbeddedMachineConfiguration, -1), func(chunk string) bool {
		provider, err := configloader.NewFromBytes([]byte(chunk))

		return err == nil && slices.ContainsFunc(provider.Documents(), func(document config.Document) bool {
			return document.Kind() == siderolinkconfig.Kind
		})
	})
}

func isTrustedRootsDocument(document config.Document) bool {
	return document.Kind() == security.TrustedRootsConfig && siderolink.IsJoinConfigDocument(document)
}

func appendDocuments(embedded, documents string) string {
	if embedded == "" {
		return documents
	}

	if !strings.HasSuffix(embedded, "\n") {
		embedded += "\n"
	}

	return embedded + "---\n" + documents
}

var documentSeparator = regexp.MustCompile(`(?m)^---[ \t]*(#.*)?\r?\n`)

// dropDocuments drops the matching documents from the config as text, so the other documents stay as they are written (only the separators between them are normalized).
// A chunk that does not parse, like a comment or a document kind this Omni does not know, is kept.
func dropDocuments(embedded string, drop func(config.Document) bool) string {
	chunks := documentSeparator.Split(embedded, -1)

	kept := xslices.Filter(chunks, func(chunk string) bool {
		provider, err := configloader.NewFromBytes([]byte(chunk))
		if err != nil {
			return true
		}

		return !slices.ContainsFunc(provider.Documents(), drop)
	})

	if len(kept) == len(chunks) {
		return embedded
	}

	return strings.Join(kept, "---\n")
}
