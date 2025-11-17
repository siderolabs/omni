// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package clusterimport provides functionality to import existing Talos clusters into Omni.
package clusterimport

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	xmaps "github.com/siderolabs/gen/maps"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/configdiff"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	clusterres "github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/image"
	"github.com/siderolabs/omni/client/pkg/machineconfig"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// Context holds the state for the import operation.
type Context struct {
	talosClient            talosClientWrapper
	omniState              state.State
	imageFactoryClient     ImageFactoryClient
	machineSetNodesMap     map[resource.ID]*omni.MachineSetNode
	importedClusterSecrets *omni.ImportedClusterSecrets
	controlPlaneMachineSet *omni.MachineSet
	cluster                *omni.Cluster
	joinOptions            *siderolink.JoinOptions
	nodeInfoMap            map[string]nodeInfo
	configPatchesMap       map[resource.ID]*omni.ConfigPatch
	ensuredSchematicIDs    map[string]struct{}
	workerMachineSet       *omni.MachineSet
	ClusterID              string
	input                  Input
}

// BuildContext builds the import context by collecting information from the existing Talos cluster.
// nolint:gocyclo,cyclop
func BuildContext(ctx context.Context, input Input, omniState state.State, imageFactoryClient ImageFactoryClient, talosClient TalosClient) (*Context, error) {
	input.logf("discovering Talos cluster state...")

	talosCli := talosClientWrapper{talosClient}

	firstNode := input.Nodes[0]
	nodeCount := len(input.Nodes)

	members, err := talosCli.getMembers(ctx, firstNode)
	if err != nil {
		return nil, fmt.Errorf("failed to get members from node %q: %w", firstNode, err)
	}

	if len(members) == 0 {
		input.logf("failed to discover any members from the cluster. skipping member validation...\n > assuming discovery service is disabled and all the nodes have been provided as input")
	} else if len(members) != nodeCount {
		return nil, fmt.Errorf("number of members in cluster (%d) does not match number of provided nodes (%d)", len(members), nodeCount)
	}

	defaultJoinToken, err := safe.ReaderGetByID[*siderolinkres.DefaultJoinToken](ctx, omniState, siderolinkres.NewDefaultJoinToken().Metadata().ID())
	if err != nil {
		return nil, fmt.Errorf("failed to get default join token: %w", err)
	}

	apiConfig, err := safe.ReaderGetByID[*siderolinkres.APIConfig](ctx, omniState, siderolinkres.NewAPIConfig().Metadata().ID())
	if err != nil {
		return nil, fmt.Errorf("failed to get API config: %w", err)
	}

	joinOpts, err := siderolink.NewJoinOptions(
		siderolink.WithJoinToken(defaultJoinToken.TypedSpec().Value.TokenId),
		siderolink.WithMachineAPIURL(apiConfig.TypedSpec().Value.MachineApiAdvertisedUrl),
		siderolink.WithEventSinkPort(int(apiConfig.TypedSpec().Value.EventsPort)),
		siderolink.WithLogServerPort(int(apiConfig.TypedSpec().Value.LogsPort)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create join input: %w", err)
	}

	controlPlane := ""

	nodeInfoMap := make(map[string]nodeInfo, len(input.Nodes))
	for _, node := range input.Nodes {
		info, nodeErr := collectNodeInfo(ctx, talosCli, node)
		if nodeErr != nil {
			return nil, fmt.Errorf("failed to collect node info: %w", nodeErr)
		}

		nodeInfoMap[node] = info
		if controlPlane == "" && info.isControlPlane {
			controlPlane = node
		}
	}

	cpInfo := nodeInfoMap[controlPlane]

	info, err := safe.ReaderGetByID[*clusterres.Info](talosclient.WithNode(ctx, controlPlane), talosCli, clusterres.InfoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info from node %q: %w", controlPlane, err)
	}

	clusterID := info.TypedSpec().ClusterName

	input.logf("importing cluster %q to Omni", clusterID)

	existingCluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, omniState, clusterID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if existingCluster != nil {
		return nil, fmt.Errorf("cluster %q already exists in Omni", clusterID)
	}

	cpKubeletVersion, err := image.GetTag(cpInfo.kubeletConfig.TypedSpec().Image)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubelet version from node %q: %w", controlPlane, err)
	}

	versions, err := input.initVersions(cpInfo.hostname, cpInfo.talosVersion.Tag, cpKubeletVersion)
	if err != nil {
		return nil, err
	}

	if semver.MustParse(constants.MinTalosVersion).GTE(semver.MustParse(versions.TalosVersion)) {
		return nil, fmt.Errorf("minimum required version of talos is %s", constants.MinTalosVersion)
	}

	talosVersionMatrix, err := safe.ReaderGetByID[*omni.TalosVersion](ctx, omniState, omni.NewTalosVersion(resources.DefaultNamespace, versions.TalosVersion).Metadata().ID())
	if err != nil {
		return nil, fmt.Errorf("failed to read talos version: %w", err)
	}

	if talosVersionMatrix == nil {
		return nil, fmt.Errorf("talos version %q not found in omni", versions.TalosVersion)
	}

	if !slices.Contains(talosVersionMatrix.TypedSpec().Value.CompatibleKubernetesVersions, versions.KubernetesVersion) {
		return nil, fmt.Errorf("talos version %q is not compatible with kubernetes version %q", versions.TalosVersion, versions.KubernetesVersion)
	}

	return newContext(input, talosCli, imageFactoryClient, nodeInfoMap, joinOpts, omniState, clusterID, versions)
}

