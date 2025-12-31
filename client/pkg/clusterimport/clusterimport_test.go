// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package clusterimport_test

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	imagefactoryconstants "github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/image-factory/pkg/schematic"
	clusterapi "github.com/siderolabs/talos/pkg/machinery/api/cluster"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	machineryconfig "github.com/siderolabs/talos/pkg/machinery/config"
	documentconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	machinetype "github.com/siderolabs/talos/pkg/machinery/config/machine"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	siderolinkmachinery "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/client/pkg/clusterimport"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

type mockNode struct {
	state                state.State
	version              *machine.VersionResponse
	healthCheckClient    *mockHealthCheckClient
	ip                   string
	id                   string
	hostname             string
	schematicID          string
	initialMachineConfig []byte
	appliedMachineConfig []byte
	machineType          machinetype.Type
}

type mockTalosClient struct {
	nodes map[string]*mockNode
	state.State

	logger *zap.Logger
}

func newMockTalosClient(nodes []*mockNode, logger *zap.Logger) *mockTalosClient {
	nodeMap := xslices.ToMap(nodes, func(n *mockNode) (string, *mockNode) { return n.ip, n })
	st := &mockNodeState{nodes: nodeMap}

	return &mockTalosClient{
		nodes:  nodeMap,
		State:  state.WrapCore(st),
		logger: logger,
	}
}

func (m *mockTalosClient) Close() error {
	m.logger.Info("mock close")

	return nil
}

func (m *mockTalosClient) Version(ctx context.Context, _ ...grpc.CallOption) (*machine.VersionResponse, error) {
	m.logger.Info("mock version")

	return getNode(ctx, m.nodes).version, nil
}

func (m *mockTalosClient) ClusterHealthCheck(ctx context.Context, _ time.Duration, _ *clusterapi.ClusterInfo) (clusterapi.ClusterService_HealthCheckClient, error) {
	m.logger.Info("mock cluster health check")

	return getNode(ctx, m.nodes).healthCheckClient, nil
}

func (m *mockTalosClient) ApplyConfiguration(ctx context.Context, req *machine.ApplyConfigurationRequest, _ ...grpc.CallOption) (*machine.ApplyConfigurationResponse, error) {
	m.logger.Info("mock apply configuration")

	node := getNode(ctx, m.nodes)
	if node == nil {
		return nil, fmt.Errorf("mock node not found")
	}

	node.appliedMachineConfig = req.Data

	return &machine.ApplyConfigurationResponse{
		Messages: []*machine.ApplyConfiguration{
			{
				Mode:        machine.ApplyConfigurationRequest_NO_REBOOT,
				ModeDetails: "mock apply done",
			},
		},
	}, nil
}

type mockHealthCheckClient struct {
	grpc.ClientStream

	items   []*clusterapi.HealthCheckProgress
	counter int
}

func (f *mockHealthCheckClient) Recv() (*clusterapi.HealthCheckProgress, error) {
	if f.counter >= len(f.items) {
		return nil, io.EOF
	}

	item := f.items[f.counter]

	f.counter++

	return item, nil
}

func (f *mockHealthCheckClient) CloseSend() error {
	return nil
}

type mockImageFactoryClient struct {
	logger              *zap.Logger
	ensuredSchematicIDs []string
}

func (m *mockImageFactoryClient) EnsureSchematic(_ context.Context, schematic schematic.Schematic) (string, error) {
	marshaled, err := schematic.Marshal()
	if err != nil {
		return "", err
	}

	m.logger.Info("mock ensure schematic", zap.String("schematic", string(marshaled)))

	schematicID, err := schematic.ID()
	if err != nil {
		return "", err
	}

	m.ensuredSchematicIDs = append(m.ensuredSchematicIDs, schematicID)

	return schematicID, nil
}

type testData struct {
	omniState           state.State
	input               *clusterimport.Input
	env                 map[string]string
	talosClient         *mockTalosClient
	imageFactoryClient  *mockImageFactoryClient
	bundle              *secrets.Bundle
	kubernetesVersion   string
	talosVersion        string
	clusterID           string
	anotherTalosVersion string
	controlPlanes       []string
	workers             []string
	nodes               []*mockNode
	invalidTalosState   bool
	invalidOmniState    bool
}

