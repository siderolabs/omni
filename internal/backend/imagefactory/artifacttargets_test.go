// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	imagefactoryinternal "github.com/siderolabs/omni/internal/backend/imagefactory"
)

// schematicID is a valid schematic ID: the hex-encoded SHA-256 of some schematic.
const schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

// TestClusterArtifactTargets covers resolving a cluster's installed (schematic, arch) pairs and
// upgrade-target versions from its COSI resources.
func TestClusterArtifactTargets(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))
	ctx := t.Context()

	const clusterID = "test-cluster"

	clusterStatus := omni.NewClusterStatus(clusterID)
	clusterStatus.TypedSpec().Value.TalosVersion = "1.9.0"
	require.NoError(t, st.Create(ctx, clusterStatus))

	createMachine := func(id, schematic, arch string, controlPlane bool) {
		t.Helper()

		machineStatus := omni.NewMachineStatus(id)
		machineStatus.Metadata().Labels().Set(omni.LabelCluster, clusterID)
		machineStatus.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{Arch: arch}
		require.NoError(t, st.Create(ctx, machineStatus))

		cfg := omni.NewClusterMachineConfigStatus(id)
		cfg.Metadata().Labels().Set(omni.LabelCluster, clusterID)

		if controlPlane {
			cfg.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
		}

		cfg.TypedSpec().Value.SchematicId = schematic
		require.NoError(t, st.Create(ctx, cfg))
	}

	// Two machines share a schematic and architecture (one of them control plane), a third is on
	// the same schematic but a different architecture - three machines, two distinct targets.
	createMachine("cp-1", schematicID, "amd64", true)
	createMachine("worker-1", schematicID, "amd64", false)
	createMachine("worker-2", schematicID, "arm64", false)

	for _, tv := range []struct {
		version    string
		deprecated bool
	}{
		{version: "1.9.0"},
		{version: "1.9.2"},
		{version: "1.10.0"},
		{version: "1.10.1"},
		{version: "1.11.0", deprecated: true},
	} {
		version := omni.NewTalosVersion(tv.version)
		version.TypedSpec().Value.Version = tv.version
		version.TypedSpec().Value.Deprecated = tv.deprecated
		require.NoError(t, st.Create(ctx, version))
	}

	resp, err := imagefactoryinternal.ClusterArtifactTargets(ctx, st, clusterID)
	require.NoError(t, err)

	require.Equal(t, "1.9.0", resp.GetCurrentTalosVersion())
	require.Equal(t, []string{"1.9.2", "1.10.1"}, resp.GetUpgradeTargetVersions())

	require.Len(t, resp.GetTargets(), 2)

	amd64 := resp.GetTargets()[0]
	require.Equal(t, schematicID, amd64.GetSchematicId())
	require.Equal(t, imagefactorypb.Arch_AMD64, amd64.GetArch())
	require.Equal(t, int32(2), amd64.GetMachineCount())
	require.True(t, amd64.GetIncludesControlPlane())

	arm64 := resp.GetTargets()[1]
	require.Equal(t, schematicID, arm64.GetSchematicId())
	require.Equal(t, imagefactorypb.Arch_ARM64, arm64.GetArch())
	require.Equal(t, int32(1), arm64.GetMachineCount())
	require.False(t, arm64.GetIncludesControlPlane())
}

// TestClusterArtifactTargetsUnknownCluster covers that an unknown cluster surfaces as a
// not-found error the caller (the gRPC handler) can map to the right status code.
func TestClusterArtifactTargetsUnknownCluster(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	_, err := imagefactoryinternal.ClusterArtifactTargets(t.Context(), st, "does-not-exist")

	require.True(t, state.IsNotFoundError(err))
}

// TestClusterArtifactTargetsUpgradeVersions covers picking the upgrade targets to scan against,
// through ClusterArtifactTargets: the latest patch of the current minor, and the latest patch of
// the next available minor, each only when strictly newer than the current version, tolerating
// gaps in the available minors and a leading "v".
func TestClusterArtifactTargetsUpgradeVersions(t *testing.T) {
	available := []string{"1.8.0", "1.8.3", "1.8.4", "1.9.0", "1.9.2", "1.10.0", "1.10.1"}

	for _, test := range []struct {
		name      string
		current   string
		available []string
		expected  []string
	}{
		{
			name:      "returns latest patch of current minor and latest patch of next minor",
			current:   "1.9.0",
			available: available,
			expected:  []string{"1.9.2", "1.10.1"},
		},
		{
			name:      "omits the patch target when already on the latest patch of the minor",
			current:   "1.9.2",
			available: available,
			expected:  []string{"1.10.1"},
		},
		{
			name:      "omits the minor target when on the newest minor",
			current:   "1.10.0",
			available: available,
			expected:  []string{"1.10.1"},
		},
		{
			name:      "handles gaps in available minors",
			current:   "1.8.4",
			available: []string{"1.8.4", "1.10.0", "1.10.2"},
			expected:  []string{"1.10.2"},
		},
		{
			name:      "picks the smallest next minor even when a later minor is listed first",
			current:   "1.8.4",
			available: []string{"1.10.2", "1.9.1"},
			expected:  []string{"1.9.1"},
		},
		{
			name:      "tolerates leading v prefixes",
			current:   "v1.9.0",
			available: []string{"v1.9.1", "v1.10.0"},
			expected:  []string{"v1.9.1", "v1.10.0"},
		},
		{
			name:      "returns nothing for an unparseable current version",
			current:   "garbage",
			available: available,
			expected:  nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := state.WrapCore(namespaced.NewState(inmem.Build))
			ctx := t.Context()

			const clusterID = "test-cluster"

			clusterStatus := omni.NewClusterStatus(clusterID)
			clusterStatus.TypedSpec().Value.TalosVersion = test.current
			require.NoError(t, st.Create(ctx, clusterStatus))

			for _, version := range test.available {
				talosVersion := omni.NewTalosVersion(version)
				talosVersion.TypedSpec().Value.Version = version
				require.NoError(t, st.Create(ctx, talosVersion))
			}

			resp, err := imagefactoryinternal.ClusterArtifactTargets(ctx, st, clusterID)
			require.NoError(t, err)

			require.Equal(t, test.expected, resp.GetUpgradeTargetVersions())
		})
	}
}