type nodeInfo struct {
	apiServerConfig         *k8s.APIServerConfig
	schedulerConfig         *k8s.SchedulerConfig
	controllerManagerConfig *k8s.ControllerManagerConfig
	talosVersion            *machine.VersionInfo
	kubeletConfig           *k8s.KubeletConfig
	machineConfig           *configres.MachineConfig
	schematic               *schematic.Schematic
	hostname                string
	machineID               string
	schematicID             string
	extensions              []*runtime.ExtensionStatus
	isControlPlane          bool
}

func collectNodeInfo(ctx context.Context, talosCli talosClientWrapper, node string) (nodeInfo, error) {
	var info nodeInfo

	isCP, err := talosCli.isControlPlane(ctx, node)
	if err != nil {
		return nodeInfo{}, fmt.Errorf("failed to determine if node %q is control plane: %w", node, err)
	}

	if isCP {
		info.isControlPlane = true

		apiServerConfig, cpErr := talosCli.getAPIServerConfig(ctx, node)
		if cpErr != nil {
			return nodeInfo{}, fmt.Errorf("failed to get API server config from node %q: %w", node, cpErr)
		}

		info.apiServerConfig = apiServerConfig

		schedulerConfig, cpErr := talosCli.getSchedulerConfig(ctx, node)
		if cpErr != nil {
			return nodeInfo{}, fmt.Errorf("failed to get scheduler config from node %q: %w", node, cpErr)
		}

		info.schedulerConfig = schedulerConfig

		controllerManagerConfig, cpErr := talosCli.getControllerManagerConfig(ctx, node)
		if cpErr != nil {
			return nodeInfo{}, fmt.Errorf("failed to get controller manager config from node %q: %w", node, cpErr)
		}

		info.controllerManagerConfig = controllerManagerConfig
	}

	version, err := talosCli.getTalosVersion(ctx, node)
	if err != nil {
		return nodeInfo{}, fmt.Errorf("failed to get talos version from node %q: %w", node, err)
	}

	info.talosVersion = version

	kubeletConfig, err := talosCli.getKubeletConfig(ctx, node)
	if err != nil {
		return nodeInfo{}, fmt.Errorf("failed to get kubelet config from node %q: %w", node, err)
	}

	info.kubeletConfig = kubeletConfig

	mc, err := talosCli.getMachineConfig(ctx, node)
	if err != nil {
		return nodeInfo{}, fmt.Errorf("failed to get machine config from node %q: %w", node, err)
	}

	info.machineConfig = mc

	hostnameStatus, err := talosCli.getHostnameStatus(ctx, node)
	if err != nil {
		return nodeInfo{}, fmt.Errorf("failed to get hostname from node %q: %w", node, err)
	}

	info.hostname = hostnameStatus.TypedSpec().Hostname

	uuid, err := talosCli.getUUID(ctx, node)
	if err != nil {
		return nodeInfo{}, fmt.Errorf("failed to get UUID from node %q: %w", node, err)
	}

	info.machineID = uuid

	extensions, err := talosCli.getExtensionStatuses(ctx, node)
	if err != nil {
		return nodeInfo{}, fmt.Errorf("failed to get extensions from node %q: %w", node, err)
	}

	info.extensions = extensions

	schematicID, imageSchematic, err := talosCli.getSchematic(extensions)
	if err != nil {
		return nodeInfo{}, fmt.Errorf("failed to get schematic from node %q: %w. make sure talos installer image is provided by omni's image factory", node, err)
	}

	info.schematicID = schematicID
	info.schematic = imageSchematic

	return info, nil
}