func (data *testData) prepare(ctx context.Context, t *testing.T, logger *zap.Logger) {
	t.Helper()

	data.input.Nodes = slices.Concat(data.controlPlanes, data.workers)
	data.input.LogWriter = &zapio.Writer{Log: logger}

	data.prepareNodes(ctx, t)
	data.prepareOmniState(ctx, t)

	data.imageFactoryClient = &mockImageFactoryClient{
		logger: logger.With(zap.String("component", "image-factory-client")),
	}
	data.talosClient = newMockTalosClient(data.nodes, logger.With(zap.String("component", "talos-client")))
}

func (data *testData) prepareNodes(ctx context.Context, t *testing.T) {
	t.Helper()

	allNodes := slices.Concat(data.controlPlanes, data.workers)
	nodes := make([]*mockNode, 0, len(allNodes))

	version, err := machineryconfig.ParseContractFromVersion(data.talosVersion)
	require.NoError(t, err)

	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), version)
	require.NoError(t, err)

	data.bundle = bundle

	for _, n := range data.controlPlanes {
		nodes = append(nodes, data.prepareNode(ctx, t, n, machinetype.TypeControlPlane))
	}

	for _, n := range data.workers {
		nodes = append(nodes, data.prepareNode(ctx, t, n, machinetype.TypeWorker))
	}

	data.nodes = nodes
}

func (data *testData) prepareNode(ctx context.Context, t *testing.T, node string, machineType machinetype.Type) *mockNode {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	buildHostname := func(n string) string {
		return "node-" + n
	}

	allNodes := slices.Concat(data.controlPlanes, data.workers)

	for _, n := range allNodes {
		require.NoError(t, st.Create(ctx, cluster.NewMember(cluster.NamespaceName, buildHostname(n))))
	}

	machineTypeRes := config.NewMachineType()
	machineTypeRes.SetMachineType(machineType)
	require.NoError(t, st.Create(ctx, machineTypeRes))

	kubeletConfig := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	kubeletConfig.TypedSpec().Image = "ghcr.io/siderolabs/kubelet:" + data.kubernetesVersion
	require.NoError(t, st.Create(ctx, kubeletConfig))

	machineConfig := data.generateMachineConfig(t, machineType)
	require.NoError(t, st.Create(ctx, machineConfig))

	initialMachineConfig, err := machineConfig.Provider().Bytes()
	require.NoError(t, err)

	hostname := buildHostname(node)
	machineID := uuid.NewString()

	var schematicID string

	if !data.invalidTalosState {
		hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
		hostnameStatus.TypedSpec().Hostname = hostname
		require.NoError(t, st.Create(ctx, hostnameStatus))

		systemInformation := hardware.NewSystemInformation(hardware.SystemInformationID)
		systemInformation.TypedSpec().UUID = machineID
		require.NoError(t, st.Create(ctx, systemInformation))

		schematicID = data.createExtensionStatuses(ctx, t, st, machineType)

		if machineType == machinetype.TypeControlPlane {
			apiServerConfig := k8s.NewAPIServerConfig()
			apiServerConfig.TypedSpec().Image = "registry.k8s.io/kube-apiserver:" + data.kubernetesVersion
			require.NoError(t, st.Create(ctx, apiServerConfig))

			schedulerConfig := k8s.NewSchedulerConfig()
			schedulerConfig.TypedSpec().Image = "registry.k8s.io/kube-scheduler:" + data.kubernetesVersion
			require.NoError(t, st.Create(ctx, schedulerConfig))

			controllerManagerConfig := k8s.NewControllerManagerConfig()
			controllerManagerConfig.TypedSpec().Image = "registry.k8s.io/kube-controller-manager:" + data.kubernetesVersion
			require.NoError(t, st.Create(ctx, controllerManagerConfig))

			clusterInfo := cluster.NewInfo()
			clusterInfo.TypedSpec().ClusterName = data.clusterID

			require.NoError(t, st.Create(ctx, clusterInfo))
		}
	}

	talosVersion := data.talosVersion

	if data.anotherTalosVersion != "" && node == data.controlPlanes[0] {
		talosVersion = data.anotherTalosVersion
	}

	return &mockNode{
		ip:                   node,
		id:                   machineID,
		hostname:             hostname,
		initialMachineConfig: initialMachineConfig,
		machineType:          machineType,
		state:                st,
		schematicID:          schematicID,
		version: &machine.VersionResponse{
			Messages: []*machine.Version{
				{
					Version: &machine.VersionInfo{
						Tag: talosVersion,
					},
				},
			},
		},
		healthCheckClient: &mockHealthCheckClient{
			items: []*clusterapi.HealthCheckProgress{
				{Message: "waiting for etcd to be healthy: ..."},
				{Message: "waiting for etcd to be healthy: OK"},
				{Message: "waiting for etcd members to be consistent across nodes: ..."},
				{Message: "waiting for etcd members to be consistent across nodes: OK"},
				{Message: "waiting for etcd members to be control plane nodes: ..."},
				{Message: "waiting for etcd members to be control plane nodes: OK"},
				{Message: "waiting for apid to be ready: ..."},
				{Message: "waiting for apid to be ready: OK"},
				{Message: "waiting for all nodes memory sizes: ..."},
				{Message: "waiting for all nodes memory sizes: OK"},
				{Message: "waiting for all nodes disk sizes: ..."},
				{Message: "waiting for all nodes disk sizes: OK"},
				{Message: "waiting for kubelet to be healthy: ..."},
				{Message: "waiting for kubelet to be healthy: OK"},
				{Message: "waiting for all nodes to finish boot sequence: ..."},
				{Message: "waiting for all nodes to finish boot sequence: OK"},
				{Message: "waiting for all k8s nodes to report: ..."},
				{Message: "waiting for all k8s nodes to report: OK"},
				{Message: "waiting for all k8s nodes to report ready: ..."},
				{Message: "waiting for all k8s nodes to report ready: OK"},
				{Message: "waiting for all control plane static pods to be running: ..."},
				{Message: "waiting for all control plane static pods to be running: OK"},
				{Message: "waiting for all control plane components to be ready: ..."},
				{Message: "waiting for all control plane components to be ready: OK"},
				{Message: "waiting for kube-proxy to report ready: ..."},
				{Message: "waiting for kube-proxy to report ready: OK"},
				{Message: "waiting for coredns to report ready: ..."},
				{Message: "waiting for coredns to report ready: OK"},
				{Message: "waiting for all k8s nodes to report schedulable: ..."},
				{Message: "waiting for all k8s nodes to report schedulable: OK"},
			},
		},
	}
}

