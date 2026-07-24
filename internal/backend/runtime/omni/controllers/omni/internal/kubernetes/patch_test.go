// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/image"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/kubernetes"
)

// TestMergePatch verifies that patches for several components accumulate into a single machine patch.
func TestMergePatch(t *testing.T) {
	t.Parallel()

	for _, vc := range []*config.VersionContract{config.TalosVersion1_13, config.TalosVersion1_14} {
		t.Run(vc.String(), func(t *testing.T) {
			t.Parallel()

			var accumulated []byte

			for _, component := range []kubernetes.Component{
				kubernetes.APIServer,
				kubernetes.ControllerManager,
				kubernetes.Scheduler,
				kubernetes.Kubelet,
			} {
				patch, err := component.Patch(vc, "1.2.3")
				require.NoError(t, err)

				accumulated, err = kubernetes.MergePatch(accumulated, patch.Patch)
				require.NoError(t, err)
			}

			// reapplying a component's patch should replace its contribution, not duplicate it.
			newAPIServerPatch, err := kubernetes.APIServer.Patch(vc, "1.2.4")
			require.NoError(t, err)

			accumulated, err = kubernetes.MergePatch(accumulated, newAPIServerPatch.Patch)
			require.NoError(t, err)

			in, err := generate.NewInput("merge-patch-test", "https://127.0.0.1/", "1.34.0", generate.WithVersionContract(vc))
			require.NoError(t, err)

			cfg, err := in.Config(machine.TypeControlPlane)
			require.NoError(t, err)

			patch, err := configpatcher.LoadPatch(accumulated)
			require.NoError(t, err)

			patched, err := configpatcher.Apply(configpatcher.WithConfig(cfg), []configpatcher.Patch{patch})
			require.NoError(t, err)

			patchedCfg, err := patched.Config()
			require.NoError(t, err)

			assertTag := func(t *testing.T, expected, ref string) {
				t.Helper()

				tag, err := image.GetTag(ref)

				require.NoError(t, err)
				assert.Equal(t, expected, tag)
			}

			assertTag(t, "v1.2.4", patchedCfg.K8sAPIServerConfig().Image())
			assertTag(t, "v1.2.4", patchedCfg.K8sProxyConfig().Image())
			assertTag(t, "v1.2.3", patchedCfg.K8sControllerManagerConfig().Image())
			assertTag(t, "v1.2.3", patchedCfg.K8sSchedulerConfig().Image())
			assertTag(t, "v1.2.3", patchedCfg.K8sKubeletConfig().Image())
		})
	}
}

// TestMergePatchIdempotent verifies that merging an already-applied patch again reproduces byte-identical output.
func TestMergePatchIdempotent(t *testing.T) {
	t.Parallel()

	for _, vc := range []*config.VersionContract{config.TalosVersion1_13, config.TalosVersion1_14} {
		t.Run(vc.String(), func(t *testing.T) {
			t.Parallel()

			patch, err := kubernetes.APIServer.Patch(vc, "1.2.3")
			require.NoError(t, err)

			// first reconcile: no patch stored yet on the ConfigPatch resource.
			firstMerge, err := kubernetes.MergePatch(nil, patch.Patch)
			require.NoError(t, err)

			// second reconcile: the desired version hasn't changed, so the same component patch is merged again on
			// top of the result of the first reconcile.
			secondMerge, err := kubernetes.MergePatch(firstMerge, patch.Patch)
			require.NoError(t, err)

			assert.Equal(t, firstMerge, secondMerge)
		})
	}
}

// TestMultiPatcherNoPatchers verifies that MultiPatcher rejects an empty patcher list instead of panicking.
func TestMultiPatcherNoPatchers(t *testing.T) {
	t.Parallel()

	_, err := kubernetes.MultiPatcher()
	require.Error(t, err)
}

// TestMergePatchEmptyNewPatch verifies that merging an empty new patch is a no-op that returns the existing patch
// unchanged, rather than failing to parse the empty input.
func TestMergePatchEmptyNewPatch(t *testing.T) {
	t.Parallel()

	existing, err := kubernetes.APIServer.Patch(config.TalosVersion1_14, "1.2.3")
	require.NoError(t, err)

	merged, err := kubernetes.MergePatch(existing.Patch, nil)
	require.NoError(t, err)
	assert.Equal(t, existing.Patch, merged)

	merged, err = kubernetes.MergePatch(existing.Patch, []byte("   \n"))
	require.NoError(t, err)
	assert.Equal(t, existing.Patch, merged)
}
