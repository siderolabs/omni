// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package app contains the Omni startup methods.
package app

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/go-logr/zapr"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend"
	"github.com/siderolabs/omni/internal/backend/discovery"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/resourcelogger"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/user"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	"github.com/siderolabs/omni/internal/pkg/features"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// Run the Omni service.
func Run(ctx context.Context, state *omni.State, cfg *config.Params, logger *zap.Logger) error {
	talosClientFactory := talos.NewClientFactory(state.Default(), logger)
	talosRuntime := talos.New(talosClientFactory, logger, cfg.Account.GetName(), cfg.Services.Api.URL())

	oidcIssuerEndpoint, err := cfg.GetOIDCIssuerEndpoint()
	if err != nil {
		return fmt.Errorf("failed to get OIDC issuer endpoint: %w", err)
	}

	kubernetesRuntime, err := kubernetes.New(state.Default(), oidcIssuerEndpoint, cfg.Account.GetName(), cfg.Services.KubernetesProxy.URL())
	if err != nil {
		return err
	}

	prometheus.MustRegister(talosClientFactory)
	prometheus.MustRegister(kubernetesRuntime)

	runtime.Install(kubernetes.Name, kubernetesRuntime)
	runtime.Install(talos.Name, talosRuntime)

	grpc_prometheus.EnableHandlingTimeHistogram(grpc_prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 1, 10, 30, 60, 120, 300, 600}))

	logger.Debug("using config", zap.Any("config", cfg))

	klog.SetLogger(zapr.NewLogger(logger.WithOptions(zap.IncreaseLevel(zapcore.WarnLevel)).With(logging.Component("kubernetes"))))

	ctx = actor.MarkContextAsInternalActor(ctx)

	dnsService := dns.NewService(state.Default(), logger)
	workloadProxyReconciler := workloadproxy.NewReconciler(logger.With(logging.Component("workload_proxy_reconciler")),
		zapcore.DebugLevel, cfg.Services.WorkloadProxy.GetStopLBsAfter())

	var resourceLogger *resourcelogger.Logger

	if len(cfg.Logs.ResourceLogger.Types) > 0 {
		resourceLogger, err = resourcelogger.New(ctx, state.Default(), logger.With(logging.Component("resourcelogger")),
			cfg.Logs.ResourceLogger.GetLogLevel(), cfg.Logs.ResourceLogger.Types...)
		if err != nil {
			return fmt.Errorf("failed to set up resource logger: %w", err)
		}
	}

	imageFactoryClient, err := imagefactory.NewClient(state.Default(), cfg.Registries.GetImageFactoryBaseURL())
	if err != nil {
		return fmt.Errorf("failed to set up image factory client: %w", err)
	}

	linkCounterDeltaCh := make(chan siderolink.LinkCounterDeltas)
	siderolinkEventsCh := make(chan *omnires.MachineStatusSnapshot)
	installEventCh := make(chan resource.ID)

	discoveryClientCache := discovery.NewClientCache(logger.With(logging.Component("discovery_client_factory")))
	defer discoveryClientCache.Close()

	prometheus.MustRegister(discoveryClientCache)

	omniRuntime, err := omni.NewRuntime(cfg, talosClientFactory, dnsService, workloadProxyReconciler, resourceLogger,
		imageFactoryClient, linkCounterDeltaCh, siderolinkEventsCh, installEventCh, state,
		prometheus.DefaultRegisterer, discoveryClientCache, kubernetesRuntime, logger.With(logging.Component("omni_runtime")),
	)
	if err != nil {
		return fmt.Errorf("failed to set up the controller runtime: %w", err)
	}

	runtime.Install(omni.Name, omniRuntime)

	err = user.EnsureInitialResources(ctx, state.Default(), logger, cfg.Auth.InitialUsers)
	if err != nil {
		return fmt.Errorf("failed to write initial user resources to state: %w", err)
	}

	machineMap := siderolink.NewMachineMap(siderolink.NewStateStorage(state.Default()))

	logHandler, err := siderolink.NewLogHandler(
		state.SecondaryStorageDB(),
		machineMap,
		state.Default(),
		&cfg.Logs.Machine,
		logger.With(logging.Component("siderolink_log_handler")),
		siderolink.WithLogHandlerCleanupCallback(state.SQLiteMetrics().CleanupCallback(sqlite.SubsystemMachineLogs)),
	)
	if err != nil {
		return fmt.Errorf("failed to set up log handler: %w", err)
	}

	authConfig, err := auth.EnsureAuthConfigResource(ctx, state.Default(), logger, cfg.Auth)
	if err != nil {
		return fmt.Errorf("failed to write auth parameters to state: %w", err)
	}

	imageFactoryPXEBaseURL, err := cfg.GetImageFactoryPXEBaseURL()
	if err != nil {
		return fmt.Errorf("failed to get image factory PXE base URL: %w", err)
	}

	if err = features.UpdateResources(ctx, state.Default(), logger, features.Params{
		WorkloadProxyEnabled:            cfg.Services.WorkloadProxy.GetEnabled(),
		EmbeddedDiscoveryServiceEnabled: cfg.Services.EmbeddedDiscoveryService.GetEnabled(),
		EtcdBackupTickInterval:          cfg.EtcdBackup.GetTickInterval(),
		EtcdBackupMinInterval:           cfg.EtcdBackup.GetMinInterval(),
		EtcdBackupMaxInterval:           cfg.EtcdBackup.GetMaxInterval(),
		AuditLogPath:                    cfg.Logs.Audit.GetPath(),
		ImageFactoryBaseURL:             cfg.Registries.GetImageFactoryBaseURL(),
		ImageFactoryPXEBaseURL:          imageFactoryPXEBaseURL,
		UserPilotAppToken:               cfg.Account.UserPilot.GetAppToken(),
		StripeEnabled:                   cfg.Logs.Stripe.GetEnabled(),
		StripeMinCommit:                 cfg.Logs.Stripe.GetMinCommit(),
		AccountID:                       cfg.Account.GetId(),
		AccountName:                     cfg.Account.GetName(),
		TalosPreReleaseVersionsEnabled:  cfg.Features.GetEnableTalosPreReleaseVersions(),
	}); err != nil {
		return fmt.Errorf("failed to update features config resources: %w", err)
	}

	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: authres.Enabled(authConfig)})

	server, err := backend.NewServer(
		cfg,
		state,
		dnsService,
		workloadProxyReconciler,
		imageFactoryClient,
		linkCounterDeltaCh,
		siderolinkEventsCh,
		installEventCh,
		omniRuntime,
		logHandler,
		authConfig,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("failed to run server: %w", err)
	}

	return nil
}