func newContext(input Input, talosCli talosClientWrapper, imageFactoryClient ImageFactoryClient, nodeInfoMap map[string]nodeInfo,
	joinOptions *siderolink.JoinOptions, omniState state.State, clusterID string, versions Versions,
) (*Context, error) {
	importContext := &Context{
		nodeInfoMap:         nodeInfoMap,
		joinOptions:         joinOptions,
		talosClient:         talosCli,
		imageFactoryClient:  imageFactoryClient,
		omniState:           omniState,
		input:               input,
		ensuredSchematicIDs: map[string]struct{}{},

		ClusterID: clusterID,
	}

	var controlPlanes, workers []string

	for node, info := range nodeInfoMap {
		if info.isControlPlane {
			controlPlanes = append(controlPlanes, node)
		} else {
			workers = append(workers, node)
		}
	}

	ics := omni.NewImportedClusterSecrets(resources.DefaultNamespace, clusterID)
	secretsBundle := secrets.NewBundleFromConfig(secrets.NewFixedClock(time.Now()), nodeInfoMap[controlPlanes[0]].machineConfig.Provider())

	bundleYaml, err := yaml.Marshal(secretsBundle)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal secrets bundle: %w", err)
	}

	ics.TypedSpec().Value.Data = string(bundleYaml)

	cluster := omni.NewCluster(resources.DefaultNamespace, clusterID)
	cluster.Metadata().Annotations().Set(omni.ClusterLocked, "")
	cluster.Metadata().Annotations().Set(omni.ClusterImportIsInProgress, "")
	cluster.TypedSpec().Value.TalosVersion, _ = strings.CutPrefix(versions.TalosVersion, "v")
	cluster.TypedSpec().Value.KubernetesVersion, _ = strings.CutPrefix(versions.KubernetesVersion, "v")

	controlPlaneMachineSet := omni.NewMachineSet(resources.DefaultNamespace, clusterID+"-control-planes")
	controlPlaneMachineSet.Metadata().Labels().Set(omni.LabelCluster, clusterID)
	controlPlaneMachineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	workerMachineSet := omni.NewMachineSet(resources.DefaultNamespace, clusterID+"-workers")
	workerMachineSet.Metadata().Labels().Set(omni.LabelCluster, clusterID)
	workerMachineSet.Metadata().Labels().Set(omni.LabelWorkerRole, "")

	configPatchesMap := make(map[resource.ID]*omni.ConfigPatch, len(input.Nodes))

	machineSetNodesMap := make(map[resource.ID]*omni.MachineSetNode, len(input.Nodes))
	for _, node := range input.Nodes {
		info := nodeInfoMap[node]

		if info.isControlPlane {
			machineSetNode := *omni.NewMachineSetNode(resources.DefaultNamespace, info.machineID, controlPlaneMachineSet)
			machineSetNode.Metadata().Labels().Set(omni.LabelCluster, clusterID)
			machineSetNode.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
			machineSetNode.Metadata().Labels().Set(omni.LabelMachineSet, controlPlaneMachineSet.Metadata().ID())
			machineSetNodesMap[machineSetNode.Metadata().ID()] = &machineSetNode
		} else {
			machineSetNode := *omni.NewMachineSetNode(resources.DefaultNamespace, info.machineID, workerMachineSet)
			machineSetNode.Metadata().Labels().Set(omni.LabelCluster, clusterID)
			machineSetNode.Metadata().Labels().Set(omni.LabelWorkerRole, "")
			machineSetNode.Metadata().Labels().Set(omni.LabelMachineSet, workerMachineSet.Metadata().ID())
			machineSetNodesMap[machineSetNode.Metadata().ID()] = &machineSetNode
		}

		genOutput, nodeErr := machineconfig.Generate(machineconfig.GenerateInput{
			ClusterID:                clusterID,
			MachineID:                info.machineID,
			InitialTalosVersion:      versions.InitialTalosVersion,
			InitialKubernetesVersion: versions.InitialKubernetesVersion,
			IsControlPlane:           info.isControlPlane,
			Secrets:                  secretsBundle,
		})
		if nodeErr != nil {
			return nil, nodeErr
		}

		machineConfigProvider := info.machineConfig.Provider()

		patches, nodeErr := configdiff.Patch(genOutput.Config, machineConfigProvider)
		if nodeErr != nil {
			return nil, fmt.Errorf("failed to create config patch for node %q: %w", node, nodeErr)
		}

		for i, patch := range patches {
			provider, ok := patch.(configpatcher.StrategicMergePatch)
			if !ok {
				return nil, fmt.Errorf("expected StrategicMergePatch, got %T", patch)
			}

			patchBytes, patchErr := provider.Provider().Bytes()
			if patchErr != nil {
				return nil, fmt.Errorf("failed to get bytes from patch for node %q: %w", node, patchErr)
			}

			sanitizedDiffBytes, patchErr := omni.SanitizeConfigPatch(patchBytes)
			if patchErr != nil {
				return nil, fmt.Errorf("failed to sanitize config patch for node %q: %w", node, patchErr)
			}

			configPatchId := fmt.Sprintf("%d-%s", (i+1)*10, info.machineID)
			configPatch := omni.NewConfigPatch(resources.DefaultNamespace, configPatchId)
			configPatch.Metadata().Labels().Set(omni.LabelCluster, clusterID)
			configPatch.Metadata().Labels().Set(omni.LabelClusterMachine, info.machineID)
			configPatch.Metadata().Annotations().Set(omni.ConfigPatchName, "User defined patch")
			configPatch.Metadata().Annotations().Set(omni.ConfigPatchDescription, "Config patch imported from existing Talos node")

			if patchErr = configPatch.TypedSpec().Value.SetUncompressedData(sanitizedDiffBytes); patchErr != nil {
				return nil, fmt.Errorf("failed to set data for config patch %q: %w", configPatchId, patchErr)
			}

			configPatchesMap[configPatchId] = configPatch
		}
	}

	importContext.importedClusterSecrets = ics
	importContext.cluster = cluster
	importContext.controlPlaneMachineSet = controlPlaneMachineSet
	importContext.workerMachineSet = workerMachineSet
	importContext.machineSetNodesMap = machineSetNodesMap
	importContext.configPatchesMap = configPatchesMap

	input.logf("importing cluster %q with %d nodes (%d control planes, %d workers)", clusterID, len(input.Nodes), len(controlPlanes), len(workers))

	return importContext, nil
}

