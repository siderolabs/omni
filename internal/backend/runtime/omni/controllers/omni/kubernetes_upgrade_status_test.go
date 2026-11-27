// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

func TestKubernetesUpgradePatchesFollowVersionContract(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	const legacyPatch = "machine:\n  kubelet:\n    image: ghcr.io/siderolabs/kubelet:v1.33.1\n"

	testutils.WithRuntime(
		ctx,
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewKubernetesUpgradeStatusController()))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			// locked, so the upgrade itself stays out of the way
			cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.Modify(func(res *omni.Cluster) error {
				res.Metadata().Annotations().Set(omni.ClusterLocked, "")

				return nil
			}))

			clusterConfigVersion := omni.NewClusterConfigVersion(cluster.Metadata().ID())
			clusterConfigVersion.TypedSpec().Value.Version = "v1.13.5"
			require.NoError(t, st.Create(ctx, clusterConfigVersion))

			upgradeStatus := omni.NewKubernetesUpgradeStatus(cluster.Metadata().ID())
			upgradeStatus.TypedSpec().Value.LastUpgradeVersion = "1.33.1"
			require.NoError(t, st.Create(ctx, upgradeStatus, state.WithCreateOwner(omnictrl.KubernetesUpgradeStatusControllerName)))

			machines := map[string]bool{"control-plane": true, "worker": false}
			patchIDs := make([]resource.ID, 0, len(machines))

			for machineID, controlPlane := range machines {
				clusterMachineStatus := omni.NewClusterMachineStatus(machineID)
				clusterMachineStatus.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

				if controlPlane {
					clusterMachineStatus.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
				}

				require.NoError(t, st.Create(ctx, clusterMachineStatus))

				patch := omni.NewConfigPatch("900-cm-" + machineID + "-kubernetes-upgrade")
				patch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
				patch.Metadata().Labels().Set(omni.LabelClusterMachine, machineID)
				require.NoError(t, patch.TypedSpec().Value.SetUncompressedData([]byte(legacyPatch)))
				require.NoError(t, st.Create(ctx, patch, state.WithCreateOwner(omnictrl.KubernetesUpgradeStatusControllerName)))

				patchIDs = append(patchIDs, patch.Metadata().ID())
			}

			patchData := func(a *assert.Assertions, patch *omni.ConfigPatch) string {
				buffer, err := patch.TypedSpec().Value.GetUncompressedData()
				a.NoError(err)

				defer buffer.Free()

				return string(buffer.Data())
			}

			rtestutils.AssertResources(ctx, t, st, patchIDs, func(patch *omni.ConfigPatch, a *assert.Assertions) {
				patchContract, _ := patch.Metadata().Annotations().Get(omni.VersionContract)
				a.Equal("v1.13", patchContract)
				a.Equal(legacyPatch, patchData(a, patch))
			})

			_, err := safe.StateUpdateWithConflicts(ctx, st, clusterConfigVersion.Metadata(), func(res *omni.ClusterConfigVersion) error {
				res.TypedSpec().Value.Version = "v1.14.0"

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertResources(ctx, t, st, patchIDs, func(patch *omni.ConfigPatch, a *assert.Assertions) {
				patchContract, _ := patch.Metadata().Annotations().Get(omni.VersionContract)
				a.Equal("v1.14", patchContract)

				data := patchData(a, patch)
				a.NotContains(data, "machine:")
				a.Contains(data, "kind: KubeletConfig")
				a.Contains(data, "kubelet:v1.33.1")

				machineID, _ := patch.Metadata().Labels().Get(omni.LabelClusterMachine)
				if machines[machineID] {
					a.Contains(data, "kind: KubeAPIServerConfig")
					a.Contains(data, "kube-apiserver:v1.33.1")
				} else {
					a.NotContains(data, "kube-apiserver")
				}
			})

			_, err = safe.StateUpdateWithConflicts(ctx, st, clusterConfigVersion.Metadata(), func(res *omni.ClusterConfigVersion) error {
				res.TypedSpec().Value.Version = "v1.13.0"

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertResources(ctx, t, st, patchIDs, func(patch *omni.ConfigPatch, a *assert.Assertions) {
				patchContract, _ := patch.Metadata().Annotations().Get(omni.VersionContract)
				a.Equal("v1.13", patchContract)

				data := patchData(a, patch)
				a.Contains(data, "machine:")
				a.Contains(data, "kubelet:v1.33.1")
				a.NotContains(data, "kind:")

				machineID, _ := patch.Metadata().Labels().Get(omni.LabelClusterMachine)
				if machines[machineID] {
					a.Contains(data, "apiServer:")
				}
			})
		},
	)
}
