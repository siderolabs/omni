// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/internal/models"
)

type clusterResources struct {
	patches    *layeredResources[*omni.ConfigPatch]
	extensions *layeredResources[*omni.ExtensionsConfiguration]

	machineSetNodes            map[string][]*omni.MachineSetNode
	kernelArgs                 map[string]*omni.KernelArgs
	clusterMachineInstallDisks map[string]string

	cluster *omni.Cluster

	machineSets []*omni.MachineSet
}

// ExportTemplate exports the cluster configuration as a template.
func ExportTemplate(ctx context.Context, st state.State, clusterID string, includeKernelArgs bool, writer io.Writer) (models.List, error) {
	resources, err := collectClusterResources(ctx, st, clusterID, includeKernelArgs)
	if err != nil {
		return nil, err
	}

	clusterModel, err := transformClusterToModel(resources.cluster, resources.patches.cluster)
	if err != nil {
		return nil, err
	}

	clusterModel.SystemExtensions = transformExtensions(resources.extensions.cluster)

	var controlPlaneMachineSetModel models.ControlPlane

	workerMachineSetModels := make([]models.Workers, 0, len(resources.machineSets))

	for _, machineSet := range resources.machineSets {
		machineSetModel, transformErr := transformMachineSetToModel(machineSet,
			resources.machineSetNodes[machineSet.Metadata().ID()],
			resources.patches.machineSet[machineSet.Metadata().ID()])
		if transformErr != nil {
			return nil, transformErr
		}

		machineSetModel.SystemExtensions = transformExtensions(resources.extensions.machineSet[machineSet.Metadata().ID()])

		if _, isControlPlane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole); isControlPlane {
			controlPlaneMachineSetModel = models.ControlPlane{MachineSet: machineSetModel}

			continue
		}

		workerMachineSetModels = append(workerMachineSetModels, models.Workers{MachineSet: machineSetModel})
	}

	machineModels := make([]models.Machine, 0, len(resources.machineSetNodes))

	for _, machineSetNodes := range resources.machineSetNodes {
		for _, machineSetNode := range machineSetNodes {
			machineModel, transformErr := transformMachineSetNodeToModel(machineSetNode,
				resources.kernelArgs[machineSetNode.Metadata().ID()],
				resources.patches.clusterMachine[machineSetNode.Metadata().ID()],
				resources.clusterMachineInstallDisks[machineSetNode.Metadata().ID()],
				includeKernelArgs,
			)
			if transformErr != nil {
				return nil, transformErr
			}

			machineModel.SystemExtensions = transformExtensions(resources.extensions.clusterMachine[machineSetNode.Metadata().ID()])

			machineModels = append(machineModels, machineModel)
		}
	}

	modelList := buildModelList(clusterModel, controlPlaneMachineSetModel, workerMachineSetModels, machineModels)
	if err = modelList.Validate(); err != nil {
		return nil, fmt.Errorf("error validating models: %w", err)
	}

	return modelList, writeYAML(writer, modelList)
}

func buildModelList(clusterModel models.Cluster, controlPlaneMachineSetModel models.ControlPlane,
	workerMachineSetModels []models.Workers, machineModels []models.Machine,
) models.List {
	modelList := models.List{
		&clusterModel,
		&controlPlaneMachineSetModel,
	}

	slices.SortFunc(workerMachineSetModels, func(a, b models.Workers) int {
		return strings.Compare(a.Name, b.Name)
	})

	for _, workerMachineSetModel := range workerMachineSetModels {
		modelList = append(modelList, &workerMachineSetModel) //nolint:exportloopref
	}

	slices.SortFunc(machineModels, func(a, b models.Machine) int {
		return strings.Compare(string(a.Name), string(b.Name))
	})

	for _, machineModel := range machineModels {
		modelList = append(modelList, &machineModel) //nolint:exportloopref
	}

	return modelList
}