// ErrValidation indicates a validation error.
var ErrValidation = errors.New("validation error")

// Run executes the import operation.
func (c *Context) Run(ctx context.Context) error {
	if err := c.validate(ctx); err != nil {
		if !c.input.Force {
			return fmt.Errorf("%w: %w", ErrValidation, err)
		}

		c.input.logf("failed to validate cluster status %q, but continuing since 'force' is requested: %v", c.ClusterID, err)
	}

	if err := c.checkClusterHealth(ctx); err != nil {
		return fmt.Errorf("cluster health check failed: %w", err)
	}

	if err := c.saveMachineConfigBackup(); err != nil {
		return fmt.Errorf("failed to save backup of machine config(s): %w", err)
	}

	if err := c.connectNodesToOmni(ctx); err != nil {
		return fmt.Errorf("failed to connect nodes to omni: %w", err)
	}

	if err := c.importClusterToOmni(ctx); err != nil {
		return fmt.Errorf("failed to import cluster: %w", err)
	}

	return nil
}

func (c *Context) importClusterToOmni(ctx context.Context) error {
	c.input.logf("creating cluster resources in omni...")

	if c.input.DryRun {
		c.input.logf(" > skipped in dry-run")
	}

	if err := c.createImportedClusterSecrets(ctx); err != nil {
		return err
	}

	if err := c.createCluster(ctx); err != nil {
		return err
	}

	if err := c.createMachineSetControlPlanes(ctx); err != nil {
		return err
	}

	if err := c.createMachineSetWorkers(ctx); err != nil {
		return err
	}

	if err := c.createMachineSetNodes(ctx); err != nil {
		return err
	}

	if err := c.createConfigPatches(ctx); err != nil {
		return err
	}

	c.input.logf("successfully created all cluster resources in omni")

	if c.input.DryRun {
		c.input.logf(" > skipped in dry-run")
	}

	return nil
}

