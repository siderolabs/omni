// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package omni implements the internal service runtime.
package omni

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/destroy"
	cosiruntime "github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	cosiresource "github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/siderolabs/gen/optional"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	virtualres "github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	pkgruntime "github.com/siderolabs/omni/client/pkg/runtime"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/powerstage"
	"github.com/siderolabs/omni/internal/backend/resourcelogger"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/cosi"
	"github.com/siderolabs/omni/internal/backend/runtime/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/clustermachine"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/image"
	kernelargsctrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineupgrade"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/redactedmachineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual/pkg/producers"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
	newgroup "github.com/siderolabs/omni/internal/pkg/errgroup"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// Name is the runtime time.
var Name = common.Runtime_Omni.String()

// Runtime implements Omni internal runtime.
type Runtime struct {
	controllerRuntime  *cosiruntime.Runtime
	talosClientFactory *talos.ClientFactory
	kubernetesRuntime  omnictrl.KubernetesRuntime
	storeFactory       store.Factory

	dnsService              *dns.Service
	workloadProxyReconciler *workloadproxy.Reconciler
	resourceLogger          *resourcelogger.Logger

	// resource state for internal consumers
	state       state.State
	cachedState state.State
	virtual     *virtual.State

	logger            *zap.Logger
	powerStageWatcher *powerstage.Watcher
}