func writeYAML(writer io.Writer, modelList models.List) (err error) {
	encoder := yaml.NewEncoder(writer)

	defer func() {
		err = errors.Join(err, encoder.Close())
	}()

	encoder.SetIndent(2)

	for _, model := range modelList {
		if err = encoder.Encode(model); err != nil {
			return fmt.Errorf("error encoding model: %w", err)
		}
	}

	return nil
}

func transformConfigPatchToModel(configPatch *omni.ConfigPatch) (models.Patch, error) {
	var data map[string]any

	buffer, err := configPatch.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return models.Patch{}, fmt.Errorf("failed to get patch data for patch %q: %w", configPatch.Metadata().ID(), err)
	}

	defer buffer.Free()

	patchData := buffer.Data()

	if err = yaml.Unmarshal(patchData, &data); err != nil {
		return models.Patch{}, fmt.Errorf("failed to unmarshal patch %q: %w", configPatch.Metadata().ID(), err)
	}

	return models.Patch{
		Descriptors: getUserDescriptors(configPatch),
		IDOverride:  configPatch.Metadata().ID(),
		Inline:      data,
	}, nil
}

func transformConfigPatchesToModels(configPatches []*omni.ConfigPatch) (models.PatchList, error) {
	patchModels := make(models.PatchList, 0, len(configPatches))

	for _, configPatch := range configPatches {
		patchModel, err := transformConfigPatchToModel(configPatch)
		if err != nil {
			return nil, err
		}

		patchModels = append(patchModels, patchModel)
	}

	slices.SortFunc(patchModels, func(a, b models.Patch) int {
		return strings.Compare(a.IDOverride, b.IDOverride)
	})

	return patchModels, nil
}

func transformMachineSetNodeToModel(machineSetNode *omni.MachineSetNode, kernelArgsRes *omni.KernelArgs,
	patches []*omni.ConfigPatch, installDisk string, includeKernelArgs bool,
) (models.Machine, error) {
	_, locked := machineSetNode.Metadata().Annotations().Get(omni.MachineLocked)

	patchModels, err := transformConfigPatchesToModels(patches)
	if err != nil {
		return models.Machine{}, err
	}

	machine := models.Machine{
		Meta: models.Meta{
			Kind: models.KindMachine,
		},
		Name:        models.MachineID(machineSetNode.Metadata().ID()),
		Descriptors: getUserDescriptors(machineSetNode),
		Locked:      locked,
		Install: models.MachineInstall{
			Disk: installDisk,
		},
		Patches: patchModels,
	}

	if !includeKernelArgs {
		return machine, nil
	}

	var kernelArgs []string

	if kernelArgsRes != nil {
		kernelArgs = kernelArgsRes.TypedSpec().Value.Args
	}

	machine.KernelArgs.Set(kernelArgs)

	return machine, nil
}