//nolint:gocognit
func (c *Context) validate(ctx context.Context) error {
	c.input.logf("validating cluster status...")

	var multiErr *multierror.Error

	var controlPlanes []string

	for node, info := range c.nodeInfoMap {
		if info.isControlPlane {
			controlPlanes = append(controlPlanes, node)
		}
	}

	if len(controlPlanes) == 0 {
		multiErr = multierror.Append(multiErr, errors.New("no control plane nodes found"))
	}

	talosVersions := make(map[string]struct{})
	apiServerImages := make(map[string]struct{})
	controllerManagerImages := make(map[string]struct{})
	schedulerImages := make(map[string]struct{})
	kubeletImages := make(map[string]struct{})

	for _, info := range c.nodeInfoMap {
		talosVersions[info.talosVersion.Tag] = struct{}{}

		if info.isControlPlane {
			version, imageErr := image.GetTag(info.apiServerConfig.TypedSpec().Image)
			if imageErr != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("failed to parse API server image version %q: %w", info.apiServerConfig.TypedSpec().Image, imageErr))
			}

			apiServerImages[version] = struct{}{}

			version, imageErr = image.GetTag(info.controllerManagerConfig.TypedSpec().Image)
			if imageErr != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("failed to parse controller manager image version %q: %w", info.controllerManagerConfig.TypedSpec().Image, imageErr))
			}

			controllerManagerImages[version] = struct{}{}

			version, imageErr = image.GetTag(info.schedulerConfig.TypedSpec().Image)
			if imageErr != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("failed to parse scheduler image version %q: %w", info.schedulerConfig.TypedSpec().Image, imageErr))
			}

			schedulerImages[version] = struct{}{}
		}

		version, imageErr := image.GetTag(info.kubeletConfig.TypedSpec().Image)
		if imageErr != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to parse kubelet image version %q: %w", info.kubeletConfig.TypedSpec().Image, imageErr))
		}

		kubeletImages[version] = struct{}{}

		if imageErr = c.ensureSchematic(ctx, &info); imageErr != nil {
			multiErr = multierror.Append(multiErr, imageErr)
		}
	}

	joinSortedKeys := func(m map[string]struct{}) string {
		versions := xmaps.Keys(m)
		slices.Sort(versions)

		return strings.Join(versions, ", ")
	}

	if len(talosVersions) > 1 {
		multiErr = multierror.Append(multiErr, fmt.Errorf("multiple different Talos versions found: %v", joinSortedKeys(talosVersions)))
	}

	if len(apiServerImages) > 1 {
		multiErr = multierror.Append(multiErr, fmt.Errorf("multiple different API server images found: %v", joinSortedKeys(apiServerImages)))
	}

	if len(controllerManagerImages) > 1 {
		multiErr = multierror.Append(multiErr, fmt.Errorf("multiple different controller manager images found: %v", joinSortedKeys(controllerManagerImages)))
	}

	if len(schedulerImages) > 1 {
		multiErr = multierror.Append(multiErr, fmt.Errorf("multiple different scheduler images found: %v", joinSortedKeys(schedulerImages)))
	}

	if len(kubeletImages) > 1 {
		multiErr = multierror.Append(multiErr, fmt.Errorf("multiple different kubelet images found: %v", joinSortedKeys(kubeletImages)))
	}

	if multiErr != nil {
		multiErr.ErrorFormat = func(errors []error) string {
			var sb strings.Builder
			for _, e := range errors {
				sb.WriteString(fmt.Sprintf("\n - %v", e))
			}

			return sb.String()
		}
	}

	return multiErr.ErrorOrNil()
}