// NewRuntime creates a new Omni runtime.
//
//nolint:maintidx
func NewRuntime(cfg *config.Params, talosClientFactory *talos.ClientFactory, dnsService *dns.Service, workloadProxyReconciler *workloadproxy.Reconciler,
	resourceLogger *resourcelogger.Logger, imageFactoryClient *imagefactory.Client, linkCounterDeltaCh <-chan siderolink.LinkCounterDeltas,
	siderolinkEventsCh <-chan *omni.MachineStatusSnapshot, installEventCh <-chan cosiresource.ID, st *State, metricsRegistry prometheus.Registerer,
	discoveryClientCache omnictrl.DiscoveryClientCache, kubernetesRuntime omnictrl.KubernetesRuntime, talosRuntime omnictrl.TalosClientGetter, logger *zap.Logger,
) (*Runtime, error) {
	var opts []options.Option

	if !cfg.Features.GetDisableControllerRuntimeCache() {
		opts = append(opts, RuntimeCacheOptions()...)
	}

	defaultState := st.Default()

	controllerRuntime, err := cosiruntime.NewRuntime(defaultState, logger, opts...)
	if err != nil {
		return nil, err
	}

	storeFactory := st.StoreFactory()

	powerStageEventsCh := make(chan *omni.MachineStatusSnapshot)
	powerStageWatcher := powerstage.NewWatcher(defaultState, powerStageEventsCh, logger.With(logging.Component("power_stage_watcher")), powerstage.WatcherOptions{})

	uploadLimitPerSecondBytes := cfg.EtcdBackup.GetUploadLimitMbps() * 125000     // Mbit in bytes
	downloadLimitPerSecondBytes := cfg.EtcdBackup.GetDownloadLimitMbps() * 125000 // Mbit in bytes

	storeFactory.SetThroughputs(uploadLimitPerSecondBytes, downloadLimitPerSecondBytes)
	metricsRegistry.MustRegister(storeFactory)

	backupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(ctx context.Context, clusterName string) (omnictrl.TalosClient, error) {
			return talosClientFactory.Get(ctx, clusterName)
		},
		StoreFactory: storeFactory,
		TickInterval: cfg.EtcdBackup.GetTickInterval(),
		Jitter:       cfg.EtcdBackup.GetJitter(),
	})
	if err != nil {
		return nil, err
	}

	controllers := []controller.Controller{
		omnictrl.NewCertRefreshTickController(constants.CertificateValidityTime / 10), // issue ticks at 10% of the validity, as we refresh certificates at 50% of the validity
		omnictrl.NewClusterController(kubernetesRuntime),
		omnictrl.NewMachineSetController(),
		&omnictrl.ClusterMachineIdentityController{},
		&omnictrl.ClusterMachineStatusMetricsController{},
		omnictrl.NewClusterMachineController(),
		&omnictrl.ClusterMachineEncryptionController{},
		&omnictrl.ClusterMetricsController{},
		&omnictrl.ClusterStatusMetricsController{},
		&omnictrl.ClusterWorkloadProxyController{},
		&omnictrl.ConfigPatchCleanupController{},
		&omnictrl.ConfigPatchMetricsController{},
		omnictrl.NewEtcdBackupOverallStatusController(),
		backupController,
		omnictrl.NewImagePullStatusController(&image.TalosImageClient{
			TalosClientFactory: talosClientFactory,
			NodeResolver:       dnsService,
		}),
		omnictrl.NewKubernetesStatusController(kubernetesRuntime, cfg.Services.Api.URL(), cfg.Services.WorkloadProxy.GetSubdomain(), cfg.Services.WorkloadProxy.GetEnabled()),
		&omnictrl.LoadBalancerController{
			LBConfig: cfg.Services.LoadBalancer,
		},
		omnictrl.NewMachineCleanupController(),
		omnictrl.NewMachineStatusLinkController(linkCounterDeltaCh),
		&omnictrl.MachineStatusMetricsController{},
		omnictrl.NewVersionsController(cfg.Registries.GetImageFactoryBaseURL(), cfg.Features.GetEnableTalosPreReleaseVersions(), cfg.Registries.GetKubernetes()),
		omnictrl.NewClusterLoadBalancerController(
			cfg.Services.LoadBalancer.GetMinPort(),
			cfg.Services.LoadBalancer.GetMaxPort(),
		),
		omnictrl.NewInstallationMediaController(cfg.Account.GetName()),
		omnictrl.NewKeyPrunerController(
			cfg.Auth.KeyPruner.GetInterval(),
		),
		&omnictrl.OngoingTaskController{},
		omnictrl.NewMachineRequestStatusCleanupController(),
		omnictrl.NewInfraProviderCleanupController(),
		omnictrl.NewLinkCleanupController(),
	}

	imageFactoryHost := imageFactoryClient.Host()
	peers := siderolink.NewPeersPool(logger, siderolink.DefaultWireguardHandler)

	exraKernelArgsInitializer, err := kernelargs.NewInitializer(defaultState, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create extra kernel args initializer: %w", err)
	}

	qcontrollers := []controller.QController{
		destroy.NewController[*siderolinkres.Link](optional.Some[uint](4)),
		destroy.NewController[*omni.Cluster](optional.Some[uint](4)),
		destroy.NewController[*omni.MachineSet](optional.Some[uint](4)),

		omnictrl.NewBackupDataController(),
		omnictrl.NewClusterBootstrapStatusController(storeFactory),
		omnictrl.NewClusterConfigVersionController(),
		omnictrl.NewClusterDestroyStatusController(),
		omnictrl.NewMachineSetDestroyStatusController(),
		omnictrl.NewClusterEndpointController(),
		omnictrl.NewClusterKubernetesNodesController(),
		omnictrl.NewClusterMachineConfigController(
			imageFactoryHost,
			cfg.Registries.Mirrors,
			cfg.Registries.GetTalos(),
		),
		omnictrl.NewClusterMachineTeardownController(omnictrl.NewGetKubernetesClientFunc(kubernetesRuntime)),
		omnictrl.NewMachineConfigGenOptionsController(),
		omnictrl.NewMachineStatusController(imageFactoryClient, exraKernelArgsInitializer),
		machineconfig.NewClusterMachineConfigStatusController(imageFactoryHost, cfg.Registries.GetTalos()),
		omnictrl.NewClusterMachineEncryptionKeyController(),
		omnictrl.NewClusterMachineStatusController(),
		omnictrl.NewClusterStatusController(cfg.Services.EmbeddedDiscoveryService.GetEnabled()),
		omnictrl.NewClusterDiagnosticsController(),
		omnictrl.NewClusterUUIDController(),
		omnictrl.NewControlPlaneStatusController(),
		omnictrl.NewDiscoveryServiceConfigPatchController(cfg.Services.EmbeddedDiscoveryService.GetPort()),
		omnictrl.NewKubernetesNodeAuditController(omnictrl.NewGetKubernetesClientFunc(kubernetesRuntime), time.Minute),
		omnictrl.NewEtcdBackupEncryptionController(),
		omnictrl.NewClusterWorkloadProxyStatusController(workloadProxyReconciler),
		omnictrl.NewKubeconfigController(constants.CertificateValidityTime),
		omnictrl.NewKubernetesUpgradeManifestStatusController(talosRuntime, kubernetesRuntime),
		omnictrl.NewKubernetesUpgradeStatusController(),
		omnictrl.NewMachineController(),
		omnictrl.NewMachineExtensionsController(),
		omnictrl.NewMachineSetStatusController(),
		omnictrl.NewMachineSetNodeController(),
		omnictrl.NewMachineSetEtcdAuditController(talosClientFactory, time.Minute),
		redactedmachineconfig.NewController(redactedmachineconfig.ControllerOptions{}),
		omnictrl.NewSchematicConfigurationController(imageFactoryClient),
		secrets.NewSecretsController(storeFactory),
		secrets.NewTalosConfigController(constants.CertificateValidityTime),
		omnictrl.NewTalosExtensionsController(imageFactoryClient),
		omnictrl.NewTalosUpgradeStatusController(),
		omnictrl.NewMachineStatusSnapshotController(siderolinkEventsCh, powerStageEventsCh),
		omnictrl.NewMachineProvisionController(),
		omnictrl.NewMachineRequestLinkController(defaultState),
		omnictrl.NewLabelsExtractorController[*omni.MachineStatus](),
		omnictrl.NewMachineRequestSetStatusController(),
		omnictrl.NewClusterMachineRequestStatusController(),
		omnictrl.NewMachineTeardownController(),
		omnictrl.NewInfraMachineController(installEventCh),
		omnictrl.NewBMCConfigController(),
		omnictrl.NewInfraProviderConfigPatchController(),
		omnictrl.NewLinkStatusController[*siderolinkres.Link](peers),
		omnictrl.NewLinkStatusController[*siderolinkres.PendingMachine](peers),
		omnictrl.NewPendingMachineStatusController(),
		omnictrl.NewMaintenanceConfigStatusController(nil,
			cfg.Services.Siderolink.GetEventSinkPort(),
			cfg.Services.Siderolink.GetLogServerPort()),
		omnictrl.NewDiscoveryAffiliateDeleteTaskController(clockwork.NewRealClock(), discoveryClientCache),
		omnictrl.NewInfraProviderCombinedStatusController(),
		omnictrl.NewServiceAccountStatusController(),
		omnictrl.NewInfraMachineRegistrationController(),
		omnictrl.NewSiderolinkAPIConfigController(cfg.Services.MachineAPI.URL(), cfg.Services.Siderolink),
		omnictrl.NewProviderJoinConfigController(),
		omnictrl.NewMachineJoinConfigController(),
		omnictrl.NewConnectionParamsController(cfg.Services.MachineAPI.URL(), cfg.Services.Siderolink),
		omnictrl.NewJoinTokenStatusController(),
		omnictrl.NewNodeUniqueTokenCleanupController(time.Minute),
		clustermachine.NewConfigPatchesController(),
		secrets.NewSecretRotationStatusController(&secrets.TalosRemoteGeneratorFactory{}),
		machineupgrade.NewStatusController(imageFactoryHost, cfg.Registries.GetTalos(), nil),
		kernelargsctrl.NewStatusController(),
	}

	if cfg.Auth.Saml.GetEnabled() {
		controllers = append(controllers,
			&omnictrl.SAMLAssertionController{},
		)
	}

	if cfg.Services.Siderolink.GetJoinTokensMode() != config.SiderolinkServiceJoinTokensModeLegacy {
		qcontrollers = append(qcontrollers,
			omnictrl.NewNodeUniqueTokenStatusController(),
		)
	}

	if cfg.Logs.Stripe.GetEnabled() {
		stripeAPIKey, ok := os.LookupEnv("STRIPE_API_KEY")
		if !ok {
			return nil, fmt.Errorf("environment variable STRIPE_API_KEY is not set")
		}

		subscriptionItemID, ok := os.LookupEnv("STRIPE_SUBSCRIPTION_ITEM_ID")
		if !ok {
			return nil, fmt.Errorf("environment variable STRIPE_SUBSCRIPTION_ITEM_ID is not set")
		}

		controllers = append(controllers,
			omnictrl.NewStripeMetricsReporterController(stripeAPIKey, subscriptionItemID, cfg.Logs.Stripe.GetMinCommit()),
		)
	}

	for _, c := range controllers {
		if err = controllerRuntime.RegisterController(c); err != nil {
			return nil, err
		}

		if collector, ok := c.(prometheus.Collector); ok {
			metricsRegistry.MustRegister(collector)
		}
	}

	for _, c := range qcontrollers {
		if err = controllerRuntime.RegisterQController(c); err != nil {
			return nil, err
		}

		if collector, ok := c.(prometheus.Collector); ok {
			metricsRegistry.MustRegister(collector)
		}
	}

	//nolint:lll
	expvarCollector := collectors.NewExpvarCollector(map[string]*prometheus.Desc{
		"controller_crashes":         prometheus.NewDesc("omni_runtime_controller_crashes", "Number of controller-runtime controller crashes by controller name.", []string{"controller"}, nil),
		"controller_wakeups":         prometheus.NewDesc("omni_runtime_controller_wakeups", "Number of controller-runtime controller wakeups by controller name.", []string{"controller"}, nil),
		"controller_reads":           prometheus.NewDesc("omni_runtime_controller_reads", "Number of controller-runtime controller reads by controller name.", []string{"controller"}, nil),
		"controller_writes":          prometheus.NewDesc("omni_runtime_controller_writes", "Number of controller-runtime controller writes by controller name.", []string{"controller"}, nil),
		"reconcile_busy":             prometheus.NewDesc("omni_runtime_controller_busy_seconds", "Number of seconds controller-runtime controller is busy by controller name.", []string{"controller"}, nil),
		"reconcile_cycles":           prometheus.NewDesc("omni_runtime_controller_reconcile_cycles", "Number of reconcile cycles by controller name.", []string{"controller"}, nil),
		"reconcile_input_items":      prometheus.NewDesc("omni_runtime_controller_reconcile_input_items", "Number of reconciled input items by controller name.", []string{"controller"}, nil),
		"qcontroller_crashes":        prometheus.NewDesc("omni_runtime_qcontroller_crashes", "Number of queue controller crashes by controller name.", []string{"controller"}, nil),
		"qcontroller_requeues":       prometheus.NewDesc("omni_runtime_qcontroller_requeues", "Number of queue controller requeues by controller name.", []string{"controller"}, nil),
		"qcontroller_processed":      prometheus.NewDesc("omni_runtime_qcontroller_processed", "Number of queue controller processed items by controller name.", []string{"controller"}, nil),
		"qcontroller_mapped_in":      prometheus.NewDesc("omni_runtime_qcontroller_mapped_in", "Number of queue controller mapped in items by controller name.", []string{"controller"}, nil),
		"qcontroller_mapped_out":     prometheus.NewDesc("omni_runtime_qcontroller_mapped_out", "Number of queue controller mapped out items by controller name.", []string{"controller"}, nil),
		"qcontroller_queue_length":   prometheus.NewDesc("omni_runtime_qcontroller_queue_length", "Number of queue controller queue length by controller name.", []string{"controller"}, nil),
		"qcontroller_map_busy":       prometheus.NewDesc("omni_runtime_qcontroller_map_busy_seconds", "Number of seconds queue controller is busy by controller name.", []string{"controller"}, nil),
		"qcontroller_reconcile_busy": prometheus.NewDesc("omni_runtime_qcontroller_reconcile_busy_seconds", "Number of seconds queue controller reconcile is busy by controller name.", []string{"controller"}, nil),
		"cached_resources":           prometheus.NewDesc("omni_runtime_cached_resources", "Number of cached resources by resource type.", []string{"resource_type"}, nil),
	})

	metricsRegistry.MustRegister(expvarCollector)

	validationOptions := slices.Concat(
		clusterValidationOptions(defaultState, cfg.EtcdBackup, cfg.Services.EmbeddedDiscoveryService),
		relationLabelsValidationOptions(),
		accessPolicyValidationOptions(),
		authorizationValidationOptions(defaultState, cfg.Auth.Saml.GetEnabled()),
		roleValidationOptions(),
		machineSetNodeValidationOptions(defaultState),
		machineSetValidationOptions(defaultState, storeFactory),
		machineClassValidationOptions(defaultState),
		identityValidationOptions(cfg.Auth.Saml),
		exposedServiceValidationOptions(),
		configPatchValidationOptions(defaultState),
		etcdManualBackupValidationOptions(),
		samlLabelRuleValidationOptions(),
		s3ConfigValidationOptions(),
		machineRequestSetValidationOptions(defaultState),
		infraMachineConfigValidationOptions(defaultState),
		nodeForceDestroyRequestValidationOptions(defaultState),
		joinTokenValidationOptions(defaultState),
		defaultJoinTokenValidationOptions(defaultState),
		importedClusterSecretValidationOptions(defaultState, cfg.Features.GetEnableClusterImport()),
		infraProviderValidationOptions(defaultState),
		installationMediaConfigValidationOptions(),
		rotateSecretsValidationOptions(defaultState),
	)

	return &Runtime{
		controllerRuntime:       controllerRuntime,
		talosClientFactory:      talosClientFactory,
		kubernetesRuntime:       kubernetesRuntime,
		storeFactory:            storeFactory,
		dnsService:              dnsService,
		workloadProxyReconciler: workloadProxyReconciler,
		resourceLogger:          resourceLogger,
		powerStageWatcher:       powerStageWatcher,
		state:                   state.WrapCore(validated.NewState(defaultState, validationOptions...)),
		cachedState:             state.WrapCore(validated.NewState(controllerRuntime.CachedState(), validationOptions...)),
		virtual:                 st.Virtual(),
		logger:                  logger,
	}, nil
}