func (data *testData) createExtensionStatuses(ctx context.Context, t *testing.T, st state.State, machineType machinetype.Type) string {
	t.Helper()

	extraKernelArgs := []string{
		"console=tty0",
	}

	if machineType == machinetype.TypeWorker {
		extraKernelArgs = append(extraKernelArgs, "console=tty1")
	}

	sch := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: []string{
					"siderolabs/hello-world-service",
				},
			},
			ExtraKernelArgs: extraKernelArgs,
		},
	}

	marshaled, err := sch.Marshal()
	require.NoError(t, err)

	id, err := sch.ID()
	require.NoError(t, err)

	ext1 := runtime.NewExtensionStatus(runtime.NamespaceName, "0")
	ext1.TypedSpec().Metadata.Name = "hello-world-service"
	ext1.TypedSpec().Metadata.Version = "v1.6.7"
	require.NoError(t, st.Create(ctx, ext1))

	ext2 := runtime.NewExtensionStatus(runtime.NamespaceName, "1")
	ext2.TypedSpec().Metadata.Name = imagefactoryconstants.SchematicIDExtensionName
	ext2.TypedSpec().Metadata.Version = id
	ext2.TypedSpec().Metadata.ExtraInfo = string(marshaled)
	require.NoError(t, st.Create(ctx, ext2))

	return id
}

func (data *testData) generateMachineConfig(t *testing.T, machineType machinetype.Type) *config.MachineConfig {
	t.Helper()

	versionContract, err := machineryconfig.ParseContractFromVersion(data.talosVersion)
	require.NoError(t, err)

	input, err := generate.NewInput(data.clusterID, "https://localhost:6443", constants.DefaultKubernetesVersion,
		generate.WithSecretsBundle(data.bundle), generate.WithVersionContract(versionContract))
	require.NoError(t, err)

	conf, err := input.Config(machineType)
	require.NoError(t, err)

	machineConfig, ok := conf.Machine().(*v1alpha1.MachineConfig)
	require.True(t, ok)

	machineConfig.MachineEnv = data.env

	return config.NewMachineConfig(conf)
}