func (c *Context) ensureSchematic(ctx context.Context, info *nodeInfo) error {
	if info.schematicID != "" && info.schematic == nil {
		return fmt.Errorf("schematid ID %q found, but it can't be ensured for node %q in image factory", info.schematicID, info.hostname)
	}

	if info.schematicID == "" {
		return fmt.Errorf("schematic ID missing for node %q. talos was not installed using an image factory", info.hostname)
	}

	if _, ok := c.ensuredSchematicIDs[info.schematicID]; ok {
		return nil
	}

	ensuredSchematicID, schematicErr := c.imageFactoryClient.EnsureSchematic(ctx, *info.schematic)
	if schematicErr != nil {
		return fmt.Errorf("failed to ensure schematic %q for node %q in image factory: %w", info.schematicID, info.hostname, schematicErr)
	}

	if info.schematicID != ensuredSchematicID {
		return fmt.Errorf("schematic ID mismatch for node %q: expected %q, got %q", info.hostname, info.schematicID, ensuredSchematicID)
	}

	c.ensuredSchematicIDs[info.schematicID] = struct{}{}

	return nil
}

func (c *Context) checkClusterHealth(ctx context.Context) error {
	c.input.logf("checking cluster health...")

	if c.input.SkipHealthCheck {
		c.input.logf(" > skipped as per user request")

		return nil
	}

	var controlPlanes, workers []string

	for node, info := range c.nodeInfoMap {
		if info.isControlPlane {
			controlPlanes = append(controlPlanes, node)
		} else {
			workers = append(workers, node)
		}
	}

	return c.talosClient.checkClusterHealth(ctx, controlPlanes[0], controlPlanes, workers, 5*time.Minute, c.input.LogWriter)
}

func (c *Context) connectNodesToOmni(ctx context.Context) error {
	c.input.logf("connecting the nodes to omni...")

	if c.input.DryRun {
		c.input.logf(" > skipped in dry-run")

		return nil
	}

	docs, err := c.joinOptions.JoinConfigDocuments()
	if err != nil {
		return fmt.Errorf("failed to render join config: %w", err)
	}

	configContainer, err := container.New(docs...)
	if err != nil {
		return fmt.Errorf("failed to create config container: %w", err)
	}

	patchBytes, err := configContainer.Bytes()
	if err != nil {
		return fmt.Errorf("failed to get bytes from config container: %w", err)
	}

	patch, err := configpatcher.LoadPatch(patchBytes)
	if err != nil {
		return fmt.Errorf("failed to load config patch: %w", err)
	}

	for _, node := range c.input.Nodes {
		nodeCtx := talosclient.WithNode(ctx, node)
		info := c.nodeInfoMap[node]

		cfg, nodeErr := configpatcher.Apply(configpatcher.WithConfig(info.machineConfig.Provider()), []configpatcher.Patch{patch})
		if nodeErr != nil {
			return fmt.Errorf("failed to apply patch: %w", nodeErr)
		}

		patched, nodeErr := cfg.Bytes()
		if nodeErr != nil {
			return fmt.Errorf("failed to get bytes from patched config: %w", nodeErr)
		}

		_, nodeErr = c.talosClient.ApplyConfiguration(nodeCtx, &machine.ApplyConfigurationRequest{
			Data: patched,
			Mode: machine.ApplyConfigurationRequest_AUTO,
		})
		if nodeErr != nil {
			return fmt.Errorf("failed to apply configuration to node %q: %w", node, nodeErr)
		}
	}

	return nil
}

func (c *Context) saveMachineConfigBackup() error {
	c.input.logf("saving machine config backups to %q", c.input.getBackupOutput(c.ClusterID))

	if c.input.DryRun {
		c.input.logf(" > skipped in dry-run")

		return nil
	}

	zipFile, err := c.openArchive(c.input.getBackupOutput(c.ClusterID))
	if err != nil {
		return fmt.Errorf("could not create zip file: %w", err)
	}
	defer c.logClose(zipFile)

	zipWriter := zip.NewWriter(zipFile)
	defer c.logClose(zipWriter)

	for _, node := range c.input.Nodes {
		info := c.nodeInfoMap[node]

		fileName := fmt.Sprintf("%s-%s.yaml", info.hostname, node)

		writer, mcErr := zipWriter.Create(fileName)
		if mcErr != nil {
			return fmt.Errorf("could not create entry for %s: %w", fileName, mcErr)
		}

		machineConfigBytes, mcErr := info.machineConfig.Provider().Bytes()
		if mcErr != nil {
			return fmt.Errorf("could not encode machineConfig for %s: %w", fileName, mcErr)
		}

		_, mcErr = writer.Write(machineConfigBytes)
		if mcErr != nil {
			return fmt.Errorf("could not write machineConfig for %s: %w", fileName, mcErr)
		}
	}

	return nil
}

