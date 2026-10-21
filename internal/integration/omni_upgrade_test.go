// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	configChanged       bool
	installImageRemoval bool
}

type assertClusterSnapshotOption func(*assertClusterSnapshotOptions)

// withConfigChanged expects every machine config to differ from the snapshot, and waits for that, e.g., after a change that regenerates the configs without a reboot.
func withConfigChanged() assertClusterSnapshotOption {
	return func(o *assertClusterSnapshotOptions) {
		o.configChanged = true
	}
}

// withInstallImageRemoval allows the one-time config change of an upgrade from an Omni version that still put the install image into
// the config of the machines installed over the LifecycleService: the install image is removed and nothing else changes.
// The configs themselves are checked by AssertAppliedConfigsOnlyLoseInstallImage.
func withInstallImageRemoval() assertClusterSnapshotOption {
	return func(o *assertClusterSnapshotOptions) {
		o.installImageRemoval = true
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

			if snapshotOptions.installImageRemoval {
				return
			}

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
func AssertClusterSnapshotHolds(testCtx context.Context, options *TestOptions, clusterName string, duration time.Duration, opts ...assertClusterSnapshotOption) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, duration+time.Minute)
		defer cancel()

		var snapshotOptions assertClusterSnapshotOptions

		for _, o := range opts {
			o(&snapshotOptions)
		}

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
				if res.TypedSpec().Value.Upgrade == nil && snapshotOptions.installImageRemoval && removesOnlyInstallImage(res) {
					continue
				}

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

// SaveAppliedConfigs records the config every machine of the cluster runs, in the redacted form recorded on its config status.
func SaveAppliedConfigs(testCtx context.Context, options *TestOptions, clusterName string, appliedConfigs map[string]string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute)
		defer cancel()

		configStatuses, err := safe.ReaderListAll[*omni.ClusterMachineConfigStatus](ctx, options.omniClient.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NoError(t, err)

		for res := range configStatuses.All() {
			appliedConfigs[res.Metadata().ID()], err = appliedConfig(res)
			require.NoError(t, err)
		}
	}
}

// AssertAppliedConfigsOnlyLoseInstallImage waits until every machine of the cluster runs its recorded config without the install image line,
// and has no pending update. A recorded config without an install image has to stay as it is. A config with an install image keeps it until
// the change is applied, so this cannot pass before that.
func AssertAppliedConfigsOnlyLoseInstallImage(testCtx context.Context, options *TestOptions, clusterName string, appliedConfigs map[string]string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 5*time.Minute)
		defer cancel()

		st := options.omniClient.Omni().State()
		clusterQuery := state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName))

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			configStatuses, err := safe.ReaderListAll[*omni.ClusterMachineConfigStatus](ctx, st, clusterQuery)
			require.NoError(collect, err)

			for res := range configStatuses.All() {
				recorded, ok := appliedConfigs[res.Metadata().ID()]
				if !assert.True(collect, ok, "machine %q has no recorded config", res.Metadata().ID()) {
					continue
				}

				applied, configErr := appliedConfig(res)
				require.NoError(collect, configErr)

				assert.Equal(collect, withoutInstallImage(recorded), applied, "machine %q", res.Metadata().ID())
			}

			pendingUpdates, err := safe.ReaderListAll[*omni.MachinePendingUpdates](ctx, st, clusterQuery)
			require.NoError(collect, err)

			for res := range pendingUpdates.All() {
				assert.Fail(collect, "pending update", "machine %q has a pending update", res.Metadata().ID())
			}
		}, 5*time.Minute, 2*time.Second)
	}
}

// removesOnlyInstallImage reports whether the pending config diff removes the install image line and changes nothing else.
func removesOnlyInstallImage(res *omni.MachinePendingUpdates) bool {
	buffer, err := res.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return false
	}

	defer buffer.Free()

	removed := 0

	for line := range strings.Lines(string(buffer.Data())) {
		switch {
		case strings.HasPrefix(line, "+"):
			return false
		case strings.HasPrefix(line, "-"):
			if !isInstallImageLine(line[1:]) {
				return false
			}

			removed++
		}
	}

	return removed == 1
}

// withoutInstallImage returns the config without its install image line.
func withoutInstallImage(config string) string {
	var result strings.Builder

	for line := range strings.Lines(config) {
		if !isInstallImageLine(line) {
			result.WriteString(line)
		}
	}

	return result.String()
}

// isInstallImageLine reports whether the config line sets the install image (the image key of the installer).
func isInstallImageLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "image: ") && strings.Contains(line, "installer")
}

// appliedConfig returns the config the machine runs, in the redacted form recorded on its config status.
func appliedConfig(res *omni.ClusterMachineConfigStatus) (string, error) {
	buffer, err := res.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return "", err
	}

	defer buffer.Free()

	if len(buffer.Data()) == 0 {
		return "", errors.New("no config is applied")
	}

	return string(buffer.Data()), nil
}