// RuntimeCacheOptions returns the COSI runtime cache options.
func RuntimeCacheOptions() []options.Option {
	return []options.Option{
		safe.WithResourceCache[*omni.BackupData](),
		safe.WithResourceCache[*omni.ConfigPatch](),
		safe.WithResourceCache[*omni.Cluster](),
		safe.WithResourceCache[*omni.ClusterBootstrapStatus](),
		safe.WithResourceCache[*omni.ClusterConfigVersion](),
		safe.WithResourceCache[*omni.ClusterDestroyStatus](),
		safe.WithResourceCache[*omni.ClusterEndpoint](),
		safe.WithResourceCache[*omni.ClusterKubernetesNodes](),
		safe.WithResourceCache[*omni.ClusterMachine](),
		safe.WithResourceCache[*omni.ClusterMachineConfig](),
		safe.WithResourceCache[*omni.ClusterMachineConfigPatches](),
		safe.WithResourceCache[*omni.ClusterMachineConfigStatus](),
		safe.WithResourceCache[*omni.ClusterMachineEncryptionKey](),
		safe.WithResourceCache[*omni.ClusterMachineIdentity](),
		safe.WithResourceCache[*omni.ClusterMachineSecrets](),
		safe.WithResourceCache[*omni.ClusterMachineStatus](),
		safe.WithResourceCache[*omni.ClusterMachineRequestStatus](),
		safe.WithResourceCache[*omni.ClusterMachineTalosVersion](),
		safe.WithResourceCache[*omni.ClusterStatus](),
		safe.WithResourceCache[*omni.ClusterSecrets](),
		safe.WithResourceCache[*omni.ClusterSecretsRotationStatus](),
		safe.WithResourceCache[*omni.ClusterUUID](),
		safe.WithResourceCache[*omni.ClusterWorkloadProxyStatus](),
		safe.WithResourceCache[*omni.ControlPlaneStatus](),
		safe.WithResourceCache[*omni.DiscoveryAffiliateDeleteTask](),
		safe.WithResourceCache[*omni.EtcdAuditResult](),
		safe.WithResourceCache[*omni.EtcdBackupEncryption](),
		safe.WithResourceCache[*omni.EtcdBackupStatus](),
		safe.WithResourceCache[*omni.ExposedService](),
		safe.WithResourceCache[*omni.ExtensionsConfiguration](),
		safe.WithResourceCache[*omni.KernelArgs](),
		safe.WithResourceCache[*omni.KernelArgsStatus](),
		safe.WithResourceCache[*omni.ImagePullRequest](),
		safe.WithResourceCache[*omni.ImagePullStatus](),
		safe.WithResourceCache[*omni.ImportedClusterSecrets](),
		safe.WithResourceCache[*omni.InstallationMediaConfig](),
		safe.WithResourceCache[*omni.InfraProviderCombinedStatus](),
		safe.WithResourceCache[*omni.Kubeconfig](),
		safe.WithResourceCache[*omni.KubernetesNodeAuditResult](),
		safe.WithResourceCache[*omni.KubernetesStatus](),
		safe.WithResourceCache[*omni.KubernetesUpgradeManifestStatus](),
		safe.WithResourceCache[*omni.KubernetesUpgradeStatus](),
		safe.WithResourceCache[*omni.KubernetesVersion](),
		safe.WithResourceCache[*omni.LoadBalancerConfig](),
		safe.WithResourceCache[*omni.LoadBalancerStatus](),
		safe.WithResourceCache[*omni.Machine](),
		safe.WithResourceCache[*omni.MachineClass](),
		safe.WithResourceCache[*omni.MachineConfigGenOptions](),
		safe.WithResourceCache[*omni.MachineExtensions](),
		safe.WithResourceCache[*omni.MachineExtensionsStatus](),
		safe.WithResourceCache[*omni.MachineLabels](),
		safe.WithResourceCache[*omni.MachineSet](),
		safe.WithResourceCache[*omni.MachineSetDestroyStatus](),
		safe.WithResourceCache[*omni.MachineSetStatus](),
		safe.WithResourceCache[*omni.MachineSetNode](),
		safe.WithResourceCache[*omni.MachineStatus](),
		safe.WithResourceCache[*omni.MachineStatusSnapshot](),
		safe.WithResourceCache[*omni.NodeForceDestroyRequest](),
		safe.WithResourceCache[*omni.RedactedClusterMachineConfig](),
		safe.WithResourceCache[*omni.RotateTalosCA](),
		safe.WithResourceCache[*omni.Schematic](),
		safe.WithResourceCache[*omni.SchematicConfiguration](),
		safe.WithResourceCache[*omni.SecretRotation](),
		safe.WithResourceCache[*omni.TalosConfig](),
		safe.WithResourceCache[*omni.TalosExtensions](),
		safe.WithResourceCache[*omni.TalosUpgradeStatus](),
		safe.WithResourceCache[*omni.TalosVersion](),
		safe.WithResourceCache[*omni.MaintenanceConfigStatus](),
		safe.WithResourceCache[*omni.MachineUpgradeStatus](),
		safe.WithResourceCache[*omni.MachineConfigDiff](),
		safe.WithResourceCache[*siderolinkres.Config](),
		safe.WithResourceCache[*siderolinkres.ConnectionParams](), //nolint:staticcheck
		safe.WithResourceCache[*siderolinkres.Link](),
		safe.WithResourceCache[*siderolinkres.NodeUniqueToken](),
		safe.WithResourceCache[*siderolinkres.NodeUniqueTokenStatus](),
		safe.WithResourceCache[*system.ResourceLabels[*omni.MachineStatus]](),
		safe.WithResourceCache[*infra.ConfigPatchRequest](),
		safe.WithResourceCache[*auth.ServiceAccountStatus](),
		safe.WithResourceCache[*siderolinkres.MachineJoinConfig](),
		safe.WithResourceCache[*siderolinkres.ProviderJoinConfig](),
		safe.WithResourceCache[*siderolinkres.APIConfig](),
		safe.WithResourceCache[*siderolinkres.JoinToken](),
		safe.WithResourceCache[*siderolinkres.JoinTokenStatus](),
		safe.WithResourceCache[*siderolinkres.DefaultJoinToken](),
		safe.WithResourceCache[*siderolinkres.JoinTokenUsage](),
		options.WithWarnOnUncachedReads(false), // turn this to true to debug resource cache misses
	}
}