func transformMachineSetToModel(machineSet *omni.MachineSet, nodes []*omni.MachineSetNode, patches []*omni.ConfigPatch) (models.MachineSet, error) {
	cluster, _ := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	_, isControlPlane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole)
	_, isWorker := machineSet.Metadata().Labels().Get(omni.LabelWorkerRole)

	if !isControlPlane && !isWorker {
		return models.MachineSet{}, fmt.Errorf("machine set %q has no role label", machineSet.Metadata().ID())
	}

	spec := machineSet.TypedSpec().Value

	var bootstrapSpec *models.BootstrapSpec

	if spec.GetBootstrapSpec() != nil {
		bootstrapSpec = &models.BootstrapSpec{
			ClusterUUID: spec.GetBootstrapSpec().GetClusterUuid(),
			Snapshot:    spec.GetBootstrapSpec().GetSnapshot(),
		}
	}

	var (
		machineIDs        models.MachineIDList
		machineAllocation *models.MachineClassConfig
	)

	allocationConfig := omni.GetMachineAllocation(machineSet)

	if allocationConfig != nil {
		machineAllocation = &models.MachineClassConfig{
			Name: allocationConfig.Name,
			Size: models.Size{
				Value:          allocationConfig.GetMachineCount(),
				AllocationType: allocationConfig.GetAllocationType(),
			},
		}
	} else {
		machineIDs = xslices.Map(nodes, func(node *omni.MachineSetNode) models.MachineID {
			return models.MachineID(node.Metadata().ID())
		})

		slices.Sort(machineIDs)
	}

	var updateStrategyConfig *models.UpdateStrategyConfig

	if spec.GetUpdateStrategyConfig() != nil {
		updateStrategyConfig = &models.UpdateStrategyConfig{}

		if spec.GetUpdateStrategy() != specs.MachineSetSpec_Rolling { // Rolling is the default for update, so set the strategy type only when it is not Rolling.
			updateStrategyConfig.Type = new(models.UpdateStrategyType(spec.GetUpdateStrategy()))
		}

		if spec.GetUpdateStrategyConfig().GetRolling() != nil {
			updateStrategyConfig.Rolling = &models.RollingUpdateStrategyConfig{
				MaxParallelism: spec.GetUpdateStrategyConfig().GetRolling().GetMaxParallelism(),
			}
		}
	}

	var deleteStrategyConfig *models.UpdateStrategyConfig

	if spec.GetDeleteStrategyConfig() != nil {
		deleteStrategyConfig = &models.UpdateStrategyConfig{}

		if spec.GetDeleteStrategy() != specs.MachineSetSpec_Unset { // Unset is the default for delete, so set the strategy type only when it is not Unset.
			deleteStrategyConfig.Type = new(models.UpdateStrategyType(spec.GetDeleteStrategy()))
		}

		if spec.GetDeleteStrategyConfig().GetRolling() != nil {
			deleteStrategyConfig.Rolling = &models.RollingUpdateStrategyConfig{
				MaxParallelism: spec.GetDeleteStrategyConfig().GetRolling().GetMaxParallelism(),
			}
		}
	}

	kind := models.KindControlPlane
	if isWorker {
		kind = models.KindWorkers
	}

	name := ""
	if isWorker && machineSet.Metadata().ID() != omni.WorkersResourceID(cluster) {
		name = strings.TrimPrefix(machineSet.Metadata().ID(), cluster+"-")
	}

	patchModels, err := transformConfigPatchesToModels(patches)
	if err != nil {
		return models.MachineSet{}, err
	}

	return models.MachineSet{
		Meta: models.Meta{
			Kind: kind,
		},
		Name:           name,
		Descriptors:    getUserDescriptors(machineSet),
		BootstrapSpec:  bootstrapSpec,
		Machines:       machineIDs,
		MachineClass:   machineAllocation,
		Patches:        patchModels,
		UpdateStrategy: updateStrategyConfig,
		DeleteStrategy: deleteStrategyConfig,
	}, nil
}

func transformClusterToModel(cluster *omni.Cluster, patches []*omni.ConfigPatch) (models.Cluster, error) {
	spec := cluster.TypedSpec().Value
	backupIntervalDuration := time.Duration(0)

	if spec.GetBackupConfiguration().GetInterval() != nil {
		backupIntervalDuration = spec.GetBackupConfiguration().GetInterval().AsDuration()
	}

	patchModels, err := transformConfigPatchesToModels(patches)
	if err != nil {
		return models.Cluster{}, err
	}

	return models.Cluster{
		Meta: models.Meta{
			Kind: models.KindCluster,
		},
		Name:        cluster.Metadata().ID(),
		Descriptors: getUserDescriptors(cluster),
		Kubernetes: models.KubernetesCluster{
			Version: "v" + spec.GetKubernetesVersion(),
		},
		Talos: models.TalosCluster{
			Version: "v" + spec.GetTalosVersion(),
		},
		Features: models.Features{
			DiskEncryption:              spec.GetFeatures().GetDiskEncryption(),
			EnableWorkloadProxy:         spec.GetFeatures().GetEnableWorkloadProxy(),
			UseEmbeddedDiscoveryService: spec.GetFeatures().GetUseEmbeddedDiscoveryService(),
			BackupConfiguration: models.BackupConfiguration{
				Interval: backupIntervalDuration,
			},
		},
		Patches: patchModels,
	}, nil
}