func (data *testData) prepareOmniState(ctx context.Context, t *testing.T) {
	t.Helper()

	omniState := state.WrapCore(namespaced.NewState(inmem.Build))

	if !data.invalidOmniState {
		defaultJoinToken := siderolink.NewDefaultJoinToken()
		defaultJoinToken.TypedSpec().Value.TokenId = "test-join-token"

		require.NoError(t, omniState.Create(ctx, defaultJoinToken))

		siderolinkAPIConfig := siderolink.NewAPIConfig()

		siderolinkAPIConfig.TypedSpec().Value.LogsPort = 1234
		siderolinkAPIConfig.TypedSpec().Value.EventsPort = 4321
		siderolinkAPIConfig.TypedSpec().Value.MachineApiAdvertisedUrl = "https://example.com:8443"

		require.NoError(t, omniState.Create(ctx, siderolinkAPIConfig))

		talosVersionRes := omni.NewTalosVersion(strings.TrimLeft(data.talosVersion, "v"))
		talosVersionRes.TypedSpec().Value.CompatibleKubernetesVersions = []string{strings.TrimLeft(data.kubernetesVersion, "v")}
		require.NoError(t, omniState.Create(ctx, talosVersionRes))

		if data.anotherTalosVersion != "" {
			anotherTalosVersionRes := omni.NewTalosVersion(strings.TrimLeft(data.anotherTalosVersion, "v"))
			anotherTalosVersionRes.TypedSpec().Value.CompatibleKubernetesVersions = []string{strings.TrimLeft(data.kubernetesVersion, "v")}
			require.NoError(t, omniState.Create(ctx, anotherTalosVersionRes))
		}
	}

	data.omniState = omniState
}

type testCase struct {
	prepareFunc func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData)
	assertFunc  func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData)
	name        string
	testData    testData
}