func (c *Context) Close() error {
	if c.talosClient.TalosClient != nil {
		if err := c.talosClient.Close(); err != nil {
			return fmt.Errorf("failed to close talos client: %w", err)
		}
	}

	return nil
}

func (c *Context) openArchive(file string) (*os.File, error) {
	if _, err := os.Stat(file); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		buf := bufio.NewReader(os.Stdin)

		fmt.Printf("%s already exists, overwrite? [y/N]: ", file)

		choice, err := buf.ReadString('\n')
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(strings.ToLower(choice)) != "y" {
			return nil, fmt.Errorf("operation was aborted")
		}
	}

	return os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
}

func (c *Context) createConfigPatches(ctx context.Context) error {
	c.input.logf("creating %d config patches", len(c.machineSetNodesMap))

	if c.input.DryRun {
		for _, patch := range c.configPatchesMap {
			c.input.logf("[dry-run] creating config patch %q", patch.Metadata().ID())
		}

		c.input.logf(" > skipped in dry-run")

		return nil
	}

	for _, cp := range c.configPatchesMap {
		if err := c.omniState.Modify(ctx, cp, noop); err != nil {
			return fmt.Errorf("failed to create config patch %q: %w", cp.Metadata().ID(), err)
		}
	}

	return nil
}

func (c *Context) createMachineSetNodes(ctx context.Context) error {
	c.input.logf("creating %d machine set nodes", len(c.machineSetNodesMap))

	if c.input.DryRun {
		for _, node := range c.machineSetNodesMap {
			c.input.logf("[dry-run] creating machine set node %q", node.Metadata().ID())
		}

		c.input.logf(" > skipped in dry-run")

		return nil
	}

	for _, msn := range c.machineSetNodesMap {
		if err := c.omniState.Modify(ctx, msn, noop); err != nil {
			return fmt.Errorf("failed to create machine set node %q: %w", msn.Metadata().ID(), err)
		}
	}

	return nil
}

func (c *Context) createMachineSetWorkers(ctx context.Context) error {
	c.input.logf("creating machine set for workers %q", c.workerMachineSet.Metadata().ID())

	if c.input.DryRun {
		c.input.logf(" > skipped in dry-run")

		return nil
	}

	if err := c.omniState.Modify(ctx, c.workerMachineSet, noop); err != nil {
		return fmt.Errorf("failed to create worker machine set: %w", err)
	}

	return nil
}

func (c *Context) createMachineSetControlPlanes(ctx context.Context) error {
	c.input.logf("creating machine set for control planes %q", c.controlPlaneMachineSet.Metadata().ID())

	if c.input.DryRun {
		c.input.logf(" > skipped in dry-run")

		return nil
	}

	if err := c.omniState.Modify(ctx, c.controlPlaneMachineSet, noop); err != nil {
		return fmt.Errorf("failed to create control plane machine set: %w", err)
	}

	return nil
}

func (c *Context) createCluster(ctx context.Context) error {
	c.input.logf("creating cluster %q", c.cluster.Metadata().ID())

	if c.input.DryRun {
		c.input.logf(" > skipped in dry-run")

		return nil
	}

	if err := c.omniState.Modify(ctx, c.cluster, noop); err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	return nil
}

func (c *Context) createImportedClusterSecrets(ctx context.Context) error {
	c.input.logf("creating imported cluster secrets %q", c.importedClusterSecrets.Metadata().ID())

	if c.input.DryRun {
		c.input.logf(" > skipped in dry-run")

		return nil
	}

	if err := c.omniState.Modify(ctx, c.importedClusterSecrets, noop); err != nil {
		return fmt.Errorf("failed to create imported cluster secrets: %w", err)
	}

	return nil
}

func (c *Context) logClose(closer io.Closer) {
	if err := closer.Close(); err != nil {
		c.input.logf("failed to close: %v", err)
	}
}

var noop = func(res resource.Resource) error { return nil }
