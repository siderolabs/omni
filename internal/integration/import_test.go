// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"net/netip"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapio"
	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/clusterimport"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func testClusterImport(t *testing.T, options *TestOptions) {
	t.Parallel()

	var clusterID string
	var clusterNodes []string

	if options.ImportedClusterStatePath != "" {
		f, err := os.ReadFile(options.ImportedClusterStatePath)

		require.NoError(t, err, "failed to open talos cluster state")

		var st vmState

		err = yaml.Unmarshal(f, &st)
		require.NoError(t, err, "failed to parse talos cluster state file")

		clusterID = st.ClusterInfo.ClusterName

		for _, info := range st.ClusterInfo.Nodes {
			require.Len(t, info.IPs, 1)
			clusterNodes = append(clusterNodes, info.IPs[0].String())
		}
	}

	require.NotEmpty(t, clusterNodes, "no cluster nodes provided for import test")
	require.NotEmpty(t, clusterID, "no cluster ID provided for import test")

	t.Run(
		"ClusterShouldBeImported",
		func(t *testing.T) { testImport(t, options, clusterID, clusterNodes) },
	)

	t.Run(
		"UnlockedClusterShouldReceiveConfigChanges",
		func(t *testing.T) { testUnlockCluster(t, options, clusterID) },
	)

	t.Run(
		"ClusterImportShouldBeAborted",
		func(t *testing.T) { testImportAbort(t, options, clusterID, clusterNodes) },
	)
}

func testImport(t *testing.T, options *TestOptions, clusterID string, clusterNodes []string) {
	t.Log(`Import an existing talos cluster, assert that the cluster is ready and accessible using provided talosconfig.`)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	logger := zaptest.NewLogger(t)

	omniState := options.omniClient.Omni().State()
	imageFactoryClient, err := clusterimport.BuildImageFactoryClient(ctx, omniState)
	require.NoError(t, err)

	talosClient, err := clusterimport.BuildTalosClient(ctx, options.TalosconfigPath, "", "", nil)
	require.NoError(t, err)

	input := clusterimport.Input{
		LogWriter: &zapio.Writer{Log: logger},
		Nodes:     clusterNodes,
	}

	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, omniState, clusterID)
	require.True(t, state.IsNotFoundError(err))
	require.Nil(t, cluster)

	importContext, err := clusterimport.BuildContext(ctx, input, omniState, imageFactoryClient, talosClient)
	require.NoError(t, err)

	defer importContext.Close() //nolint:errcheck

	err = importContext.Run(ctx)
	require.NoError(t, err)

	cluster, err = safe.StateGetByID[*omni.Cluster](ctx, omniState, clusterID)
	require.NoError(t, err)
	require.NotNil(t, cluster)

	rtestutils.AssertResource(ctx, t, omniState, clusterID, func(r *omni.ClusterStatus, assertion *assert.Assertions) {
		assertion.Equal(specs.ClusterStatusSpec_RUNNING, r.TypedSpec().Value.Phase)
		_, ok := r.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
		assertion.True(ok, "cluster status doesn't have cluster tainted label")
	})

	_, err = talosClient.Version(client.WithNode(ctx, clusterNodes[0]))
	require.NoError(t, err)
}

func testUnlockCluster(t *testing.T, options *TestOptions, clusterID string) {
	t.Log(`Unlock the cluster, assert that cluster has received config changes originating from talos`)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	omniState := options.omniClient.Omni().State()

	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, omniState, clusterID)
	require.NoError(t, err)

	clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	require.NoError(t, err)
	require.Len(t, slices.Collect(clusterMachines.All()), clusterMachines.Len(), "no cluster machines found for cluster %q", clusterID)

	cmcss, err := safe.ReaderListAll[*omni.ClusterMachineConfigStatus](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	require.NoError(t, err)
	require.Len(t, slices.Collect(cmcss.All()), 0, "cluster machine config statuses found for newly imported still locked cluster %q", clusterID)

	clusterMachineIDs := xslices.Map(slices.Collect(clusterMachines.All()), func(t *omni.ClusterMachine) resource.ID {
		return t.Metadata().ID()
	})

	_, err = omniState.UpdateWithConflicts(ctx, cluster.Metadata(), func(r resource.Resource) error {
		r.Metadata().Annotations().Delete(omni.ClusterLocked)

		return nil
	})
	require.NoError(t, err)

	rtestutils.AssertResources(ctx, t, omniState, clusterMachineIDs, func(r *omni.ClusterMachineConfigStatus, assertion *assert.Assertions) {
		assertion.Equal(resource.PhaseRunning, r.Metadata().Phase())
	})
}

func testImportAbort(t *testing.T, options *TestOptions, clusterID string, clusterNodes []string) {
	t.Log(`Abort importing an existing cluster, assert that the cluster is deleted from Omni but still accessible using provided talosconfig.`)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	logger := zaptest.NewLogger(t)

	omniState := options.omniClient.Omni().State()

	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, omniState, clusterID)
	require.NoError(t, err)

	_, err = omniState.UpdateWithConflicts(ctx, cluster.Metadata(), func(r resource.Resource) error {
		r.Metadata().Annotations().Set(omni.ClusterLocked, "")

		return nil
	})
	require.NoError(t, err)

	clusterStatus, err := safe.StateGetByID[*omni.ClusterStatus](ctx, omniState, clusterID)
	require.NoError(t, err)

	_, ok := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
	require.True(t, ok, "cluster status doesn't have cluster tainted label")

	clusterMachines, err := safe.StateListAll[*omni.ClusterMachine](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	require.NoError(t, err)
	require.NotEmpty(t, clusterMachines.Len(), "no cluster machines found for imported cluster %q", clusterID)

	err = clusterimport.Abort(ctx, omniState, clusterID, &zapio.Writer{Log: logger})
	require.NoError(t, err)

	rtestutils.AssertNoResource[*omni.Cluster](ctx, t, omniState, clusterID)

	talosClient, err := clusterimport.BuildTalosClient(ctx, options.TalosconfigPath, "", "", nil)
	require.NoError(t, err)

	_, err = talosClient.Version(client.WithNode(ctx, clusterNodes[0]))
	require.NoError(t, err)
}

// vmState has the structure of "github.com/siderolabs/talos/pkg/provision/providers/vm".State, but we copy its needed parts to avoid importing talos as a dependency for it.
type vmState struct {
	ClusterInfo struct {
		ClusterName string
		Nodes       []struct {
			IPs []netip.Addr
		}
	}
}
