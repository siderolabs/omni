// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

func TestBuildInstallImage(t *testing.T) {
	t.Parallel()

	c, err := imagefactory.NewClient("https://factory.talos.dev", "", "")
	require.NoError(t, err)

	m := lifecycle.NewManager(zapNop(t), imagefactory.NewClients(
		state.WrapCore(namespaced.NewState(inmem.Build)),
		c,
	), "ghcr.io/siderolabs/installer", nil, nil)

	ms := omni.NewMachineStatus("machine-1")
	ms.TypedSpec().Value.TalosVersion = "1.13.1"
	ms.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{Platform: "metal"}
	ms.TypedSpec().Value.SecurityState = &specs.SecurityState{}
	ms.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		FullId: "current-schematic",
	}

	t.Run("target install image takes precedence over the machine's current schematic", func(t *testing.T) {
		t.Parallel()

		image, err := m.BuildInstallImageForTest(t.Context(), "machine-1", ms, "1.13.1", &specs.MachineConfigGenOptionsSpec_InstallImage{
			TalosVersion:         "1.14.0",
			SchematicId:          "target-schematic",
			SchematicInitialized: true,
			Platform:             "metal",
			SecurityState:        &specs.SecurityState{},
			ImageFactoryHost:     "factory.talos.dev",
		})
		require.NoError(t, err)

		assert.Equal(t, "factory.talos.dev/metal-installer/target-schematic:v1.14.0", image)
	})

	t.Run("nil target falls back to the machine's current schematic", func(t *testing.T) {
		t.Parallel()

		image, err := m.BuildInstallImageForTest(t.Context(), "machine-1", ms, "1.14.0", nil)
		require.NoError(t, err)

		assert.Equal(t, "factory.talos.dev/metal-installer/current-schematic:v1.14.0", image)
	})

	t.Run("nil target and empty version reuses the machine's running version", func(t *testing.T) {
		t.Parallel()

		image, err := m.BuildInstallImageForTest(t.Context(), "machine-1", ms, "", nil)
		require.NoError(t, err)

		assert.Equal(t, "factory.talos.dev/metal-installer/current-schematic:v1.13.1", image)
	})

	t.Run("missing platform metadata fails fast", func(t *testing.T) {
		t.Parallel()

		bare := omni.NewMachineStatus("machine-bare")
		bare.TypedSpec().Value.TalosVersion = "1.13.1"

		_, err := m.BuildInstallImageForTest(t.Context(), "machine-bare", bare, "", nil)
		require.Error(t, err)
	})
}
