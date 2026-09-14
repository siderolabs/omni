// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const annotationSnapshot = "snapshot"

type clusterSnapshot struct {
	BootTimes map[string]time.Time
	ShaSums   map[string]string
}

func (vs clusterSnapshot) saveShaSum(res resource.Resource, shaSum string) {
	vs.ShaSums[res.Metadata().Type()+"/"+res.Metadata().ID()] = shaSum
}

func (vs clusterSnapshot) getShaSum(res resource.Resource) (string, bool) {
	val, ok := vs.ShaSums[res.Metadata().Type()+"/"+res.Metadata().ID()]

	return val, ok
}

// SaveClusterSnapshot saves resources versions as the annotations for the given cluster.
func SaveClusterSnapshot(testCtx context.Context, options *TestOptions, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute)
		defer cancel()

		omniClient := options.omniClient
		st := omniClient.Omni().State()

		snapshot := clusterSnapshot{
			BootTimes: map[string]time.Time{},
			ShaSums:   map[string]string{},
		}

		cmcss := rtestutils.ResourceIDs[*omni.ClusterMachineConfigStatus](
			ctx, t, st,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		rtestutils.AssertResources(ctx, t, st, cmcss, func(res *omni.ClusterMachineConfigStatus, _ *assert.Assertions) {
			snapshot.saveShaSum(res, res.TypedSpec().Value.ClusterMachineConfigSha256)
		})

		require := require.New(t)
		c := getTalosClientForCluster(ctx, t, options, clusterName)

		t.Cleanup(func() {
			require.NoError(c.Close())
		})

		machineIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](
			ctx, t, omniClient.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		for _, machineID := range machineIDs {
			snapshot.BootTimes[machineID] = readTalosMachineStatus(ctx, t, c, machineID).Metadata().Created()
		}

		snapshotData, err := json.Marshal(snapshot)

		require.NoError(err)

		_, err = safe.StateUpdateWithConflicts(
			ctx,
			omniClient.Omni().State(),
			omni.NewCluster(clusterName).Metadata(),
			func(res *omni.Cluster) error {
				res.Metadata().Annotations().Set(annotationSnapshot, string(snapshotData))

				return nil
			},
		)

		require.NoError(err)
	}
}

// AssertNoPendingMachineUpdates asserts that there are no pending machine updates for the cluster, waiting for the ones in flight to settle.
func AssertNoPendingMachineUpdates(testCtx context.Context, options *TestOptions, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute)
		defer cancel()

		st := options.omniClient.Omni().State()

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			machinePendingUpdates, err := safe.ReaderListAll[*omni.MachinePendingUpdates](ctx, st,
				state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
			require.NoError(collect, err)

			for res := range machinePendingUpdates.All() {
				assert.Empty(collect, res.TypedSpec().Value.Upgrade, "machine %q has a pending upgrade", res.Metadata().ID())
				assert.False(collect, res.TypedSpec().Value.HasConfigDiff(), "machine %q has a pending config change", res.Metadata().ID())
			}
		}, time.Minute, 2*time.Second)
	}
}

type assertClusterSnapshotOptions struct {
	configChanged bool
}

type assertClusterSnapshotOption func(*assertClusterSnapshotOptions)

// withConfigChanged expects every machine config to differ from the snapshot, and waits for that, e.g., after a change that regenerates the configs without a reboot.
func withConfigChanged() assertClusterSnapshotOption {
	return func(o *assertClusterSnapshotOptions) {
		o.configChanged = true
	}
}