// Run starts underlying COSI controller runtime.
func (r *Runtime) Run(ctx context.Context, eg newgroup.EGroup) {
	makeWrap := func(f func(context.Context) error, msgOnError string) func() error {
		return func() error {
			if err := f(ctx); err != nil {
				return fmt.Errorf("%s: %w", msgOnError, err)
			}

			return nil
		}
	}

	if r.resourceLogger != nil {
		newgroup.GoWithContext(ctx, eg, makeWrap(r.resourceLogger.Start, "resource logger failed"))
	}

	newgroup.GoWithContext(ctx, eg, makeWrap(r.powerStageWatcher.Run, "power stage watcher failed"))
	newgroup.GoWithContext(ctx, eg, makeWrap(r.talosClientFactory.StartCacheManager, "talos client factory failed"))
	newgroup.GoWithContext(ctx, eg, makeWrap(r.dnsService.Start, "dns service failed"))
	newgroup.GoWithContext(ctx, eg, makeWrap(r.controllerRuntime.Run, "controller runtime failed"))
	newgroup.GoWithContext(ctx, eg, makeWrap(r.workloadProxyReconciler.Run, "workload proxy reconciler failed"))

	newgroup.GoWithContext(ctx, eg, func() error { return r.storeFactory.Start(ctx, r.state, r.logger) })

	if r.virtual == nil {
		return
	}

	newgroup.GoWithContext(ctx, eg, func() error {
		r.virtual.RunComputed(ctx, virtualres.KubernetesUsageType, producers.NewKubernetesUsageFactory(r.kubernetesRuntime), producers.KubernetesUsageResourceTransformer(r.state), r.logger)

		return nil
	})
}