func getUserDescriptors(res resource.Resource) models.Descriptors {
	rawLabels := res.Metadata().Labels().Raw()
	labels := make(map[string]string, len(rawLabels))

	for k, v := range res.Metadata().Labels().Raw() {
		if !strings.HasPrefix(k, omni.SystemLabelPrefix) {
			labels[k] = v
		}
	}

	rawAnnotations := res.Metadata().Annotations().Raw()
	annotations := make(map[string]string, len(rawAnnotations))

	for k, v := range res.Metadata().Annotations().Raw() {
		if !strings.HasPrefix(k, omni.SystemLabelPrefix) {
			annotations[k] = v
		}
	}

	return models.Descriptors{
		Labels:      labels,
		Annotations: annotations,
	}
}

func collectClusterResources(ctx context.Context, st state.State, clusterID string, includeKernelArgs bool) (clusterResources, error) {
	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterID)
	if err != nil {
		return clusterResources{}, fmt.Errorf("error getting cluster %q: %w", clusterID, err)
	}

	machineSetList, err := safe.StateListAll[*omni.MachineSet](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	if err != nil {
		return clusterResources{}, fmt.Errorf("error listing machine sets of cluster %q: %w", clusterID, err)
	}

	machineSetIDToMachineSet := listToMap(machineSetList, func(machineSet *omni.MachineSet) resource.ID {
		return machineSet.Metadata().ID()
	})

	machineSetNodeList, err := safe.StateListAll[*omni.MachineSetNode](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	if err != nil {
		return clusterResources{}, fmt.Errorf("error listing machine set nodes of cluster %q: %w", clusterID, err)
	}

	machineSetNodes := make(map[string][]*omni.MachineSetNode, machineSetNodeList.Len())
	machineKernelArgs := make(map[string]*omni.KernelArgs, machineSetNodeList.Len())

	for machineSetNode := range machineSetNodeList.All() {
		machineSetLabel, ok := machineSetNode.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			return clusterResources{}, fmt.Errorf("machine set node %q has no machine set label", machineSetNode.Metadata().ID())
		}

		machineSet, ok := machineSetIDToMachineSet[machineSetLabel]
		if !ok {
			return clusterResources{}, fmt.Errorf("unexpected machine set label %q", machineSetLabel)
		}

		allocationConfig := omni.GetMachineAllocation(machineSet)

		// skip the node if its machine set picks machines automatically
		if allocationConfig != nil {
			continue
		}

		machineSetNodes[machineSetLabel] = append(machineSetNodes[machineSetLabel], machineSetNode)

		if !includeKernelArgs {
			continue
		}

		var kernelArgs *omni.KernelArgs

		if kernelArgs, err = safe.StateGetByID[*omni.KernelArgs](ctx, st, machineSetNode.Metadata().ID()); err != nil && !state.IsNotFoundError(err) {
			return clusterResources{}, fmt.Errorf("error getting kernel args for machine set node %q: %w", machineSetNode.Metadata().ID(), err)
		}

		if kernelArgs != nil {
			machineKernelArgs[machineSetNode.Metadata().ID()] = kernelArgs
		}
	}

	clusterMachineInstallDisks := map[string]string{}

	patches, err := collectResourceLayers[*omni.ConfigPatch](ctx, st, clusterID, func(item *omni.ConfigPatch) bool {
		if clusterMachineLabel, ok := item.Metadata().Labels().Get(omni.LabelClusterMachine); ok {
			installDisk := getInstallDiskFromConfigPatch(item)
			if installDisk != "" {
				clusterMachineInstallDisks[clusterMachineLabel] = installDisk

				// this is an install disk patch, therefore it will be set as the machine's install disk on the Machine model,
				// not in patches, so we skip adding it to the set of regular patches
				return true
			}
		}

		return false
	})
	if err != nil {
		return clusterResources{}, err
	}

	extensions, err := collectResourceLayers[*omni.ExtensionsConfiguration](ctx, st, clusterID, nil)
	if err != nil {
		return clusterResources{}, err
	}

	return clusterResources{
		cluster:                    cluster,
		machineSets:                slices.AppendSeq(make([]*omni.MachineSet, 0, machineSetList.Len()), machineSetList.All()),
		machineSetNodes:            machineSetNodes,
		kernelArgs:                 machineKernelArgs,
		patches:                    patches,
		extensions:                 extensions,
		clusterMachineInstallDisks: clusterMachineInstallDisks,
	}, nil
}

