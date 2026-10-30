// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"archive/zip"
	"context"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapio"
	"go.uber.org/zap/zaptest"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/clusterimport"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func testClusterImport(t *testing.T, options *TestOptions) {
	t.Parallel()

	var (
		clusterID    string
		clusterNodes []string
	)

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

	hostnames := getNodeHostnames(t, options, clusterNodes)
	firstBackup := filepath.Join(t.TempDir(), "first-import-backup.zip")
	secondBackup := filepath.Join(t.TempDir(), "second-import-backup.zip")

	t.Run(
		"ClusterShouldBeImported",
		func(t *testing.T) { testImport(t, options, clusterID, clusterNodes, firstBackup) },
	)

	t.Run(
		"ClusterImportShouldBeAborted",
		func(t *testing.T) { testImportAbort(t, options, clusterID, clusterNodes) },
	)

	t.Run(
		"NodesShouldBeRestoredFromBackup",
		func(t *testing.T) { testRestoreFromBackup(t, options, clusterNodes, firstBackup) },
	)

	t.Run(
		"ClusterShouldBeImportedAgain",
		func(t *testing.T) { testImport(t, options, clusterID, clusterNodes, secondBackup) },
	)

	t.Run(
		"UnlockedClusterShouldReceiveConfigChanges",
		func(t *testing.T) { testUnlockCluster(t, options, clusterID, clusterNodes, hostnames) },
	)
}

func testImport(t *testing.T, options *TestOptions, clusterID string, clusterNodes []string, backupOutput string) {
	t.Log(`Import an existing talos cluster, assert that the cluster is ready and the nodes are accessible using provided talosconfig.`)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	logger := zaptest.NewLogger(t)

	omniState := options.omniClient.Omni().State()

	talosClient, err := clusterimport.BuildTalosClient(ctx, options.TalosconfigPath, "", "", nil)
	require.NoError(t, err)

	input := clusterimport.Input{
		LogWriter:    &zapio.Writer{Log: logger},
		Nodes:        clusterNodes,
		BackupOutput: backupOutput,
		Versions: clusterimport.Versions{
			TalosVersion:        options.MachineOptions.TalosVersion,
			InitialTalosVersion: options.StableTalosVersion,
		},
	}

	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, omniState, clusterID)
	require.True(t, state.IsNotFoundError(err))
	require.Nil(t, cluster)

	importContext, err := clusterimport.BuildContext(ctx, input, omniState, clusterimport.NewSchematicEnsurer(options.omniClient), talosClient)
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

func testUnlockCluster(t *testing.T, options *TestOptions, clusterID string, clusterNodes []string, hostnames map[string]string) {
	t.Log(`Unlock the cluster, assert that cluster has received config changes originating from talos without config errors and the nodes kept their hostnames`)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
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
		assertion.Empty(r.TypedSpec().Value.LastConfigError)
		assertion.NotEmpty(r.TypedSpec().Value.ClusterMachineConfigSha256, "the config is not applied yet")
	})

	require.Equal(t, hostnames, getNodeHostnames(t, options, clusterNodes), "the nodes must keep their hostnames")
}

func testImportAbort(t *testing.T, options *TestOptions, clusterID string, clusterNodes []string) {
	t.Log(`Abort importing an existing cluster, assert that the cluster is deleted from Omni without any config change on the nodes and the nodes are still accessible using provided talosconfig.`)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	logger := zaptest.NewLogger(t)

	omniState := options.omniClient.Omni().State()

	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, omniState, clusterID)
	require.NoError(t, err)

	_, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked)
	require.True(t, locked, "cluster %q is not locked", clusterID)

	clusterStatus, err := safe.StateGetByID[*omni.ClusterStatus](ctx, omniState, clusterID)
	require.NoError(t, err)

	_, ok := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
	require.True(t, ok, "cluster status doesn't have cluster tainted label")

	clusterMachines, err := safe.StateListAll[*omni.ClusterMachine](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	require.NoError(t, err)
	require.NotEmpty(t, clusterMachines.Len(), "no cluster machines found for imported cluster %q", clusterID)

	talosClient, err := clusterimport.BuildTalosClient(ctx, options.TalosconfigPath, "", "", nil)
	require.NoError(t, err)

	defer talosClient.Close() //nolint:errcheck

	configVersions := getMachineConfigVersions(ctx, t, talosClient, clusterNodes)

	err = clusterimport.Abort(ctx, omniState, clusterID, &zapio.Writer{Log: logger})
	require.NoError(t, err)

	rtestutils.AssertNoResource[*omni.Cluster](ctx, t, omniState, clusterID)

	require.Equal(t, configVersions, getMachineConfigVersions(ctx, t, talosClient, clusterNodes), "the machine config of the nodes must not change on abort")

	_, err = talosClient.Version(client.WithNode(ctx, clusterNodes[0]))
	require.NoError(t, err)
}

func testRestoreFromBackup(t *testing.T, options *TestOptions, clusterNodes []string, backupPath string) {
	t.Log(`Apply the machine configs from the backup taken during the import, assert that the nodes are disconnected from Omni.`)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	talosClient, err := clusterimport.BuildTalosClient(ctx, options.TalosconfigPath, "", "", nil)
	require.NoError(t, err)

	defer talosClient.Close() //nolint:errcheck

	backup, err := zip.OpenReader(backupPath)
	require.NoError(t, err)

	defer backup.Close() //nolint:errcheck

	for _, node := range clusterNodes {
		idx := slices.IndexFunc(backup.File, func(f *zip.File) bool { return strings.HasSuffix(f.Name, "-"+node+".yaml") })
		require.GreaterOrEqual(t, idx, 0, "no backup found for node %q", node)

		config := readZipFile(t, backup.File[idx])
		nodeCtx := client.WithNode(ctx, node)

		_, err = talosClient.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: config,
			Mode: machineapi.ApplyConfigurationRequest_AUTO,
		})
		require.NoError(t, err)

		rtestutils.AssertResource(nodeCtx, t, talosClient, configres.ActiveID, func(r *configres.MachineConfig, assertion *assert.Assertions) {
			assertion.Nil(r.Provider().SideroLink(), "node %q is still connected to Omni", node)
		})
	}
}

func getNodeHostnames(t *testing.T, options *TestOptions, nodes []string) map[string]string {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	talosClient, err := clusterimport.BuildTalosClient(ctx, options.TalosconfigPath, "", "", nil)
	require.NoError(t, err)

	defer talosClient.Close() //nolint:errcheck

	hostnames := make(map[string]string, len(nodes))

	for _, node := range nodes {
		hostnameStatus, err := safe.StateGetByID[*network.HostnameStatus](client.WithNode(ctx, node), talosClient, network.HostnameID)
		require.NoError(t, err)

		hostnames[node] = hostnameStatus.TypedSpec().Hostname
	}

	return hostnames
}

func getMachineConfigVersions(ctx context.Context, t *testing.T, talosClient clusterimport.TalosClient, nodes []string) map[string]string {
	versions := make(map[string]string, len(nodes))

	for _, node := range nodes {
		machineConfig, err := safe.StateGetByID[*configres.MachineConfig](client.WithNode(ctx, node), talosClient, configres.ActiveID)
		require.NoError(t, err)

		versions[node] = machineConfig.Metadata().Version().String()
	}

	return versions
}

func readZipFile(t *testing.T, f *zip.File) []byte {
	r, err := f.Open()
	require.NoError(t, err)

	defer r.Close() //nolint:errcheck

	data, err := io.ReadAll(r)
	require.NoError(t, err)

	return data
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