//nolint:gocognit,maintidx
func TestImportContext(t *testing.T) {
	t.Parallel()

	tests := []testCase{
		{
			name: "success",
			testData: testData{
				clusterID:         "test-1",
				talosVersion:      "v1.11.2",
				kubernetesVersion: "v1.34.1",
				controlPlanes:     []string{"172.20.0.2"},
				workers:           []string{"172.20.0.3"},
				env:               map[string]string{"TEST_KEY": "test-val"},
				input: &clusterimport.Input{
					BackupOutput: filepath.Join(t.TempDir(), "backup.zip"),
					Versions: clusterimport.Versions{
						TalosVersion:             "v1.11.2",
						KubernetesVersion:        "v1.34.1",
						InitialTalosVersion:      "v1.11.2",
						InitialKubernetesVersion: "v1.34.1",
					},
				},
			},
			prepareFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData) {
				testData.prepare(ctx, t, logger)
			},
			assertFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, data *testData) {
				importContext, err := clusterimport.BuildContext(ctx, *data.input, data.omniState, data.imageFactoryClient, data.talosClient)
				require.NoError(t, err)

				defer func() {
					require.NoError(t, importContext.Close())
				}()

				require.NoError(t, importContext.Run(ctx))

				rtestutils.AssertResource(ctx, t, data.omniState, data.clusterID, func(res *omni.Cluster, assertion *assert.Assertions) {
					assertion.Equal(res.TypedSpec().Value.TalosVersion, strings.TrimLeft(data.talosVersion, "v"))
					assertion.Equal(res.TypedSpec().Value.KubernetesVersion, strings.TrimLeft(data.kubernetesVersion, "v"))

					_, locked := res.Metadata().Annotations().Get(omni.ClusterLocked)
					assertion.True(locked)

					_, importIsInProgress := res.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)
					assertion.True(importIsInProgress)
				})

				rtestutils.AssertResource(ctx, t, data.omniState, omni.ControlPlanesResourceID(data.clusterID), func(res *omni.MachineSet, assertion *assert.Assertions) {
					clusterLabel, _ := res.Metadata().Labels().Get(omni.LabelCluster)
					assertion.Equal(clusterLabel, data.clusterID)
				})

				rtestutils.AssertResource(ctx, t, data.omniState, omni.WorkersResourceID(data.clusterID), func(res *omni.MachineSet, assertion *assert.Assertions) {
					clusterLabel, _ := res.Metadata().Labels().Get(omni.LabelCluster)
					assertion.Equal(clusterLabel, data.clusterID)
				})

				for _, node := range data.nodes {
					isCP := node.machineType == machinetype.TypeControlPlane

					rtestutils.AssertResource(ctx, t, data.omniState, node.id, func(res *omni.MachineSetNode, assertion *assert.Assertions) {
						clusterLabel, _ := res.Metadata().Labels().Get(omni.LabelCluster)
						assertion.Equal(clusterLabel, data.clusterID)

						machineSetLabel, _ := res.Metadata().Labels().Get(omni.LabelMachineSet)

						_, hasCPLabel := res.Metadata().Labels().Get(omni.LabelControlPlaneRole)
						_, hasWorkerLabel := res.Metadata().Labels().Get(omni.LabelWorkerRole)

						if isCP {
							assertion.True(hasCPLabel)
							assertion.False(hasWorkerLabel)

							assertion.Equal(machineSetLabel, omni.ControlPlanesResourceID(data.clusterID))
						} else {
							assertion.False(hasCPLabel)
							assertion.True(hasWorkerLabel)

							assertion.Equal(machineSetLabel, omni.WorkersResourceID(data.clusterID))
						}
					})

					patchList, listErr := safe.StateListAll[*omni.ConfigPatch](ctx, data.omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelClusterMachine, node.id)))
					require.NoError(t, listErr)

					if isCP {
						assert.Equal(t, 2, patchList.Len())
					} else {
						assert.Equal(t, 1, patchList.Len())
					}

					for i, patch := range slices.Collect(patchList.All()) {
						clusterLabel, _ := patch.Metadata().Labels().Get(omni.LabelCluster)
						assert.Equal(t, clusterLabel, data.clusterID)

						machineLabel, _ := patch.Metadata().Labels().Get(omni.LabelClusterMachine)
						assert.Equal(t, machineLabel, node.id)

						if i == 0 {
							uncompressedData, patchErr := patch.TypedSpec().Value.GetUncompressedData()
							require.NoError(t, patchErr)

							require.Contains(t, string(uncompressedData.Data()), "TEST_KEY: test-val")
							uncompressedData.Free()
						}
					}

					assertAppliedMachineConfig(t, node)
				}

				assertBackup(t, data.nodes, data.input.BackupOutput)

				schematicIDs := xslices.Map(data.nodes, func(n *mockNode) string { return n.schematicID })
				assert.ElementsMatch(t, schematicIDs, data.imageFactoryClient.ensuredSchematicIDs)
			},
		},
		{
			name: "success with initial talos and kubernetes versions",
			testData: testData{
				clusterID:         "test-1",
				talosVersion:      "v1.11.2",
				kubernetesVersion: "v1.34.1",
				controlPlanes:     []string{"172.20.0.2", "172.20.0.3", "172.20.0.4"},
				workers:           []string{},
				input: &clusterimport.Input{
					BackupOutput: filepath.Join(t.TempDir(), "backup.zip"),
					Versions: clusterimport.Versions{
						InitialTalosVersion:      "v1.6.0",
						InitialKubernetesVersion: "v1.24.0",
					},
				},
			},
			prepareFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData) {
				testData.prepare(ctx, t, logger)
			},
			assertFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, data *testData) {
				importContext, err := clusterimport.BuildContext(ctx, *data.input, data.omniState, data.imageFactoryClient, data.talosClient)
				require.NoError(t, err)

				defer func() {
					require.NoError(t, importContext.Close())
				}()

				require.NoError(t, importContext.Run(ctx))

				rtestutils.AssertResource(ctx, t, data.omniState, data.clusterID, func(res *omni.Cluster, assertion *assert.Assertions) {
					assertion.Equal(res.TypedSpec().Value.TalosVersion, strings.TrimLeft(data.talosVersion, "v"))
					assertion.Equal(res.TypedSpec().Value.KubernetesVersion, strings.TrimLeft(data.kubernetesVersion, "v"))

					_, locked := res.Metadata().Annotations().Get(omni.ClusterLocked)
					assertion.True(locked)

					_, importIsInProgress := res.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)
					assertion.True(importIsInProgress)
				})

				rtestutils.AssertResource(ctx, t, data.omniState, omni.ControlPlanesResourceID(data.clusterID), func(res *omni.MachineSet, assertion *assert.Assertions) {
					clusterLabel, _ := res.Metadata().Labels().Get(omni.LabelCluster)
					assertion.Equal(clusterLabel, data.clusterID)
				})

				rtestutils.AssertResource(ctx, t, data.omniState, omni.WorkersResourceID(data.clusterID), func(res *omni.MachineSet, assertion *assert.Assertions) {
					clusterLabel, _ := res.Metadata().Labels().Get(omni.LabelCluster)
					assertion.Equal(clusterLabel, data.clusterID)
				})

				for _, node := range data.nodes {
					isCP := node.machineType == machinetype.TypeControlPlane

					rtestutils.AssertResource(ctx, t, data.omniState, node.id, func(res *omni.MachineSetNode, assertion *assert.Assertions) {
						clusterLabel, _ := res.Metadata().Labels().Get(omni.LabelCluster)
						assertion.Equal(clusterLabel, data.clusterID)

						machineSetLabel, _ := res.Metadata().Labels().Get(omni.LabelMachineSet)

						_, hasCPLabel := res.Metadata().Labels().Get(omni.LabelControlPlaneRole)
						_, hasWorkerLabel := res.Metadata().Labels().Get(omni.LabelWorkerRole)

						if isCP {
							assertion.True(hasCPLabel)
							assertion.False(hasWorkerLabel)

							assertion.Equal(machineSetLabel, omni.ControlPlanesResourceID(data.clusterID))
						} else {
							assertion.False(hasCPLabel)
							assertion.True(hasWorkerLabel)

							assertion.Equal(machineSetLabel, omni.WorkersResourceID(data.clusterID))
						}
					})

					patchList, listErr := safe.StateListAll[*omni.ConfigPatch](ctx, data.omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelClusterMachine, node.id)))
					require.NoError(t, listErr)

					if isCP {
						assert.Equal(t, 2, patchList.Len())
					} else {
						assert.Equal(t, 1, patchList.Len())
					}

					for patch := range patchList.All() {
						clusterLabel, _ := patch.Metadata().Labels().Get(omni.LabelCluster)
						assert.Equal(t, clusterLabel, data.clusterID)

						machineLabel, _ := patch.Metadata().Labels().Get(omni.LabelClusterMachine)
						assert.Equal(t, machineLabel, node.id)
					}

					assertAppliedMachineConfig(t, node)
				}

				schematicIDs := xslices.Map(data.nodes, func(n *mockNode) string { return n.schematicID })
				for _, id := range schematicIDs {
					assert.Contains(t, data.imageFactoryClient.ensuredSchematicIDs, id)
				}

				assertBackup(t, data.nodes, data.input.BackupOutput)
			},
		},
		{
			name: "dry-run",
			testData: testData{
				clusterID:         "test-1",
				talosVersion:      "v1.11.2",
				kubernetesVersion: "v1.34.1",
				controlPlanes:     []string{"172.20.0.2"},
				workers:           []string{"172.20.0.3"},
				input: &clusterimport.Input{
					BackupOutput: filepath.Join(t.TempDir(), "backup.zip"),
					DryRun:       true,
				},
			},
			prepareFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData) {
				testData.prepare(ctx, t, logger)
			},
			assertFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, data *testData) {
				importContext, err := clusterimport.BuildContext(ctx, *data.input, data.omniState, data.imageFactoryClient, data.talosClient)
				require.NoError(t, err)

				defer func() {
					require.NoError(t, importContext.Close())
				}()

				require.NoError(t, importContext.Run(ctx))
				rtestutils.AssertNoResource[*omni.Cluster](ctx, t, data.omniState, data.clusterID)
				rtestutils.AssertNoResource[*omni.MachineSet](ctx, t, data.omniState, omni.ControlPlanesResourceID(data.clusterID))
				rtestutils.AssertNoResource[*omni.MachineSet](ctx, t, data.omniState, omni.WorkersResourceID(data.clusterID))

				for _, node := range data.nodes {
					rtestutils.AssertNoResource[*omni.MachineSetNode](ctx, t, data.omniState, node.id)

					patchList, listErr := safe.StateListAll[*omni.ConfigPatch](ctx, data.omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelClusterMachine, node.id)))
					require.NoError(t, listErr)
					require.Equal(t, patchList.Len(), 0)

					require.Nil(t, node.appliedMachineConfig)
				}

				assertNoBackup(t, data.input.BackupOutput)
			},
		},
		{
			name: "build context failure",
			testData: testData{
				clusterID:         "test-1",
				talosVersion:      "v1.11.2",
				kubernetesVersion: "v1.34.1",
				controlPlanes:     []string{"172.20.0.2"},
				workers:           []string{"172.20.0.3"},
				input: &clusterimport.Input{
					BackupOutput: filepath.Join(t.TempDir(), "backup.zip"),
				},
				invalidTalosState: true,
			},
			prepareFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData) {
				testData.prepare(ctx, t, logger)
			},
			assertFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, data *testData) {
				importContext, err := clusterimport.BuildContext(ctx, *data.input, data.omniState, data.imageFactoryClient, data.talosClient)
				require.Error(t, err)
				require.Contains(t, err.Error(), "failed to collect node info")
				require.Nil(t, importContext)

				assert.Len(t, data.imageFactoryClient.ensuredSchematicIDs, 0)
			},
		},
		{
			name: "validation failure",
			testData: testData{
				clusterID:         "test-1",
				talosVersion:      "v1.11.2",
				kubernetesVersion: "v1.34.1",
				controlPlanes:     []string{"172.20.0.2"},
				workers:           []string{"172.20.0.3"},
				input: &clusterimport.Input{
					BackupOutput: filepath.Join(t.TempDir(), "backup.zip"),
				},
				anotherTalosVersion: "v1.12.0",
			},
			prepareFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData) {
				testData.prepare(ctx, t, logger)
			},
			assertFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, data *testData) {
				importContext, err := clusterimport.BuildContext(ctx, *data.input, data.omniState, data.imageFactoryClient, data.talosClient)
				require.NoError(t, err)

				defer func() {
					require.NoError(t, importContext.Close())
				}()

				err = importContext.Run(ctx)
				require.ErrorIs(t, err, clusterimport.ErrValidation)
				require.ErrorContains(t, err, "multiple different Talos versions found: v1.11.2, v1.12.0")
			},
		},
		{
			name: "validation failure, continue with force",
			testData: testData{
				clusterID:         "test-1",
				talosVersion:      "v1.11.2",
				kubernetesVersion: "v1.34.1",
				controlPlanes:     []string{"172.20.0.2"},
				workers:           []string{"172.20.0.3"},
				input: &clusterimport.Input{
					BackupOutput: filepath.Join(t.TempDir(), "backup.zip"),
					Force:        true,
				},
				anotherTalosVersion: "v1.12.0",
			},
			prepareFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData) {
				testData.prepare(ctx, t, logger)
			},
			assertFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, data *testData) {
				importContext, err := clusterimport.BuildContext(ctx, *data.input, data.omniState, data.imageFactoryClient, data.talosClient)
				require.NoError(t, err)

				defer func() {
					require.NoError(t, importContext.Close())
				}()

				err = importContext.Run(ctx)
				require.NoError(t, err)
			},
		},
		{
			name: "not supported talos version",
			testData: testData{
				clusterID:         "test-1",
				talosVersion:      "1.5.2",
				kubernetesVersion: "1.27.1",
				controlPlanes:     []string{"172.20.0.2"},
				workers:           []string{"172.20.0.3"},
				input: &clusterimport.Input{
					BackupOutput: filepath.Join(t.TempDir(), "backup.zip"),
				},
			},
			prepareFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, testData *testData) {
				testData.prepare(ctx, t, logger)
			},
			assertFunc: func(ctx context.Context, t *testing.T, logger *zap.Logger, data *testData) {
				importContext, err := clusterimport.BuildContext(ctx, *data.input, data.omniState, data.imageFactoryClient, data.talosClient)
				require.Error(t, err)
				require.ErrorContains(t, err, "minimum required version of talos is")
				require.Nil(t, importContext)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			t.Cleanup(cancel)
			logger := zaptest.NewLogger(t)

			tt.prepareFunc(ctx, t, logger, &tt.testData)

			tt.assertFunc(ctx, t, logger, &tt.testData)
		})
	}
}