func transformExtensions(extensions []*omni.ExtensionsConfiguration) models.SystemExtensions {
	if len(extensions) == 0 {
		return models.SystemExtensions{}
	}

	return models.SystemExtensions{SystemExtensions: extensions[0].TypedSpec().Value.Extensions}
}

type layeredResources[T meta.ResourceWithRD] struct {
	machineSet     map[string][]T
	clusterMachine map[string][]T
	cluster        []T
}

func collectResourceLayers[T meta.ResourceWithRD](ctx context.Context, st state.State, clusterID string, callback func(res T) bool) (*layeredResources[T], error) {
	resources, err := safe.StateListAll[T](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	if err != nil {
		return nil, fmt.Errorf("error listing config patches of cluster %q: %w", clusterID, err)
	}

	res := &layeredResources[T]{
		cluster:        make([]T, 0, resources.Len()),
		machineSet:     make(map[string][]T, resources.Len()),
		clusterMachine: make(map[string][]T, resources.Len()),
	}

	resources.ForEach(func(item T) {
		_, programmaticallyCreatedMachineSetNode := item.Metadata().Labels().Get(omni.LabelManagedByMachineSetNodeController)

		// skip the resources with an owner, as they are not user-defined
		if item.Metadata().Owner() != "" || programmaticallyCreatedMachineSetNode {
			return
		}

		if callback != nil && callback(item) {
			return
		}

		if clusterMachineLabel, ok := item.Metadata().Labels().Get(omni.LabelClusterMachine); ok {
			res.clusterMachine[clusterMachineLabel] = append(res.clusterMachine[clusterMachineLabel], item)

			return
		}

		if machineSetLabel, ok := item.Metadata().Labels().Get(omni.LabelMachineSet); ok {
			res.machineSet[machineSetLabel] = append(res.machineSet[machineSetLabel], item)

			return
		}

		res.cluster = append(res.cluster, item)
	})

	return res, nil
}

func getInstallDiskFromConfigPatch(configPatch *omni.ConfigPatch) string {
	clusterMachine, ok := configPatch.Metadata().Labels().Get(omni.LabelClusterMachine)
	if !ok {
		return ""
	}

	expectedID := fmt.Sprintf("%03d-cm-%s-install-disk", constants.PatchWeightInstallDisk, clusterMachine)
	if configPatch.Metadata().ID() != expectedID {
		return ""
	}

	var data map[string]any

	buffer, err := configPatch.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return "" // ignore the error, as it will be caught by the validation later
	}

	defer buffer.Free()

	patchData := buffer.Data()

	if err = yaml.Unmarshal(patchData, &data); err != nil {
		return "" // ignore the error, as it will be caught by the validation later
	}

	machine, ok := data["machine"].(map[string]any)
	if !ok {
		return ""
	}

	install, ok := machine["install"].(map[string]any)
	if !ok {
		return ""
	}

	if slices.ContainsFunc([]map[string]any{data, machine, install}, func(m map[string]any) bool { return len(m) != 1 }) {
		return "" // there is some data in the patch other than the install disk, so it cannot be converted into install.disk block in the Machine model
	}

	disk, ok := install["disk"].(string)
	if !ok {
		return ""
	}

	return disk
}

func listToMap[K comparable, T resource.Resource](list safe.List[T], keyFunc func(T) K) map[K]T {
	result := make(map[K]T, list.Len())

	for value := range list.All() {
		result[keyFunc(value)] = value
	}

	return result
}