// Watch implements runtime.Runtime.
func (r *Runtime) Watch(ctx context.Context, events chan<- runtime.WatchResponse, setters ...runtime.QueryOption) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("watch panic")

			r.logger.Error("watch panicked", zap.Stack("stack"), zap.Error(err))
		}
	}()

	return r.watch(ctx, events, setters...)
}

func (r *Runtime) watch(ctx context.Context, events chan<- runtime.WatchResponse, setters ...runtime.QueryOption) error {
	opts := runtime.NewQueryOptions(setters...)

	var (
		queries []cosiresource.LabelQuery
		err     error
	)

	if len(opts.LabelSelectors) > 0 {
		queries, err = labels.ParseSelectors(opts.LabelSelectors)
		if err != nil {
			return err
		}
	}

	return cosi.Watch(
		ctx,
		r.state,
		cosiresource.NewMetadata(
			opts.Namespace,
			opts.Resource,
			opts.Name,
			cosiresource.VersionUndefined,
		),
		events,
		opts.TailEvents,
		queries,
	)
}

// Get implements runtime.Runtime.
func (r *Runtime) Get(ctx context.Context, setters ...runtime.QueryOption) (any, error) {
	opts := runtime.NewQueryOptions(setters...)

	metadata := cosiresource.NewMetadata(opts.Namespace, opts.Resource, opts.Name, cosiresource.VersionUndefined)

	res, err := r.cachedState.Get(ctx, metadata)
	if err != nil {
		return nil, err
	}

	return runtime.NewResource(res)
}

