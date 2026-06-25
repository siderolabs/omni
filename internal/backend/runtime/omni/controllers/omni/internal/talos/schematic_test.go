// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/image-factory/pkg/schematic"
	talosextensions "github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
)

// extension describes one runtime.ExtensionStatus to seed into the test state.
type extension struct {
	name      string
	version   string
	extraInfo string
}

func buildState(t *testing.T, extensions []extension) state.State {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	for i, ext := range extensions {
		res := runtime.NewExtensionStatus(runtime.NamespaceName, strconv.Itoa(i))
		res.TypedSpec().Metadata = talosextensions.Metadata{
			Name:      ext.name,
			Version:   ext.version,
			ExtraInfo: ext.extraInfo,
		}

		require.NoError(t, st.Create(t.Context(), res))
	}

	return st
}

// schematicID computes the canonical id of the given schematic.
func schematicID(t *testing.T, s schematic.Schematic) string {
	t.Helper()

	id, err := s.ID()
	require.NoError(t, err)

	return id
}

// schematicYAML marshals the schematic to its canonical YAML form.
func schematicYAML(t *testing.T, s schematic.Schematic) string {
	t.Helper()

	data, err := s.Marshal()
	require.NoError(t, err)

	return string(data)
}

func TestGetSchematicInfo(t *testing.T) {
	t.Parallel()

	// Schematic that mimics what a machine booted with the factory would carry as the meta extension's ExtraInfo.
	factorySchematic := schematic.Schematic{
		Customization: schematic.Customization{
			ExtraKernelArgs: []string{"console=ttyS0", "talos.platform=metal"},
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: []string{"siderolabs/gvisor", "siderolabs/mdadm"},
			},
		},
	}
	factorySchematicID := schematicID(t, factorySchematic)
	factorySchematicYAML := schematicYAML(t, factorySchematic)

	t.Run("metal agent extension short-circuits and returns InAgentMode", func(t *testing.T) {
		t.Parallel()

		st := buildState(t, []extension{
			{name: "metal-agent"},
			{name: "siderolabs/gvisor"}, // would otherwise be picked up - ignored due to agent mode
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.NoError(t, err)

		assert.Equal(t, talos.SchematicInfo{InAgentMode: true}, info)
	})

	t.Run("schematic meta extension with ExtraInfo populates Raw and KernelArgs", func(t *testing.T) {
		t.Parallel()

		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: factorySchematicID, extraInfo: factorySchematicYAML},
			{name: "siderolabs/gvisor"},
			{name: "siderolabs/mdadm"},
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, []string{"this-fallback-must-not-be-used"})
		require.NoError(t, err)

		assert.Equal(t, factorySchematicID, info.FullID)
		assert.Equal(t, factorySchematicYAML, info.Raw)
		assert.Equal(t, factorySchematic.Customization.ExtraKernelArgs, info.KernelArgs)
		assert.Equal(t, []string{"siderolabs/gvisor", "siderolabs/mdadm"}, info.Extensions)
		assert.False(t, info.InAgentMode)
	})

	t.Run("schematic meta extension without ExtraInfo leaves Raw and KernelArgs empty", func(t *testing.T) {
		t.Parallel()

		// Older Talos: meta extension present (so fullID is known), but ExtraInfo is empty.
		// fullID is non-empty, so the synthesizing fallback path is NOT taken.
		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: factorySchematicID},
			{name: "siderolabs/gvisor"},
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, []string{"this-fallback-must-not-be-used"})
		require.NoError(t, err)

		assert.Equal(t, factorySchematicID, info.FullID)
		assert.Empty(t, info.Raw)
		assert.Empty(t, info.KernelArgs)
		assert.Equal(t, []string{"siderolabs/gvisor"}, info.Extensions)
	})

	t.Run("schematic meta extension with invalid ExtraInfo YAML returns error", func(t *testing.T) {
		t.Parallel()

		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: factorySchematicID, extraInfo: "not: valid: schematic: yaml"},
		})

		_, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to unmarshal schematic manifest")
	})

	t.Run("extensions without meta extension produce ErrInvalidSchematic", func(t *testing.T) {
		t.Parallel()

		// Extensions baked into a custom Talos build bypassing the factory: no schematic meta extension.
		st := buildState(t, []extension{
			{name: "siderolabs/some-custom-thing"},
		})

		_, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, talos.ErrInvalidSchematic), "expected ErrInvalidSchematic, got %v", err)
	})

	t.Run("no extensions and no fallback args synthesizes empty schematic", func(t *testing.T) {
		t.Parallel()

		st := buildState(t, nil)

		info, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.NoError(t, err)

		expected := schematic.Schematic{}

		assert.Equal(t, schematicID(t, expected), info.FullID)
		assert.Equal(t, schematicYAML(t, expected), info.Raw)
		assert.Empty(t, info.KernelArgs)
		assert.Empty(t, info.Extensions)
	})

	t.Run("no meta extension, no extensions, fallback args are used in synthesis", func(t *testing.T) {
		t.Parallel()

		fallbackArgs := []string{"console=tty0", "siderolink.api=grpc://example:9090"}

		st := buildState(t, nil)

		info, err := talos.GetSchematicInfo(t.Context(), st, fallbackArgs)
		require.NoError(t, err)

		expected := schematic.Schematic{
			Customization: schematic.Customization{
				ExtraKernelArgs: fallbackArgs,
			},
		}

		assert.Equal(t, schematicID(t, expected), info.FullID)
		assert.Equal(t, schematicYAML(t, expected), info.Raw)
		assert.Equal(t, fallbackArgs, info.KernelArgs)
		assert.Empty(t, info.Extensions)
	})

	t.Run("modules.dep is filtered out", func(t *testing.T) {
		t.Parallel()

		// No raw manifest, so the extension list is reconstructed from the extension status resources and modules.dep must be filtered out.
		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: factorySchematicID},
			{name: "modules.dep"},
			{name: "siderolabs/gvisor"},
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.NoError(t, err)

		assert.Equal(t, []string{"siderolabs/gvisor"}, info.Extensions)
	})

	t.Run("embedded-config meta extension is filtered out", func(t *testing.T) {
		t.Parallel()

		// No raw manifest, so the extension list is reconstructed from the extension status resources.
		// Machines carrying an embedded machine configuration report an "embedded-config" meta extension which must not leak into the extensions list.
		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: factorySchematicID},
			{name: "embedded-config"},
			{name: "siderolabs/gvisor"},
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.NoError(t, err)

		assert.Equal(t, []string{"siderolabs/gvisor"}, info.Extensions)
	})

	t.Run("embedded-config meta extension alone does not trigger ErrInvalidSchematic", func(t *testing.T) {
		t.Parallel()

		// no real extensions, no schematic meta extension, only the embedded-config meta extension: it must be ignored, so the synthesizing fallback path is taken without error
		st := buildState(t, []extension{
			{name: "embedded-config"},
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.NoError(t, err)

		assert.Empty(t, info.Extensions)
		assert.Equal(t, schematicID(t, schematic.Schematic{}), info.FullID)
	})

	t.Run("extension name without official prefix gets prefixed", func(t *testing.T) {
		t.Parallel()

		// No raw manifest, so the extension list is reconstructed from the extension status resources and the prefix is applied.
		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: factorySchematicID},
			{name: "gvisor"}, // missing "siderolabs/" prefix
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.NoError(t, err)

		assert.Equal(t, []string{"siderolabs/gvisor"}, info.Extensions)
	})

	t.Run("extension with wrong manifest name is remapped", func(t *testing.T) {
		t.Parallel()

		// No raw manifest, so the extension list is reconstructed from the extension status resources.
		// v4l-uvc is a known wrong manifest name remapped to v4l-uvc-drivers.
		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: factorySchematicID},
			{name: "siderolabs/v4l-uvc"},
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.NoError(t, err)

		assert.Equal(t, []string{"siderolabs/v4l-uvc-drivers"}, info.Extensions)
	})

	t.Run("raw schematic manifest is the definitive extension source", func(t *testing.T) {
		t.Parallel()

		// The raw manifest lists exactly the user-requested extensions. The extension status resources additionally
		// report a future meta/virtual extension and a stray extension that are NOT part of the schematic. They must
		// not leak into the result: the extension list is taken verbatim from the raw manifest.
		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: factorySchematicID, extraInfo: factorySchematicYAML},
			{name: "siderolabs/gvisor"},
			{name: "siderolabs/mdadm"},
			{name: "some-future-meta-extension"}, // unknown meta extension we don't (yet) know to skip
			{name: "siderolabs/stray-extension"}, // present on the machine but not part of the schematic manifest
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, []string{"this-fallback-must-not-be-used"})
		require.NoError(t, err)

		assert.Equal(t, factorySchematicID, info.FullID)
		assert.Equal(t, factorySchematicYAML, info.Raw)
		assert.Equal(t, factorySchematic.Customization.ExtraKernelArgs, info.KernelArgs)
		assert.Equal(t, factorySchematic.Customization.SystemExtensions.OfficialExtensions, info.Extensions)
	})

	t.Run("wrong manifest name in the raw manifest is normalized", func(t *testing.T) {
		t.Parallel()

		// A schematic can carry a known wrong manifest name (e.g. v4l-uvc instead of v4l-uvc-drivers). Even on the
		// manifest path we normalize it, so the result stays consistent with the reconstruction path and with the
		// desired extension list computed elsewhere.
		wrongNameSchematic := schematic.Schematic{
			Customization: schematic.Customization{
				SystemExtensions: schematic.SystemExtensions{
					OfficialExtensions: []string{"siderolabs/v4l-uvc"},
				},
			},
		}

		st := buildState(t, []extension{
			{name: constants.SchematicIDExtensionName, version: schematicID(t, wrongNameSchematic), extraInfo: schematicYAML(t, wrongNameSchematic)},
			{name: "siderolabs/v4l-uvc"},
		})

		info, err := talos.GetSchematicInfo(t.Context(), st, nil)
		require.NoError(t, err)

		assert.Equal(t, []string{"siderolabs/v4l-uvc-drivers"}, info.Extensions)
	})
}