func assertBackup(t *testing.T, nodes []*mockNode, backupPath string) {
	t.Helper()

	// read zip
	zipReader, err := zip.OpenReader(backupPath)
	require.NoError(t, err)

	defer zipReader.Close() //nolint:errcheck

	require.Len(t, zipReader.File, len(nodes))

	for _, node := range nodes {
		assertNodeBackup(t, node, &zipReader.Reader)
	}
}

func assertNoBackup(t *testing.T, backupPath string) {
	t.Helper()

	_, err := zip.OpenReader(backupPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such file or directory")
}

func assertNodeBackup(t *testing.T, node *mockNode, zipReader *zip.Reader) {
	t.Helper()

	expected := fmt.Sprintf("%s-%s.yaml", node.hostname, node.ip)

	rdr, err := zipReader.Open(expected)
	require.NoError(t, err)

	defer rdr.Close() //nolint:errcheck

	data, err := io.ReadAll(rdr)
	require.NoError(t, err)

	assert.Equal(t, string(node.initialMachineConfig), string(data))
}

func assertAppliedMachineConfig(t *testing.T, node *mockNode) {
	t.Helper()

	require.NotNil(t, node.appliedMachineConfig)

	initialConf, err := configloader.NewFromBytes(node.initialMachineConfig)
	require.NoError(t, err)

	appliedConf, err := configloader.NewFromBytes(node.appliedMachineConfig)
	require.NoError(t, err)

	toConfMap := func(conf machineryconfig.Provider) map[string]documentconfig.Document {
		return xslices.ToMap(conf.Documents(), func(doc documentconfig.Document) (string, documentconfig.Document) {
			if named, ok := doc.(documentconfig.NamedDocument); ok {
				return fmt.Sprintf("%s/%s/%s", doc.APIVersion(), doc.Kind(), named.Name()), doc
			}

			return fmt.Sprintf("%s/%s", doc.APIVersion(), doc.Kind()), doc
		})
	}

	initialConfMap := toConfMap(initialConf)
	appliedConfMap := toConfMap(appliedConf)
	extraConfMap := make(map[string]documentconfig.Document, 3)

	for id := range initialConfMap {
		_, ok := appliedConfMap[id]
		require.Truef(t, ok, "document %s not found in applied config", id)
	}

	for id := range appliedConfMap {
		if _, ok := initialConfMap[id]; !ok {
			extraConfMap[id] = appliedConfMap[id]
		}
	}

	require.Len(t, extraConfMap, 3)

	for _, doc := range extraConfMap {
		switch configDoc := doc.(type) {
		case *runtimecfg.KmsgLogV1Alpha1:
			assert.Equal(t, "omni-kmsg", configDoc.Name())
			assert.Equal(t, "tcp://[fdae:41e4:649b:9303::1]:1234", configDoc.KmsgLogURL.String())

		case *siderolinkmachinery.ConfigV1Alpha1:
			assert.Equal(t, "https://example.com:8443?jointoken=test-join-token", configDoc.APIUrlConfig.String())
		case *runtimecfg.EventSinkV1Alpha1:
			assert.Equal(t, "[fdae:41e4:649b:9303::1]:4321", configDoc.Endpoint)
		default:
			require.Failf(t, "unexpected document in applied config", "document: %T", doc)
		}
	}
}

type mockNodeState struct {
	nodes map[string]*mockNode
}

func (m *mockNodeState) Get(ctx context.Context, pointer resource.Pointer, option ...state.GetOption) (resource.Resource, error) {
	return m.nodes[getNode(ctx, m.nodes).ip].state.Get(ctx, pointer, option...)
}

func (m *mockNodeState) List(ctx context.Context, kind resource.Kind, option ...state.ListOption) (resource.List, error) {
	return m.nodes[getNode(ctx, m.nodes).ip].state.List(ctx, kind, option...)
}

func (m *mockNodeState) Create(ctx context.Context, resource resource.Resource, option ...state.CreateOption) error {
	return m.nodes[getNode(ctx, m.nodes).ip].state.Create(ctx, resource, option...)
}

func (m *mockNodeState) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	return m.nodes[getNode(ctx, m.nodes).ip].state.Update(ctx, newResource, opts...)
}

func (m *mockNodeState) Destroy(ctx context.Context, pointer resource.Pointer, option ...state.DestroyOption) error {
	return m.nodes[getNode(ctx, m.nodes).ip].state.Destroy(ctx, pointer, option...)
}

func (m *mockNodeState) Watch(ctx context.Context, pointer resource.Pointer, events chan<- state.Event, option ...state.WatchOption) error {
	return m.nodes[getNode(ctx, m.nodes).ip].state.Watch(ctx, pointer, events, option...)
}

func (m *mockNodeState) WatchKind(ctx context.Context, kind resource.Kind, events chan<- state.Event, option ...state.WatchKindOption) error {
	return m.nodes[getNode(ctx, m.nodes).ip].state.WatchKind(ctx, kind, events, option...)
}

func (m *mockNodeState) WatchKindAggregated(ctx context.Context, kind resource.Kind, c chan<- []state.Event, option ...state.WatchKindOption) error {
	return m.nodes[getNode(ctx, m.nodes).ip].state.WatchKindAggregated(ctx, kind, c, option...)
}

func getNode(ctx context.Context, nodeMap map[string]*mockNode) *mockNode {
	md, _ := metadata.FromOutgoingContext(ctx)

	return nodeMap[md.Get("node")[0]]
}
