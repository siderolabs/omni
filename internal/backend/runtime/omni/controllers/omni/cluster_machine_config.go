// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config"
	documentconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/rotatepatcher"
	machineapi "github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
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
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/installdisk"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/imagefactoryauth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/uncached"
	omnicfg "github.com/siderolabs/omni/internal/pkg/config"
)

// ClusterMachineConfigControllerName is the name of the ClusterMachineConfigController.
const ClusterMachineConfigControllerName = "ClusterMachineConfigController"

// ClusterMachineConfigController manages machine configurations for each ClusterMachine.
//
// ClusterMachineConfigController generates machine configuration for each created machine.
type ClusterMachineConfigController = qtransform.QController[*omni.ClusterMachine, *omni.ClusterMachineConfig]

// NewClusterMachineConfigController initializes ClusterMachineConfigController.
func NewClusterMachineConfigController(registryMirrors []string, talosRegistry string, registries omnicfg.Registries) *ClusterMachineConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachine, *omni.ClusterMachineConfig]{
			Name: ClusterMachineConfigControllerName,
			MapMetadataFunc: func(clusterMachine *omni.ClusterMachine) *omni.ClusterMachineConfig {
				return omni.NewClusterMachineConfig(clusterMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(machineConfig *omni.ClusterMachineConfig) *omni.ClusterMachine {
				return omni.NewClusterMachine(machineConfig.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, clusterMachine *omni.ClusterMachine, machineConfig *omni.ClusterMachineConfig) error {
				return reconcileClusterMachineConfig(ctx, r, logger, clusterMachine, machineConfig, registryMirrors, talosRegistry, registries)
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
		qtransform.WithExtraMappedInput[*omni.MachineInstallDiskStatus](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineInstallDiskConfig](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.Cluster](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineSecrets](
			qtransform.MapperSameID[*omni.ClusterMachine](),
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
		qtransform.WithExtraMappedInput[*omni.ImageFactoryAuth](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r)
				if err != nil {
					return nil, err
				}

				return slices.Collect(clusterMachines.Pointers()), nil
			},
		),
		qtransform.WithConcurrency(2),
	)
}

// getResolvedInstallDisk returns the install disk of the machine, checked to be in sync with the
// current install disk selection. When the install disk resolution is stale, the reconcile is
// skipped, and it runs again when the resolution catches up.
//
// The selection is read without the cache, as installdisk.GetResolved requires. Note that the
// MachineInstallDiskConfig input declaration on this controller is required for that read, since
// controllers can only read their declared inputs. It is not there only to trigger the controller
// on selection edits.
//
// Running this check at config generation time keeps everything downstream simple. The checked disk is
// recorded on the ClusterMachineConfig next to the config data, so the consumers (e.g., the
// install logic) use the disk recorded on the exact config version they work on, and need no
// checks of their own.
func getResolvedInstallDisk(ctx context.Context, r controller.Reader, machineID resource.ID, installDiskStatus *omni.MachineInstallDiskStatus) (string, error) {
	installDiskConfig, err := safe.ReaderGetByID[*omni.MachineInstallDiskConfig](ctx, uncached.Reader(r), machineID)
	if err != nil && !state.IsNotFoundError(err) {
		return "", fmt.Errorf("failed to get the install disk config %q: %w", machineID, err)
	}

	disk, inSync := installdisk.GetResolved(installDiskConfig, installDiskStatus)
	if !inSync {
		return "", xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"the install disk resolution of %q is stale, waiting for it to catch up with the selection", machineID,
		)
	}

	return disk, nil
}

//nolint:gocognit,cyclop,gocyclo,maintidx
func reconcileClusterMachineConfig(
	ctx context.Context,
	r controller.Reader,
	logger *zap.Logger,
	clusterMachine *omni.ClusterMachine,
	machineConfig *omni.ClusterMachineConfig,
	registryMirrors []string,
	talosRegistry string,
	registries omnicfg.Registries,
) error {
	clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("missing cluster label on %s", clusterMachine.Metadata().ID())
	}

	cluster, err := safe.ReaderGet[*omni.Cluster](ctx, r, omni.NewCluster(clusterName).Metadata())
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

	clusterMachineSecrets, err := safe.ReaderGet[*omni.ClusterMachineSecrets](ctx, r, omni.NewClusterMachineSecrets(clusterMachine.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	loadBalancerConfig, err := safe.ReaderGet[*omni.LoadBalancerConfig](ctx, r, omni.NewLoadBalancerConfig(clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	clusterConfigVersion, err := safe.ReaderGet[*omni.ClusterConfigVersion](ctx, r, omni.NewClusterConfigVersion(clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	clusterMachineConfigPatches, err := safe.ReaderGet[*omni.ClusterMachineConfigPatches](
		ctx,
		r,
		omni.NewClusterMachineConfigPatches(clusterMachine.Metadata().ID()).Metadata(),
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
		omni.NewMachineConfigGenOptions(clusterMachine.Metadata().ID()).Metadata(),
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

	installDiskStatus, err := safe.ReaderGetByID[*omni.MachineInstallDiskStatus](ctx, r, clusterMachine.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	installDisk, err := getResolvedInstallDisk(ctx, r, clusterMachine.Metadata().ID(), installDiskStatus)
	if err != nil {
		return err
	}

	// Do not generate the config until the disk is resolved, on every version contract. Older
	// contracts put the disk into the config data and reject an empty one. Newer contracts do not
	// carry the disk in the config, but still need it recorded on the resource for the install
	// logic.
	if installDisk == "" {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("install disk is not resolved yet: %s", installDiskStatus.TypedSpec().Value.Message)
	}

	// clusterMachine is not on this list on purpose. Everything read from it to generate the
	// config is set when it is created and is never updated in place. Because of this, a new
	// version of it alone never changes the generated config.
	inputs := []resource.Resource{
		clusterMachineSecrets,
		loadBalancerConfig,
		cluster,
		clusterMachineConfigPatches,
		machineConfigGenOptions,
		machineJoinConfig,
		installDiskStatus,
	}

	var imageFactories safe.List[*omni.ImageFactoryAuth]

	initialTalosVersion := clusterConfigVersion.TypedSpec().Value.Version

	if quirks.New(initialTalosVersion).SupportsMultidoc() {
		var vc *config.VersionContract

		vc, err = config.ParseContractFromVersion(initialTalosVersion)
		if err != nil {
			return fmt.Errorf("failed to parse contract from version: %w", err)
		}

		if vc.MultidocNetworkConfigSupported() {
			imageFactories, err = safe.ReaderListAll[*omni.ImageFactoryAuth](ctx, r)
			if err != nil {
				return err
			}
		}
	}

	for imageFactory := range imageFactories.All() {
		inputs = append(inputs, imageFactory)
	}

	if !helpers.UpdateInputsVersions(machineConfig, inputs...) {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("config inputs not changed"))
	}

	helpers.CopyLabels(clusterMachine, machineConfig, omni.LabelMachineSet, omni.LabelCluster, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

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
		talosRegistry: talosRegistry,
		registries:    registries,
	}

	configGenOptions := make([]generate.Option, 0, len(registryMirrors))

	for _, registryMirror := range registryMirrors {
		hostname, endpoint, ok := strings.Cut(registryMirror, "=")
		if !ok {
			return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		configGenOptions = append(configGenOptions, generate.WithRegistryMirror(hostname, endpoint))
	}

	conf, err := helper.generateConfig(clusterMachine, clusterMachineConfigPatches, clusterMachineSecrets, loadBalancerConfig,
		cluster, clusterConfigVersion, machineConfigGenOptions, installDisk, configGenOptions, machineJoinConfig, imageFactories)
	if err != nil {
		machineConfig.TypedSpec().Value.GenerationError = err.Error()

		return nil //nolint:nilerr
	}

	data, err := conf.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
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

	useUKICmdline, err := grubUseUKICmdline(conf, initialTalosVersion)
	if err != nil {
		return err
	}

	machineConfig.TypedSpec().Value.GenerationError = ""
	machineConfig.TypedSpec().Value.GrubUseUkiCmdline = useUKICmdline

	// The install uses this recorded value. Because it is recorded on the config itself, the disk
	// the install uses and the config data can never disagree.
	machineConfig.TypedSpec().Value.InstallDisk = installDisk

	return nil
}

func grubUseUKICmdline(cfg config.Provider, initialTalosVersion string) (bool, error) {
	versionContract, err := config.ParseContractFromVersion(initialTalosVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse contract from version: %w", err)
	}

	// Talos 1.14 onwards does not support `machine.install` and grubUseUKICmdline is always set to true.
	if versionContract.UnattendedInstallConfig() {
		return true, nil
	}

	return cfg.Machine().Install().GrubUseUKICmdline(), nil
}

type clusterMachineConfigControllerHelper struct {
	registries    omnicfg.Registries
	talosRegistry string
}

func (helper clusterMachineConfigControllerHelper) buildRegistryAuthPatch(creds safe.List[*omni.ImageFactoryAuth]) (string, error) {
	if creds.Len() == 0 {
		return "", nil
	}

	authDocs, err := imagefactoryauth.BuildDocs(slices.Collect(creds.All()))
	if err != nil {
		return "", fmt.Errorf("failed to build image factory registry auth doc: %w", err)
	}

	if len(authDocs) == 0 {
		return "", nil
	}

	docs := make([]documentconfig.Document, 0, len(authDocs))
	for _, authDoc := range authDocs {
		docs = append(docs, authDoc)
	}

	authConfig, err := container.New(docs...)
	if err != nil {
		return "", err
	}

	return authConfig.EncodeString()
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

//nolint:gocyclo,cyclop,gocognit
func (helper clusterMachineConfigControllerHelper) generateConfig(clusterMachine *omni.ClusterMachine, clusterMachineConfigPatches *omni.ClusterMachineConfigPatches,
	clusterMachineSecrets *omni.ClusterMachineSecrets, loadbalancer *omni.LoadBalancerConfig, cluster *omni.Cluster, clusterConfigVersion *omni.ClusterConfigVersion,
	configGenOptions *omni.MachineConfigGenOptions, installDisk string, extraGenOptions []generate.Option, machineJoinConfig *siderolink.MachineJoinConfig,
	imageFactories safe.List[*omni.ImageFactoryAuth],
) (config.Provider, error) {
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

	installImageSpec := configGenOptions.TypedSpec().Value.InstallImage

	// Migration code, if the factory URL is not populated yet, use the secondary factory URL if configured
	// As the second factory should be the old one we're migrating from
	if installImageSpec.ImageFactoryHost == "" {
		// Resolve the primary factory rather than reading registries.factories.primary directly: a
		// deployment still using the deprecated flat imageFactory* config leaves that block empty, and
		// falling back to an empty URL would yield an empty host and fail the install image build.
		primaryFactory := helper.registries.GetPrimaryFactory()
		fallbackURL := primaryFactory.GetUrl()

		if f, ok := helper.registries.GetSecondaryFactory(); ok {
			fallbackURL = f.GetUrl()
		}

		u, err := url.Parse(fallbackURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image factory URL %q: %w", fallbackURL, err)
		}

		if u.Host == "" {
			return nil, fmt.Errorf("image factory URL %q has no host", fallbackURL)
		}

		// Clone rather than writing through the pointer into the cached MachineConfigGenOptions spec.
		installImageSpec = installImageSpec.CloneVT()
		installImageSpec.ImageFactoryHost = u.Host
	}

	installImage, err := installimage.Build(configGenOptions.Metadata().ID(), installImageSpec, helper.talosRegistry)
	if err != nil {
		return nil, err
	}

	secretsBundle, err := omni.ToSecretsBundle(clusterMachineSecrets.TypedSpec().Value.Data)
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
		InstallDisk:              installDisk,
		InstallImage:             installImage,
		Secrets:                  secretsBundle,
	})
	if err != nil {
		return nil, err
	}

	cfg := genOutput.Config

	if rotation := clusterMachineSecrets.TypedSpec().Value.GetRotation(); rotation != nil {
		cfg, err = cfg.PatchV1Alpha1(func(config *v1alpha1.Config) error {
			machineAcceptedCAs := []*x509.PEMEncodedCertificate{{Crt: secretsBundle.Certs.OS.Crt}}
			if rotation.GetExtraCerts().GetOs() != nil {
				machineAcceptedCAs = append(machineAcceptedCAs, &x509.PEMEncodedCertificate{Crt: rotation.ExtraCerts.Os.Crt})
			}

			config.MachineConfig.MachineAcceptedCAs = machineAcceptedCAs

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to patch machine config: %w", err)
		}

		if extraK8s := rotation.GetExtraCerts().GetK8S(); extraK8s != nil {
			cfg, err = rotatepatcher.K8sAddAcceptedCA(extraK8s.Crt)(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to add the accepted Kubernetes CA: %w", err)
			}
		}
	}

	patchList, err := clusterMachineConfigPatches.TypedSpec().Value.GetUncompressedPatches()
	if err != nil {
		return nil, err
	}

	if _, preserveApidCheckExtKeyUsage := clusterMachine.Metadata().Annotations().Get(omni.PreserveApidCheckExtKeyUsage); preserveApidCheckExtKeyUsage {
		patchList = slices.Insert(
			patchList, 0, `machine:
  features:
    apidCheckExtKeyUsage: true
`,
		)
	}

	if _, preserveDiskQuotaSupport := clusterMachine.Metadata().Annotations().Get(omni.PreserveDiskQuotaSupport); preserveDiskQuotaSupport {
		patchList = slices.Insert(
			patchList, 0, `machine:
  features:
    diskQuotaSupport: true
`,
		)
	}

	if quirks.New(initialTalosVersion).SupportsMultidoc() {
		patchList = append(patchList, machineJoinConfig.TypedSpec().Value.Config.Config)
	}

	authPatch, authErr := helper.buildRegistryAuthPatch(imageFactories)
	if authErr != nil {
		return nil, authErr
	}

	if authPatch != "" {
		patchList = append(patchList, authPatch)
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

	patchedConfig, err = overrideInstallDisk(cfg, patchedConfig)
	if err != nil {
		return nil, err
	}

	strippedConfig, err := stripTalosAPIAccessOSAdminRole(patchedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build talos api access feature allowed roles patch: %w", err)
	}

	return strippedConfig, nil
}

// overrideInstallDisk sets the install disk of the patched config back to the disk the config
// generation produced. Config patches must not change the disk in either direction, otherwise
// the config data could disagree with the disk recorded on the ClusterMachineConfig.
//
// Creating new patches carrying the disk is not allowed, so this only overrides the patches
// created before that restriction. On the version contracts whose config carries an install
// section, the disk is set back. On the newer contracts whose config has no install section, a
// section brought in by a patch is removed completely, as an install section without a disk
// would not be valid.
//
// A patch deleting the whole install section is left alone. The old-style install driven by the
// config then has nothing to work with, which was the case before this restriction too. Machines
// installed over the install API are not affected, as that path uses the disk recorded on the
// resource.
func overrideInstallDisk(generatedConfig, patchedConfig config.Provider) (config.Provider, error) {
	generatedDisk := ""
	if generatedConfig.Machine() != nil {
		generatedDisk = generatedConfig.Machine().Install().Disk()
	}

	if patchedConfig.Machine() == nil || patchedConfig.Machine().Install().Disk() == generatedDisk {
		return patchedConfig, nil
	}

	patchedConfig, err := patchedConfig.PatchV1Alpha1(func(v1alpha1Config *v1alpha1.Config) error {
		// This is reachable: a patch can delete the whole machine section while the v1alpha1
		// document remains, and the accessor used above hides that by returning an empty struct.
		if v1alpha1Config.MachineConfig == nil {
			return nil
		}

		if generatedDisk == "" {
			v1alpha1Config.MachineConfig.MachineInstall = nil //nolint:staticcheck

			return nil
		}

		if v1alpha1Config.MachineConfig.MachineInstall != nil { //nolint:staticcheck
			v1alpha1Config.MachineConfig.MachineInstall.InstallDisk = generatedDisk //nolint:staticcheck
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to override the install disk on the patched config: %w", err)
	}

	return patchedConfig, nil
}

// stripTalosAPIAccessOSAdminRole ensures that the OS admin role is never included in the allowed roles of the
// Kubernetes Talos API Access feature configuration.
//
// Config patches are already validated to not contain it, this is merely an additional safety measure.
func stripTalosAPIAccessOSAdminRole(cfg config.Provider) (config.Provider, error) {
	if cfg.Machine() == nil {
		return cfg, nil
	}

	talosAPIAccess := cfg.K8sTalosAPIAccessConfig()
	if talosAPIAccess == nil {
		return cfg, nil
	}

	allowedRoles := talosAPIAccess.AllowedRoles()
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

	// KubeTalosAPIAccessConfig is used, short-circuit
	if cfg.Has(k8s.KubeTalosAPIAccessConfig) {
		return container.PatchDocument(
			cfg,
			func(access *k8s.KubeTalosAPIAccessConfigV1Alpha1) error {
				access.AccessAllowedRoles = filteredAllowedRoles

				return nil
			},
		)
	}

	configDocs := cfg.Documents()
	updatedDocs := make([]documentconfig.Document, 0, len(configDocs))

	for _, document := range configDocs {
		if document.APIVersion() == "" && document.Kind() == v1alpha1.Version {
			v1alpha1Config := cfg.RawV1Alpha1() // this ensures that we get a writeable copy of v1alpha1 config

			v1alpha1Config.MachineConfig.MachineFeatures.KubernetesTalosAPIAccessConfig.AccessAllowedRoles = filteredAllowedRoles //nolint:staticcheck

			updatedDocs = append(updatedDocs, v1alpha1Config)

			continue
		}

		updatedDocs = append(updatedDocs, document)
	}

	return container.New(updatedDocs...)
}