// List implements runtime.Runtime.
func (r *Runtime) List(ctx context.Context, setters ...runtime.QueryOption) (runtime.ListResult, error) {
	opts := runtime.NewQueryOptions(setters...)

	var listOptions []state.ListOption

	if len(opts.LabelSelectors) > 0 {
		queries, err := labels.ParseSelectors(opts.LabelSelectors)
		if err != nil {
			return runtime.ListResult{}, err
		}

		for _, query := range queries {
			listOptions = append(listOptions, state.WithLabelQuery(cosiresource.RawLabelQuery(query)))
		}
	}

	list, err := r.cachedState.List(
		ctx,
		cosiresource.NewMetadata(opts.Namespace, opts.Resource, "", cosiresource.VersionUndefined),
		listOptions...,
	)
	if err != nil {
		return runtime.ListResult{}, err
	}

	items := make([]pkgruntime.ListItem, 0, len(list.Items))

	for _, item := range list.Items {
		res, err := runtime.NewResource(item)
		if err != nil {
			return runtime.ListResult{}, err
		}

		items = append(items, NewItem(res))
	}

	return runtime.ListResult{
		Items: items,
		Total: len(items),
	}, nil
}

// Create implements runtime.Runtime.
func (r *Runtime) Create(ctx context.Context, resource cosiresource.Resource, _ ...runtime.QueryOption) error {
	return r.state.Create(ctx, resource)
}

