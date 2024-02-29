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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/internal/models"
)

type clusterResources struct {
	machineSetNodes             map[string][]*omni.MachineSetNode
	machineSetConfigPatches     map[string][]*omni.ConfigPatch
	clusterMachineConfigPatches map[string][]*omni.ConfigPatch
	clusterMachineInstallDisks  map[string]string

	cluster *omni.Cluster

	machineSets          []*omni.MachineSet
	clusterConfigPatches []*omni.ConfigPatch
}

// ExportTemplate exports the cluster configuration as a template.
func ExportTemplate(ctx context.Context, st state.State, clusterID string, writer io.Writer) (models.List, error) {
	resources, err := collectClusterResources(ctx, st, clusterID)
	if err != nil {
		return nil, err
	}

	clusterModel, err := transformClusterToModel(resources.cluster, resources.clusterConfigPatches)
	if err != nil {
		return nil, err
	}

	var controlPlaneMachineSetModel models.ControlPlane

	workerMachineSetModels := make([]models.Workers, 0, len(resources.machineSets))

	for _, machineSet := range resources.machineSets {
		machineSetModel, transformErr := transformMachineSetToModel(machineSet,
			resources.machineSetNodes[machineSet.Metadata().ID()],
			resources.machineSetConfigPatches[machineSet.Metadata().ID()])
		if transformErr != nil {
			return nil, transformErr
		}

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
				resources.clusterMachineConfigPatches[machineSetNode.Metadata().ID()],
				resources.clusterMachineInstallDisks[machineSetNode.Metadata().ID()],
			)
			if transformErr != nil {
				return nil, transformErr
			}

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
		workerMachineSetModel := workerMachineSetModel

		modelList = append(modelList, &workerMachineSetModel)
	}

	slices.SortFunc(machineModels, func(a, b models.Machine) int {
		return strings.Compare(string(a.Name), string(b.Name))
	})

	for _, machineModel := range machineModels {
		machineModel := machineModel

		modelList = append(modelList, &machineModel)
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

	if err := yaml.Unmarshal([]byte(configPatch.TypedSpec().Value.GetData()), &data); err != nil {
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

func transformMachineSetNodeToModel(machineSetNode *omni.MachineSetNode, patches []*omni.ConfigPatch, installDisk string) (models.Machine, error) {
	_, locked := machineSetNode.Metadata().Annotations().Get(omni.MachineLocked)

	patchModels, err := transformConfigPatchesToModels(patches)
	if err != nil {
		return models.Machine{}, err
	}

	return models.Machine{
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
	}, nil
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
		machineIDs         models.MachineIDList
		machineClassConfig *models.MachineClassConfig
	)

	if spec.GetMachineClass() != nil {
		machineClassConfig = &models.MachineClassConfig{
			Name: spec.GetMachineClass().GetName(),
			Size: models.Size{
				Value:          spec.GetMachineClass().GetMachineCount(),
				AllocationType: spec.GetMachineClass().GetAllocationType(),
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
			updateStrategyConfig.Type = pointer.To(models.UpdateStrategyType(spec.GetUpdateStrategy()))
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
			deleteStrategyConfig.Type = pointer.To(models.UpdateStrategyType(spec.GetDeleteStrategy()))
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
		MachineClass:   machineClassConfig,
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
			DiskEncryption:      spec.GetFeatures().GetDiskEncryption(),
			EnableWorkloadProxy: spec.GetFeatures().GetEnableWorkloadProxy(),
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

func collectClusterResources(ctx context.Context, st state.State, clusterID string) (clusterResources, error) {
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

	for iter := machineSetNodeList.Iterator(); iter.Next(); {
		machineSetNode := iter.Value()

		machineSetLabel, ok := machineSetNode.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			return clusterResources{}, fmt.Errorf("machine set node %q has no machine set label", machineSetNode.Metadata().ID())
		}

		machineSet, ok := machineSetIDToMachineSet[machineSetLabel]
		if !ok {
			return clusterResources{}, fmt.Errorf("unexpected machine set label %q", machineSetLabel)
		}

		// skip the node if its machine set picks machines from a machine class
		if machineSet.TypedSpec().Value.GetMachineClass() != nil {
			continue
		}

		machineSetNodes[machineSetLabel] = append(machineSetNodes[machineSetLabel], machineSetNode)
	}

	configPatchList, err := safe.StateListAll[*omni.ConfigPatch](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	if err != nil {
		return clusterResources{}, fmt.Errorf("error listing config patches of cluster %q: %w", clusterID, err)
	}

	clusterConfigPatches := make([]*omni.ConfigPatch, 0, configPatchList.Len())
	machineSetConfigPatches := make(map[string][]*omni.ConfigPatch, configPatchList.Len())
	clusterMachineConfigPatches := make(map[string][]*omni.ConfigPatch, configPatchList.Len())
	clusterMachineInstallDisks := make(map[string]string, configPatchList.Len())

	for iter := configPatchList.Iterator(); iter.Next(); {
		configPatch := iter.Value()

		// skip the patches with an owner, as they are not user-defined
		if configPatch.Metadata().Owner() != "" {
			continue
		}

		if clusterMachineLabel, ok := configPatch.Metadata().Labels().Get(omni.LabelClusterMachine); ok {
			installDisk := getInstallDiskFromConfigPatch(configPatch)
			if installDisk != "" {
				clusterMachineInstallDisks[clusterMachineLabel] = installDisk

				// this is an install disk patch, therefore it will be set as the machine's install disk on the Machine model,
				// not in patches, so we skip adding it to the set of regular patches
				continue
			}

			clusterMachineConfigPatches[clusterMachineLabel] = append(clusterMachineConfigPatches[clusterMachineLabel], configPatch)

			continue
		}

		if machineSetLabel, ok := configPatch.Metadata().Labels().Get(omni.LabelMachineSet); ok {
			machineSetConfigPatches[machineSetLabel] = append(machineSetConfigPatches[machineSetLabel], configPatch)

			continue
		}

		if _, ok := configPatch.Metadata().Labels().Get(omni.LabelCluster); ok {
			clusterConfigPatches = append(clusterConfigPatches, configPatch)
		}
	}

	return clusterResources{
		cluster:                     cluster,
		machineSets:                 listToSlice(machineSetList),
		machineSetNodes:             machineSetNodes,
		clusterConfigPatches:        clusterConfigPatches,
		machineSetConfigPatches:     machineSetConfigPatches,
		clusterMachineConfigPatches: clusterMachineConfigPatches,
		clusterMachineInstallDisks:  clusterMachineInstallDisks,
	}, nil
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

	if err := yaml.Unmarshal([]byte(configPatch.TypedSpec().Value.GetData()), &data); err != nil {
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

func listToSlice[T resource.Resource](list safe.List[T]) []T {
	result := make([]T, 0, list.Len())

	for iter := list.Iterator(); iter.Next(); {
		result = append(result, iter.Value())
	}

	return result
}

func listToMap[K comparable, T resource.Resource](list safe.List[T], keyFunc func(T) K) map[K]T {
	result := make(map[K]T, list.Len())

	for iter := list.Iterator(); iter.Next(); {
		value := iter.Value()

		result[keyFunc(value)] = value
	}

	return result
}
