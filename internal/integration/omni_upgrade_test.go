// Copyright (c) 2025 Sidero Labs, Inc.
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
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/client"
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
func SaveClusterSnapshot(testCtx context.Context, client *client.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute)
		defer cancel()

		st := client.Omni().State()

		snapshot := clusterSnapshot{
			BootTimes: map[string]time.Time{},
			ShaSums:   map[string]string{},
		}

		cmcss := rtestutils.ResourceIDs[*omni.ClusterMachineConfigStatus](ctx, t, st,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		rtestutils.AssertResources(ctx, t, st, cmcss, func(res *omni.ClusterMachineConfigStatus, _ *assert.Assertions) {
			snapshot.saveShaSum(res, res.TypedSpec().Value.ClusterMachineConfigSha256)
		})

		require := require.New(t)
		assert := assert.New(t)

		data, err := client.Management().Talosconfig(ctx)
		require.NoError(err)
		assert.NotEmpty(data)

		config, err := clientconfig.FromBytes(data)
		require.NoError(err)

		c, err := talosclient.New(ctx, talosclient.WithConfig(config), talosclient.WithCluster(clusterName))
		require.NoError(err)

		t.Cleanup(func() {
			require.NoError(c.Close())
		})

		machineIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, client.Omni().State(),
			state.WithLabelQuery(
				resource.LabelEqual(omni.LabelCluster, clusterName),
				resource.LabelExists(omni.LabelControlPlaneRole),
			),
		)

		for _, machineID := range machineIDs {
			var ms *runtime.MachineStatus

			ms, err = safe.ReaderGetByID[*runtime.MachineStatus](talosclient.WithNode(ctx, machineID), c.COSI, runtime.MachineStatusID)

			require.NoError(err)

			snapshot.BootTimes[machineID] = ms.Metadata().Created()
		}

		snapshotData, err := json.Marshal(snapshot)

		require.NoError(err)

		_, err = safe.StateUpdateWithConflicts(ctx,
			client.Omni().State(),
			omni.NewCluster(clusterName).Metadata(),
			func(res *omni.Cluster) error {
				res.Metadata().Annotations().Set(annotationSnapshot, string(snapshotData))

				return nil
			},
		)

		require.NoError(err)
	}
}

// AssertClusterSnapshot reads the snapshot from the cluster resource and asserts that versions did not change
// and the last events still can be found in the node events.
func AssertClusterSnapshot(testCtx context.Context, client *client.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute)
		defer cancel()

		st := client.Omni().State()

		require := require.New(t)

		var snapshot clusterSnapshot

		cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, st, clusterName)
		require.NoError(err)

		snapshotData, ok := cluster.Metadata().Annotations().Get(annotationSnapshot)

		require.True(ok, "cluster does not have snapshot annotation")

		require.NoError(json.Unmarshal([]byte(snapshotData), &snapshot))

		ids := rtestutils.ResourceIDs[*omni.ClusterMachineConfigStatus](ctx, t, st,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		rtestutils.AssertResources(ctx, t, st, ids, func(res *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
			shaSum, ok := snapshot.getShaSum(res)

			assert.True(ok)
			require.Equal(shaSum, res.TypedSpec().Value.ClusterMachineConfigSha256, "ClusterMachineConfigStatus sha sums do not match")
		})

		data, err := client.Management().Talosconfig(ctx)
		require.NoError(err)
		assert.NotEmpty(t, data)

		config, err := clientconfig.FromBytes(data)
		require.NoError(err)

		c, err := talosclient.New(ctx, talosclient.WithConfig(config), talosclient.WithCluster(clusterName))
		require.NoError(err)

		t.Cleanup(func() {
			require.NoError(c.Close())
		})

		for machineID, bootTime := range snapshot.BootTimes {
			var ms *runtime.MachineStatus

			ms, err = safe.ReaderGetByID[*runtime.MachineStatus](talosclient.WithNode(ctx, machineID), c.COSI, runtime.MachineStatusID)

			require.NoError(err)

			require.True(ms.TypedSpec().Status.Ready)
			require.Equal(runtime.MachineStageRunning, ms.TypedSpec().Stage)

			require.Equal(bootTime, ms.Metadata().Created(), "the machine was rebooted")
		}
	}
}