// AssertClusterSnapshot reads the snapshot from the cluster resource and asserts that versions did not change
// and the last events still can be found in the node events.
func AssertClusterSnapshot(testCtx context.Context, options *TestOptions, clusterName string, opts ...assertClusterSnapshotOption) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 5*time.Minute)
		defer cancel()

		var snapshotOptions assertClusterSnapshotOptions

		for _, o := range opts {
			o(&snapshotOptions)
		}

		omniClient := options.omniClient
		omniState := omniClient.Omni().State()

		require := require.New(t)

		var snapshot clusterSnapshot

		cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, omniState, clusterName)
		require.NoError(err)

		snapshotData, ok := cluster.Metadata().Annotations().Get(annotationSnapshot)

		require.True(ok, "cluster does not have snapshot annotation")

		require.NoError(json.Unmarshal([]byte(snapshotData), &snapshot))

		ids := rtestutils.ResourceIDs[*omni.ClusterMachineConfigStatus](
			ctx, t, omniState,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		rtestutils.AssertResources(ctx, t, omniState, ids, func(res *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
			shaSum, ok := snapshot.getShaSum(res)

			assert.True(ok)

			if snapshotOptions.configChanged {
				assert.NotEqual(shaSum, res.TypedSpec().Value.ClusterMachineConfigSha256, "ClusterMachineConfigStatus sha sum did not change")

				return
			}

			require.Equal(shaSum, res.TypedSpec().Value.ClusterMachineConfigSha256, "ClusterMachineConfigStatus sha sums do not match")
		})

		// the wait for the config shas above may have used up most of the time, the boot time reads get their own
		ctx, cancel = context.WithTimeout(testCtx, 2*time.Minute)
		defer cancel()

		c := getTalosClientForCluster(ctx, t, options, clusterName)

		t.Cleanup(func() {
			require.NoError(c.Close())
		})

		for machineID, bootTime := range snapshot.BootTimes {
			ms := readTalosMachineStatus(ctx, t, c, machineID)

			require.True(ms.TypedSpec().Status.Ready)
			require.Equal(runtime.MachineStageRunning, ms.TypedSpec().Stage)

			require.Equal(bootTime, ms.Metadata().Created(), "the machine was rebooted")
		}
	}
}

// AssertClusterSnapshotHolds watches the cluster for the given duration and asserts that the snapshot stays valid: no machine config changes,
// no update gets pending, and finally no machine rebooted.
func AssertClusterSnapshotHolds(testCtx context.Context, options *TestOptions, clusterName string, duration time.Duration) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, duration+time.Minute)
		defer cancel()

		st := options.omniClient.Omni().State()

		cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, st, clusterName)
		require.NoError(t, err)

		snapshotData, ok := cluster.Metadata().Annotations().Get(annotationSnapshot)
		require.True(t, ok, "cluster does not have snapshot annotation")

		var snapshot clusterSnapshot

		require.NoError(t, json.Unmarshal([]byte(snapshotData), &snapshot))

		clusterQuery := state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName))

		require.Never(t, func() bool {
			configStatuses, listErr := safe.ReaderListAll[*omni.ClusterMachineConfigStatus](ctx, st, clusterQuery)
			if listErr != nil {
				return false
			}

			for res := range configStatuses.All() {
				if shaSum, found := snapshot.getShaSum(res); !found || shaSum != res.TypedSpec().Value.ClusterMachineConfigSha256 {
					t.Logf("machine config of %q changed", res.Metadata().ID())

					return true
				}
			}

			pendingUpdates, listErr := safe.ReaderListAll[*omni.MachinePendingUpdates](ctx, st, clusterQuery)
			if listErr != nil {
				return false
			}

			for res := range pendingUpdates.All() {
				if res.TypedSpec().Value.Upgrade != nil || res.TypedSpec().Value.HasConfigDiff() {
					t.Logf("machine %q has a pending update", res.Metadata().ID())

					return true
				}
			}

			return false
		}, duration, 5*time.Second, "the cluster changed after the snapshot")

		AssertClusterSnapshot(testCtx, options, clusterName)(t)
	}
}

// readTalosMachineStatus reads the Talos machine status of the machine, retrying the transient API errors.
func readTalosMachineStatus(ctx context.Context, t *testing.T, c *talosclient.Client, machineID string) *runtime.MachineStatus {
	var ms *runtime.MachineStatus

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		var err error

		ms, err = safe.ReaderGetByID[*runtime.MachineStatus](talosclient.WithNode(ctx, machineID), c.COSI, runtime.MachineStatusID)
		require.NoError(collect, err)
	}, time.Minute, 2*time.Second, "machine %q status", machineID)

	return ms
}