// Update implements runtime.Runtime.
func (r *Runtime) Update(ctx context.Context, resource cosiresource.Resource, _ ...runtime.QueryOption) error {
	if resource.Metadata().Version().String() == cosiresource.VersionUndefined.String() {
		current, err := r.state.Get(ctx, resource.Metadata())
		if err != nil {
			return err
		}

		resource.Metadata().SetVersion(current.Metadata().Version())
	}

	return r.state.Update(ctx, resource)
}

// Delete implements runtime.Runtime.
func (r *Runtime) Delete(ctx context.Context, setters ...runtime.QueryOption) error {
	opts := runtime.NewQueryOptions(setters...)

	md := cosiresource.NewMetadata(opts.Namespace, opts.Resource, opts.Name, cosiresource.VersionUndefined)

	if opts.TeardownOnly {
		_, err := r.state.Teardown(ctx, md)

		return err
	}

	return r.state.TeardownAndDestroy(ctx, md)
}

// CachedState returns runtime cached state.
func (r *Runtime) CachedState() state.State { //nolint:ireturn
	return r.cachedState
}

// GetCOSIRuntime returns COSI  controller runtime.
func (r *Runtime) GetCOSIRuntime() *cosiruntime.Runtime {
	return r.controllerRuntime
}

// RawTalosconfig returns the raw admin talosconfig for the cluster with the given name.
func (r *Runtime) RawTalosconfig(ctx context.Context, clusterName string) ([]byte, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	clusterEndpoint, err := safe.StateGet[*omni.ClusterEndpoint](ctx, r.state, omni.NewClusterEndpoint(clusterName).Metadata())
	if err != nil {
		return nil, err
	}

	endpoints := clusterEndpoint.TypedSpec().Value.GetManagementAddresses()

	talosConfig, err := safe.StateGet[*omni.TalosConfig](ctx, r.state, omni.NewTalosConfig(clusterName).Metadata())
	if err != nil {
		return nil, err
	}

	return omni.NewTalosClientConfig(talosConfig, endpoints...).Bytes()
}

