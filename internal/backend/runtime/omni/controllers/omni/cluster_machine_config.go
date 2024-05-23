// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config"
	documentconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	machineapi "github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	appconfig "github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

const clusterMachineConfigControllerName = "ClusterMachineConfigController"

// ClusterMachineConfigController manages machine configurations for each ClusterMachine.
//
// ClusterMachineConfigController generates machine configuration for each created machine.
type ClusterMachineConfigController = qtransform.QController[*omni.ClusterMachine, *omni.ClusterMachineConfig]

// NewClusterMachineConfigController initializes ClusterMachineConfigController.
func NewClusterMachineConfigController(defaultGenOptions []generate.Option) *ClusterMachineConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachine, *omni.ClusterMachineConfig]{
			Name: clusterMachineConfigControllerName,
			MapMetadataFunc: func(clusterMachine *omni.ClusterMachine) *omni.ClusterMachineConfig {
				return omni.NewClusterMachineConfig(resources.DefaultNamespace, clusterMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineConfig *omni.ClusterMachineConfig) *omni.ClusterMachine {
				return omni.NewClusterMachine(resources.DefaultNamespace, machineConfig.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, clusterMachine *omni.ClusterMachine, machineConfig *omni.ClusterMachineConfig) error {
				return reconcileClusterMachineConfig(ctx, r, logger, clusterMachine, machineConfig, defaultGenOptions)
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterMachineConfigPatches, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineSetNode, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineConfigGenOptions, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineStatus, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterMachineTalosVersion, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.Cluster, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterSecrets, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterConfigVersion, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.LoadBalancerConfig, *omni.ClusterMachine](),
		),
		qtransform.WithConcurrency(2),
	)
}

//nolint:gocognit,cyclop,gocyclo
func reconcileClusterMachineConfig(
	ctx context.Context,
	r controller.Reader,
	logger *zap.Logger,
	clusterMachine *omni.ClusterMachine,
	machineConfig *omni.ClusterMachineConfig,
	defaultGenOptions []generate.Option,
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

	clusterMachineTalosVersion, err := safe.ReaderGet[*omni.ClusterMachineTalosVersion](
		ctx,
		r,
		omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(),
	)
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
		clusterMachineTalosVersion,
	}

	if !helpers.UpdateInputsVersions(machineConfig, inputs...) {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("config inputs not changed"))
	}

	helpers.CopyLabels(clusterMachine, machineConfig, omni.LabelMachineSet, omni.LabelCluster, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

	// TODO: temporary transition code, remove in the future
	if clusterMachine.TypedSpec().Value.KubernetesVersion == "" {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("kubernetes version is not set yet"))
	}

	// TODO(image-factory): temporary method to preserve the existing schematics of Talos nodes on install/updates. Might change later.
	machineStatus, err := safe.ReaderGet[*omni.MachineStatus](ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	// skip if the machine schematic information is not yet detected
	if machineStatus.TypedSpec().Value.Schematic == nil {
		logger.Error("machine schematic is not set, skip reconcile")

		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("machine schematic is not set detected"))
	}

	var helper clusterMachineConfigControllerHelper

	machineConfig.TypedSpec().Value.Data, err = helper.generateConfig(
		clusterMachine,
		clusterMachineConfigPatches,
		secrets,
		loadBalancerConfig,
		cluster,
		clusterConfigVersion,
		machineConfigGenOptions,
		clusterMachineTalosVersion,
		defaultGenOptions,
		machineStatus,
	)
	if err != nil {
		machineConfig.TypedSpec().Value.GenerationError = err.Error()

		return nil //nolint:nilerr
	}

	machineConfig.TypedSpec().Value.ClusterMachineVersion = clusterMachine.Metadata().Version().String()
	machineConfig.TypedSpec().Value.GenerationError = ""

	return nil
}

type clusterMachineConfigControllerHelper struct{}

