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
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
	), "ghcr.io/siderolabs/installer", nil, nil, nil)

	ms := omni.NewMachineStatus("machine-1")
	ms.TypedSpec().Value.TalosVersion = "1.13.1"
	ms.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{Platform: "metal"}
	ms.TypedSpec().Value.SecurityState = &specs.SecurityState{}
	ms.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		FullId: "current-schematic",
	}

	t.Run("target install image takes precedence over the machine's current schematic", func(t *testing.T) {
		t.Parallel()

		image, err := m.BuildInstallImageForTest("machine-1", ms, "1.13.1", &specs.MachineConfigGenOptionsSpec_InstallImage{
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

	// A target without a factory host is refused rather than pointed at a guessed factory: only the
	// factory that issued the schematic knows it, and the host is recorded next to the schematic.
	t.Run("a target with no factory host is refused", func(t *testing.T) {
		t.Parallel()

		_, err := m.BuildInstallImageForTest("machine-1", ms, "1.13.1", &specs.MachineConfigGenOptionsSpec_InstallImage{
			TalosVersion:         "1.14.0",
			SchematicId:          "target-schematic",
			SchematicInitialized: true,
			Platform:             "metal",
			SecurityState:        &specs.SecurityState{},
		})
		require.ErrorContains(t, err, "no image factory host")
	})

	t.Run("the target install image is not modified", func(t *testing.T) {
		t.Parallel()

		// the target is the spec of a cached MachineConfigGenOptions resource, so it must be left alone
		target := &specs.MachineConfigGenOptionsSpec_InstallImage{
			SchematicId:          "target-schematic",
			SchematicInitialized: true,
			Platform:             "metal",
			SecurityState:        &specs.SecurityState{},
			ImageFactoryHost:     "factory.talos.dev",
		}

		image, err := m.BuildInstallImageForTest("machine-1", ms, "1.13.1", target)
		require.NoError(t, err)

		assert.Equal(t, "factory.talos.dev/metal-installer/target-schematic:v1.13.1", image)
		assert.Empty(t, target.TalosVersion)
	})

	t.Run("nil target is refused", func(t *testing.T) {
		t.Parallel()

		_, err := m.BuildInstallImageForTest("machine-1", ms, "1.14.0", nil)
		require.ErrorContains(t, err, "install image target is required")
	})

	t.Run("missing platform metadata fails fast", func(t *testing.T) {
		t.Parallel()

		bare := omni.NewMachineStatus("machine-bare")
		bare.TypedSpec().Value.TalosVersion = "1.13.1"

		_, err := m.BuildInstallImageForTest("machine-bare", bare, "", nil)
		require.Error(t, err)
	})
}

func TestRelayProgress(t *testing.T) {
	t.Parallel()

	t.Run("messages are split into lines and prefixed", func(t *testing.T) {
		t.Parallel()

		var got []string

		err := lifecycle.RelayProgressForTest(
			&machineapi.LifecycleServiceInstallProgress{
				Response: &machineapi.LifecycleServiceInstallProgress_Message{Message: "wiping disk  \n\nwriting image\n"},
			},
			"installation",
			func(msg string) { got = append(got, msg) },
		)
		require.NoError(t, err)

		assert.Equal(t, []string{"[talos] wiping disk", "[talos] writing image"}, got)
	})

	t.Run("a zero exit code reports success", func(t *testing.T) {
		t.Parallel()

		err := lifecycle.RelayProgressForTest(
			&machineapi.LifecycleServiceInstallProgress{
				Response: &machineapi.LifecycleServiceInstallProgress_ExitCode{ExitCode: constants.ExitSuccess},
			},
			"installation",
			nil,
		)
		assert.NoError(t, err)
	})

	t.Run("a non-zero exit code becomes a classified installer error", func(t *testing.T) {
		t.Parallel()

		err := lifecycle.RelayProgressForTest(
			&machineapi.LifecycleServiceInstallProgress{
				Response: &machineapi.LifecycleServiceInstallProgress_ExitCode{ExitCode: constants.ExitInvalidInput},
			},
			"installation",
			nil,
		)
		require.Error(t, err)

		var exitErr *lifecycle.InstallerExitError

		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, "installation", exitErr.Operation)
		assert.EqualValues(t, constants.ExitInvalidInput, exitErr.Code)
		assert.True(t, lifecycle.IsPermanentInstallerFailure(err))
	})
}