// OperatorTalosconfig returns the os:operator talosconfig for the cluster with the given name.
func (r *Runtime) OperatorTalosconfig(ctx context.Context, clusterName string) ([]byte, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	endpoints, err := helpers.GetMachineEndpoints(ctx, r.state, clusterName)
	if err != nil {
		return nil, err
	}

	s, err := safe.StateGetByID[*omni.ClusterSecrets](ctx, r.state, clusterName)
	if err != nil {
		return nil, err
	}

	bundle, err := omni.ToSecretsBundle(s.TypedSpec().Value.Data)
	if err != nil {
		return nil, err
	}

	clientSecret, err := bundle.GenerateTalosAPIClientCertificate(role.MakeSet(role.Operator))
	if err != nil {
		return nil, err
	}

	config := clientconfig.NewConfig(
		clusterName,
		endpoints,
		bundle.Certs.OS.Crt,
		clientSecret,
	)

	return config.Bytes()
}

type item struct {
	runtime.BasicItem[*runtime.Resource]
}

func (it *item) Field(name string) (string, bool) {
	val, ok := it.BasicItem.Field(name)
	if ok {
		return val, true
	}

	val, ok = runtime.ResourceField(it.BasicItem.Unwrap().Resource, name)
	if ok {
		return val, true
	}

	return "", false
}

func (it *item) Match(searchFor string) bool {
	return it.BasicItem.Match(searchFor) || runtime.MatchResource(it.BasicItem.Unwrap().Resource, searchFor)
}

func (it *item) Unwrap() any {
	return it.BasicItem.Unwrap()
}

// NewItem creates new runtime.ListItem from the resource.
func NewItem(res *runtime.Resource) pkgruntime.ListItem {
	return &item{BasicItem: runtime.MakeBasicItem(res.Metadata.ID, res.Metadata.Namespace, res)}
}