func (clusterMachineConfigControllerHelper) generateConfig(
	clusterMachine *omni.ClusterMachine,
	clusterMachineConfigPatches *omni.ClusterMachineConfigPatches,
	secrets *omni.ClusterSecrets,
	loadbalancer *omni.LoadBalancerConfig,
	cluster *omni.Cluster,
	clusterConfigVersion *omni.ClusterConfigVersion,
	machineConfigGenOptions *omni.MachineConfigGenOptions,
	clusterMachineTalosVersion *omni.ClusterMachineTalosVersion,
	extraGenOptions []generate.Option,
	machineStatus *omni.MachineStatus,
) ([]byte, error) {
	clusterName := cluster.Metadata().ID()

	talosVersion := clusterConfigVersion.TypedSpec().Value.Version
	kubernetesVersion := clusterMachine.TypedSpec().Value.KubernetesVersion

	if talosVersion == "" {
		return nil, fmt.Errorf("talos version is not set on the resource %s", clusterConfigVersion.Metadata())
	}

	installImage, err := buildInstallImage(clusterMachineTalosVersion, machineStatus, talosVersion)
	if err != nil {
		return nil, err
	}

	genOptions := []generate.Option{
		generate.WithInstallImage(installImage),
	}

	genOptions = append(genOptions, extraGenOptions...)

	if machineConfigGenOptions.TypedSpec().Value.InstallDisk != "" {
		genOptions = append(genOptions, generate.WithInstallDisk(machineConfigGenOptions.TypedSpec().Value.InstallDisk))
	}

	if talosVersion != "latest" {
		versionContract, parseErr := config.ParseContractFromVersion(talosVersion)
		if parseErr != nil {
			return nil, parseErr
		}

		genOptions = append(genOptions, generate.WithVersionContract(versionContract))

		// For Talos 1.5+, enable KubePrism feature. It's not enabled by default in the machine generation.
		if versionContract.Greater(config.TalosVersion1_4) {
			genOptions = append(genOptions, generate.WithKubePrismPort(constants.KubePrismPort))
		}
	}

	// add the advertised host of the app so kube-apiserver cert is valid for external access
	apiHost, err := appconfig.Config.GetAdvertisedAPIHost()
	if err != nil {
		return nil, err
	}

	genOptions = append(genOptions, generate.WithAdditionalSubjectAltNames([]string{apiHost}))

	secretBundle, err := omni.ToSecretsBundle(secrets)
	if err != nil {
		return nil, err
	}

	genOptions = append(genOptions, generate.WithSecretsBundle(secretBundle))

	input, err := generate.NewInput(
		clusterName,
		loadbalancer.TypedSpec().Value.SiderolinkEndpoint,
		kubernetesVersion,
		genOptions...,
	)
	if err != nil {
		return nil, err
	}

	machineType := machineapi.TypeWorker

	if _, ok := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
		machineType = machineapi.TypeControlPlane
	}

	cfg, err := input.Config(machineType)
	if err != nil {
		return nil, err
	}

	patches, err := configpatcher.LoadPatches(clusterMachineConfigPatches.TypedSpec().Value.Patches)
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

	bytes, err := strippedConfig.Bytes()
	if err != nil {
		return nil, err
	}

	headerComment := []byte(fmt.Sprintf("# Generated by Omni on %s\n", time.Now().UTC().Format(time.RFC3339Nano)))

	return append(headerComment, bytes...), nil
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

var imageFactoryHost string

func init() {
	parsed, err := url.Parse(appconfig.Config.ImageFactoryBaseURL)
	if err != nil {
		panic(err)
	}

	imageFactoryHost = parsed.Host
}

func buildInstallImage(clusterMachineTalosVersion *omni.ClusterMachineTalosVersion, machineStatus *omni.MachineStatus, talosVersion string) (string, error) {
	installerName := "installer"
	schematicConfig := machineStatus.TypedSpec().Value.Schematic

	if schematicConfig == nil {
		return "", fmt.Errorf("machine %q has no schematic information set", machineStatus.Metadata().ID())
	}

	schematicID := schematicConfig.Id

	secureBootStatus := machineStatus.TypedSpec().Value.SecureBootStatus
	if secureBootStatus == nil {
		return "", xerrors.NewTaggedf[qtransform.SkipReconcileTag]("secure boot status for machine %q is not yet set", machineStatus.Metadata().ID())
	}

	if secureBootStatus.Enabled {
		installerName = "installer-secureboot"
		schematicID = schematicConfig.FullId
	}

	if talosVersion == "latest" && schematicID != "" {
		return "", fmt.Errorf("machine %q has a schematic but using Talos version %q", machineStatus.Metadata().ID(), talosVersion)
	}

	if clusterMachineTalosVersion.TypedSpec().Value.SchematicId != "" {
		schematicID = clusterMachineTalosVersion.TypedSpec().Value.SchematicId
	}

	if schematicConfig.Invalid {
		schematicID = ""
	}

	if clusterMachineTalosVersion.TypedSpec().Value.TalosVersion != "" {
		talosVersion = clusterMachineTalosVersion.TypedSpec().Value.TalosVersion
	}

	if !strings.HasPrefix(talosVersion, "v") {
		talosVersion = "v" + talosVersion
	}

	if schematicID != "" {
		return imageFactoryHost + "/" + installerName + "/" + schematicID + ":" + talosVersion, nil
	}

	return appconfig.Config.TalosRegistry + ":" + talosVersion, nil
}
