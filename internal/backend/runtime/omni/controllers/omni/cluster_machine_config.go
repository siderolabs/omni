// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config"
	documentconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	machineapi "github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/machineconfig"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterMachineConfigControllerName is the name of the ClusterMachineConfigController.
const ClusterMachineConfigControllerName = "ClusterMachineConfigController"

// ClusterMachineConfigController manages machine configurations for each ClusterMachine.
//
// ClusterMachineConfigController generates machine configuration for each created machine.
type ClusterMachineConfigController = qtransform.QController[*omni.ClusterMachine, *omni.ClusterMachineConfig]

// NewClusterMachineConfigController initializes ClusterMachineConfigController.
func NewClusterMachineConfigController(imageFactoryHost string, registryMirrors []string) *ClusterMachineConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachine, *omni.ClusterMachineConfig]{
			Name: ClusterMachineConfigControllerName,
			MapMetadataFunc: func(clusterMachine *omni.ClusterMachine) *omni.ClusterMachineConfig {
				return omni.NewClusterMachineConfig(resources.DefaultNamespace, clusterMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineConfig *omni.ClusterMachineConfig) *omni.ClusterMachine {
				return omni.NewClusterMachine(resources.DefaultNamespace, machineConfig.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, clusterMachine *omni.ClusterMachine, machineConfig *omni.ClusterMachineConfig) error {
				return reconcileClusterMachineConfig(ctx, r, logger, clusterMachine, machineConfig, registryMirrors, imageFactoryHost)
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfigPatches](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineConfigGenOptions](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.Cluster](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterSecrets](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterConfigVersion](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.LoadBalancerConfig](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*siderolink.MachineJoinConfig](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithConcurrency(2),
	)
}

//nolint:gocognit,cyclop,gocyclo,maintidx
func reconcileClusterMachineConfig(
	ctx context.Context,
	r controller.Reader,
	logger *zap.Logger,
	clusterMachine *omni.ClusterMachine,
	machineConfig *omni.ClusterMachineConfig,
	registryMirrors []string,
	imageFactoryHost string,
) error {
	clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("missing cluster label on %s", clusterMachine.Metadata().ID())
	}

	cluster, err := safe.ReaderGet[*omni.Cluster](ctx, r, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	machineSetNode, err := safe.ReaderGet[*omni.MachineSetNode](ctx, r,
		resource.NewMetadata(resources.DefaultNamespace, omni.MachineSetNodeType, clusterMachine.Metadata().ID(), resource.VersionUndefined))
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	if machineSetNode.Metadata().Phase() == resource.PhaseTearingDown {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("machine is being torn down"))
	}

	if clusterLabel, ok := machineSetNode.Metadata().Labels().Get(omni.LabelCluster); !ok || clusterLabel != clusterName {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster label on %s doesn't match", machineSetNode.Metadata().ID())
	}

	secrets, err := safe.ReaderGet[*omni.ClusterSecrets](ctx, r, omni.NewClusterSecrets(resources.DefaultNamespace, clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	loadBalancerConfig, err := safe.ReaderGet[*omni.LoadBalancerConfig](ctx, r, omni.NewLoadBalancerConfig(resources.DefaultNamespace, clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	clusterConfigVersion, err := safe.ReaderGet[*omni.ClusterConfigVersion](ctx, r, omni.NewClusterConfigVersion(resources.DefaultNamespace, clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	clusterMachineConfigPatches, err := safe.ReaderGet[*omni.ClusterMachineConfigPatches](
		ctx,
		r,
		omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	machineConfigGenOptions, err := safe.ReaderGet[*omni.MachineConfigGenOptions](
		ctx,
		r,
		omni.NewMachineConfigGenOptions(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	machineJoinConfig, err := safe.ReaderGetByID[*siderolink.MachineJoinConfig](ctx, r, clusterMachine.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	inputs := []resource.Resource{
		secrets,
		clusterMachine,
		loadBalancerConfig,
		cluster,
		clusterMachineConfigPatches,
		machineConfigGenOptions,
		machineJoinConfig,
	}

	if !helpers.UpdateInputsVersions(machineConfig, inputs...) {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("config inputs not changed"))
	}

	helpers.CopyLabels(clusterMachine, machineConfig, omni.LabelMachineSet, omni.LabelCluster, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

	// TODO: temporary transition code, remove in the future
	if clusterMachine.TypedSpec().Value.KubernetesVersion == "" {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("kubernetes version is not set yet"))
	}

	installImage := machineConfigGenOptions.TypedSpec().Value.InstallImage
	if installImage == nil {
		logger.Error("install image is not set, skip reconcile")

		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("install image is not set yet"))
	}

	// skip if the machine schematic information is not yet detected
	if !installImage.SchematicInitialized {
		logger.Error("machine schematic is not set, skip reconcile")

		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("machine schematic is not set detected"))
	}

	if installImage.SecurityState == nil {
		logger.Error("secure boot status is not detected, skip reconcile")

		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("secure boot status for machine %q is not yet set", machineConfigGenOptions.Metadata().ID())
	}

	helper := clusterMachineConfigControllerHelper{
		imageFactoryHost: imageFactoryHost,
	}

	configGenOptions := make([]generate.Option, 0, len(registryMirrors))

	for _, registryMirror := range registryMirrors {
		hostname, endpoint, ok := strings.Cut(registryMirror, "=")
		if !ok {
			return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		configGenOptions = append(configGenOptions, generate.WithRegistryMirror(hostname, endpoint))
	}

	data, err := helper.generateConfig(clusterMachine, clusterMachineConfigPatches, secrets, loadBalancerConfig,
		cluster, clusterConfigVersion, machineConfigGenOptions, configGenOptions, machineJoinConfig)
	if err != nil {
		machineConfig.TypedSpec().Value.GenerationError = err.Error()

		return nil //nolint:nilerr
	}

	skipUpdate := false

	// skip comparing existing config to generated config if existing config has its comments stripped to avoid unnecessary decompression/unmarshalling
	if !machineConfig.TypedSpec().Value.WithoutComments {
		if skipUpdate, err = helper.configsEqual(machineConfig, data); err != nil {
			return err
		}
	}

	// skip updating the config if the existing config is effectively equal to the generated one
	if !skipUpdate {
		if err = machineConfig.TypedSpec().Value.SetUncompressedData(data); err != nil {
			return err
		}

		machineConfig.TypedSpec().Value.WithoutComments = true
	}

	machineConfig.TypedSpec().Value.ClusterMachineVersion = clusterMachine.Metadata().Version().String()
	machineConfig.TypedSpec().Value.GenerationError = ""

	return nil
}

type clusterMachineConfigControllerHelper struct {
	imageFactoryHost string
}

func (helper clusterMachineConfigControllerHelper) configsEqual(old *omni.ClusterMachineConfig, data []byte) (bool, error) {
	oldConfig, err := old.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return false, err
	}

	defer oldConfig.Free()

	oldConfigData := oldConfig.Data()
	if len(oldConfigData) == 0 {
		return false, nil
	}

	oldConf, err := configloader.NewFromBytes(oldConfigData)
	if err != nil {
		return false, err
	}

	oldBytes, err := oldConf.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return false, err
	}

	return bytes.Equal(oldBytes, data), nil
}

func (helper clusterMachineConfigControllerHelper) generateConfig(clusterMachine *omni.ClusterMachine, clusterMachineConfigPatches *omni.ClusterMachineConfigPatches, secrets *omni.ClusterSecrets,
	loadbalancer *omni.LoadBalancerConfig, cluster *omni.Cluster, clusterConfigVersion *omni.ClusterConfigVersion, configGenOptions *omni.MachineConfigGenOptions, extraGenOptions []generate.Option,
	machineJoinConfig *siderolink.MachineJoinConfig,
) ([]byte, error) {
	clusterName := cluster.Metadata().ID()

	// this is the version of Talos at the moment the cluster got created
	//
	// [NOTE]: this should be kept a constant for the lifetime of the cluster,
	// as it dictates the Talos machinery config generation defaults.
	// If this value is changed, it will cause the machine configuration to be regenerated
	// with new version contract (defaults), and might cause unexpected issues.
	//
	// The desired version of Talos for this machine (not for config generation), but for the
	// e.g. install image is stored in MachineConfigGenOptions.
	initialTalosVersion := clusterConfigVersion.TypedSpec().Value.Version

	// [NOTE]: this is the version of Kubernetes of the cluster at the moment ClusterMachine was created.
	// (i.e., the moment the Machine joined this cluster).
	// Kubernetes upgrades are handled as config patches to the cluster machines.
	initialKubernetesVersion := clusterMachine.TypedSpec().Value.KubernetesVersion

	if initialTalosVersion == "" {
		return nil, fmt.Errorf("talos version is not set on the resource %s", clusterConfigVersion.Metadata())
	}

	installImage, err := installimage.Build(helper.imageFactoryHost, configGenOptions.Metadata().ID(), configGenOptions.TypedSpec().Value.InstallImage)
	if err != nil {
		return nil, err
	}

	secretBundle, err := omni.ToSecretsBundle(secrets)
	if err != nil {
		return nil, err
	}

	machineType := machineapi.TypeWorker

	if _, ok := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
		machineType = machineapi.TypeControlPlane
	}

	genOutput, err := machineconfig.Generate(machineconfig.GenerateInput{
		ClusterID:                clusterName,
		MachineID:                clusterMachine.Metadata().ID(),
		InitialTalosVersion:      initialTalosVersion,
		InitialKubernetesVersion: initialKubernetesVersion,
		ExtraGenOptions:          extraGenOptions,
		IsControlPlane:           machineType == machineapi.TypeControlPlane,
		SiderolinkEndpoint:       loadbalancer.TypedSpec().Value.SiderolinkEndpoint,
		InstallDisk:              configGenOptions.TypedSpec().Value.InstallDisk,
		InstallImage:             installImage,
		Secrets:                  secretBundle,
	})
	if err != nil {
		return nil, err
	}

	cfg := genOutput.Config

	patchList, err := clusterMachineConfigPatches.TypedSpec().Value.GetUncompressedPatches()
	if err != nil {
		return nil, err
	}

	if _, preserveApidCheckExtKeyUsage := clusterMachine.Metadata().Annotations().Get(omni.PreserveApidCheckExtKeyUsage); preserveApidCheckExtKeyUsage {
		patchList = slices.Insert(patchList, 0, `machine:
  features:
    apidCheckExtKeyUsage: true
`,
		)
	}

	if _, preserveDiskQuotaSupport := clusterMachine.Metadata().Annotations().Get(omni.PreserveDiskQuotaSupport); preserveDiskQuotaSupport {
		patchList = slices.Insert(patchList, 0, `machine:
  features:
    diskQuotaSupport: true
`,
		)
	}

	// [TODO]: this should check current (minimum) version of the cluster (or current Talos version of the machine)
	if quirks.New(initialTalosVersion).SupportsMultidoc() {
		patchList = append(patchList, machineJoinConfig.TypedSpec().Value.Config.Config)
	}

	patches, err := configpatcher.LoadPatches(patchList)
	if err != nil {
		return nil, err
	}

	patched, err := configpatcher.Apply(configpatcher.WithConfig(cfg), patches)
	if err != nil {
		return nil, err
	}

	patchedConfig, err := patched.Config()
	if err != nil {
		return nil, fmt.Errorf("failed to get patched config: %w", err)
	}

	strippedConfig, err := stripTalosAPIAccessOSAdminRole(patchedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build talos api access feature allowed roles patch: %w", err)
	}

	// todo: for Talos 1.12 and above, if .machine.install.kernelArgs is empty,
	// we will add the new document to tell Talos to always use the kernel args in the UKI, in other words, make it "act like UKI".
	// this will allow that machine to support customizing extra kernel args.

	return strippedConfig.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
}

// stripTalosAPIAccessOSAdminRole ensures that the OS admin role is never included in the allowed roles of the
// Kubernetes Talos API Access feature configuration.
//
// Config patches are already validated to not contain it, this is merely an additional safety measure.
func stripTalosAPIAccessOSAdminRole(cfg config.Provider) (config.Provider, error) {
	if cfg.Machine() == nil {
		return cfg, nil
	}

	allowedRoles := cfg.Machine().Features().KubernetesTalosAPIAccess().AllowedRoles()
	if len(allowedRoles) == 0 {
		return cfg, nil
	}

	filteredAllowedRoles := make([]string, 0, len(allowedRoles))

	osAdminRole := string(talosrole.Admin)

	for _, role := range allowedRoles {
		if role != osAdminRole {
			filteredAllowedRoles = append(filteredAllowedRoles, role)
		}
	}

	// nothing is filtered out, short-circuit
	if len(filteredAllowedRoles) == len(allowedRoles) {
		return cfg, nil
	}

	configDocs := cfg.Documents()
	updatedDocs := make([]documentconfig.Document, 0, len(configDocs))

	for _, document := range configDocs {
		if document.APIVersion() == "" && document.Kind() == v1alpha1.Version {
			v1alpha1Config := cfg.RawV1Alpha1() // this ensures that we get a writeable copy of v1alpha1 config

			v1alpha1Config.MachineConfig.MachineFeatures.KubernetesTalosAPIAccessConfig.AccessAllowedRoles = filteredAllowedRoles

			updatedDocs = append(updatedDocs, v1alpha1Config)

			continue
		}

		updatedDocs = append(updatedDocs, document)
	}

	return container.New(updatedDocs...)
}
