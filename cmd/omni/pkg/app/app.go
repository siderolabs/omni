// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package app contains the Omni startup methods.
package app

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/go-logr/zapr"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend"
	"github.com/siderolabs/omni/internal/backend/discovery"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/resourcelogger"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/user"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	"github.com/siderolabs/omni/internal/pkg/eula"
	"github.com/siderolabs/omni/internal/pkg/features"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// Run the Omni service.
func Run(ctx context.Context, state *omni.State, cfg *config.Params, logger *zap.Logger) error {
	talosClientFactory := talos.NewClientFactory(state.Default(), logger)
	talosRuntime := talos.New(talosClientFactory, logger, cfg.Account.GetName(), cfg.Services.Api.URL())

	// Wire controller-runtime logging to the global zap logger instead of discarding it.
	log.SetLogger(zapr.NewLogger(logger))

	oidcIssuerEndpoint, err := cfg.GetOIDCIssuerEndpoint()
	if err != nil {
		return fmt.Errorf("failed to get OIDC issuer endpoint: %w", err)
	}

	kubernetesRuntime := kubernetes.New(state.Default(), logger, oidcIssuerEndpoint, cfg.Account.GetName(), cfg.Services.KubernetesProxy.URL())

	prometheus.MustRegister(talosClientFactory)
	prometheus.MustRegister(kubernetesRuntime)

	grpc_prometheus.EnableHandlingTimeHistogram(grpc_prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 1, 10, 30, 60, 120, 300, 600}))

	logger.Debug("using config", zap.Any("config", cfg))

	klog.SetLogger(zapr.NewLogger(logging.IncreaseLevel(logger, zapcore.WarnLevel).With(logging.Component("kubernetes"))))

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

	primaryFactory := cfg.Registries.GetPrimaryFactory()

	imageFactoryClients, err := setupImageFactoryClients(cfg, state)
	if err != nil {
		return err
	}

	linkCounterDeltaCh := make(chan siderolink.LinkCounterDeltas)
	siderolinkEventsCh := make(chan *omnires.MachineStatusSnapshot)
	installEventCh := make(chan resource.ID)

	discoveryClientCache := discovery.NewClientCache(logger.With(logging.Component("discovery_client_factory")))
	defer discoveryClientCache.Close()

	prometheus.MustRegister(discoveryClientCache)

	lifecycleManager := lifecycle.NewManager(
		logger.With(logging.Component("talos_lifecycle")),
		imageFactoryClients,
		cfg.Registries.GetTalos(),
		kubernetesRuntime,
		talosClientFactory,
		installEventCh,
	)

	omniRuntime, err := omni.NewRuntime(
		cfg, talosClientFactory, dnsService, workloadProxyReconciler, resourceLogger,
		imageFactoryClients, linkCounterDeltaCh, siderolinkEventsCh, installEventCh, state,
		prometheus.DefaultRegisterer, discoveryClientCache, kubernetesRuntime, talosRuntime,
		lifecycleManager, logger.With(logging.Component("omni_runtime")),
	)
	if err != nil {
		return fmt.Errorf("failed to set up the controller runtime: %w", err)
	}

	err = user.EnsureInitialResources(ctx, state.Default(), logger, cfg.Auth.InitialUsers)
	if err != nil {
		return fmt.Errorf("failed to write initial user resources to state: %w", err)
	}

	err = user.ElevateRecoveryAdmin(ctx, state.Default(), logger, cfg.Auth.GetRecoveryAdmin())
	if err != nil {
		return fmt.Errorf("failed to elevate the recovery admin: %w", err)
	}

	if cfg.EulaAccept.GetName() != "" && cfg.EulaAccept.GetEmail() != "" {
		if err = eula.Accept(ctx, state.Default(), eula.AcceptParams{Name: cfg.EulaAccept.GetName(), Email: cfg.EulaAccept.GetEmail()}); err != nil {
			return fmt.Errorf("failed to accept EULA: %w", err)
		}
	}

	machineMap := siderolink.NewMachineMap(siderolink.NewStateStorage(state.Default()))

	logIngestionLimiter := siderolink.NewLogIngestionLimiter(
		cfg.Logs.Machine.GetIngestionRateLimitBytesPerSecond(),
		cfg.Logs.Machine.GetIngestionRateBurstBytes(),
		logger,
	)
	if logIngestionLimiter != nil {
		prometheus.MustRegister(logIngestionLimiter)
	}

	logHandler, err := siderolink.NewLogHandler(
		state.SecondaryStorageDB(),
		machineMap,
		state.Default(),
		&cfg.Logs.Machine,
		logger.With(logging.Component("siderolink_log_handler")),
		siderolink.WithLogHandlerCleanupCallback(state.SQLiteMetrics().CleanupCallback(sqlite.SubsystemMachineLogs)),
		siderolink.WithLogIngestionLimiter(logIngestionLimiter),
	)
	if err != nil {
		return fmt.Errorf("failed to set up log handler: %w", err)
	}

	prometheus.MustRegister(logHandler)

	authConfig, err := auth.EnsureAuthConfigResource(ctx, state.Default(), logger, cfg.Auth)
	if err != nil {
		return fmt.Errorf("failed to write auth parameters to state: %w", err)
	}

	if err = ensureImageFactoryAuthResources(ctx, state, cfg); err != nil {
		return fmt.Errorf("failed to ensure ImageFactoryAuth resources: %w", err)
	}

	factoryURLs, err := resolveFactoryURLs(&cfg.Registries)
	if err != nil {
		return err
	}

	if err = features.UpdateResources(ctx, state.Default(), logger, features.Params{
		WorkloadProxyEnabled:            cfg.Services.WorkloadProxy.GetEnabled(),
		EmbeddedDiscoveryServiceEnabled: cfg.Services.EmbeddedDiscoveryService.GetEnabled(),
		EtcdBackupTickInterval:          cfg.EtcdBackup.GetTickInterval(),
		EtcdBackupMinInterval:           cfg.EtcdBackup.GetMinInterval(),
		EtcdBackupMaxInterval:           cfg.EtcdBackup.GetMaxInterval(),
		AuditLogEnabled:                 cfg.Logs.Audit.GetEnabled(),
		ImageFactoryBaseURL:             primaryFactory.GetUrl(),
		ImageFactoryPXEBaseURL:          factoryURLs.primaryPXE,
		SecondaryImageFactoryBaseURL:    factoryURLs.secondaryBase,
		SecondaryImageFactoryPXEBaseURL: factoryURLs.secondaryPXE,
		UserPilotAppToken:               cfg.Account.UserPilot.GetAppToken(),
		PosthogAPIKey:                   cfg.Account.Posthog.GetApiKey(),
		PosthogAPIHost:                  cfg.Account.Posthog.GetApiHost(),
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
		imageFactoryClients,
		linkCounterDeltaCh,
		siderolinkEventsCh,
		installEventCh,
		omniRuntime,
		logHandler,
		authConfig,
		logger,
		kubernetesRuntime,
		talosRuntime,
		lifecycleManager,
	)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("failed to run server: %w", err)
	}

	return nil
}

// ensureImageFactoryAuthResources creates an ImageFactoryAuth resource for each configured factory that
// has credentials, and prunes the resources of the factories which are no longer configured.
func ensureImageFactoryAuthResources(ctx context.Context, state *omni.State, cfg *config.Params) error {
	factories := []config.Factory{cfg.Registries.GetPrimaryFactory()}

	if secondary, ok := cfg.Registries.GetSecondaryFactory(); ok {
		factories = append(factories, secondary)
	}

	visited := map[string]struct{}{}

	for _, factory := range factories {
		if factory.GetUsername() == "" || factory.GetPassword() == "" {
			continue
		}

		res := omnires.NewImageFactoryAuth(factory.GetUrl())

		visited[res.Metadata().ID()] = struct{}{}

		if err := safe.StateModify(ctx, state.Default(), res, func(res *omnires.ImageFactoryAuth) error {
			res.TypedSpec().Value.Username = factory.GetUsername()
			res.TypedSpec().Value.Password = factory.GetPassword()

			return nil
		}); err != nil {
			return fmt.Errorf("failed to modify ImageFactoryAuth resource: %w", err)
		}
	}

	auths, err := safe.ReaderListAll[*omnires.ImageFactoryAuth](ctx, state.Default())
	if err != nil {
		return fmt.Errorf("failed to list ImageFactoryAuth resources: %w", err)
	}

	for auth := range auths.All() {
		if _, ok := visited[auth.Metadata().ID()]; !ok {
			if err := state.Default().Destroy(ctx, auth.Metadata()); err != nil {
				return fmt.Errorf("failed to delete ImageFactoryAuth resource %s: %w", auth.Metadata().ID(), err)
			}
		}
	}

	return nil
}

// factoryURLs holds the image factory endpoints published to the frontend via FeaturesConfig.
type factoryURLs struct {
	primaryPXE    *url.URL
	secondaryBase string
	secondaryPXE  string
}

// resolveFactoryURLs derives the PXE endpoints of the configured image factories and validates that
// the primary and the secondary factory are actually distinct.
func resolveFactoryURLs(registries *config.Registries) (factoryURLs, error) {
	primary := registries.GetPrimaryFactory()

	primaryPXE, err := primary.PXEBaseURL()
	if err != nil {
		return factoryURLs{}, fmt.Errorf("failed to get image factory PXE base URL: %w", err)
	}

	res := factoryURLs{primaryPXE: primaryPXE}

	secondary, ok := registries.GetSecondaryFactory()
	if !ok {
		return res, nil
	}

	if strings.TrimRight(primary.GetUrl(), "/") == strings.TrimRight(secondary.GetUrl(), "/") {
		return factoryURLs{}, fmt.Errorf("primary and secondary image factory URLs cannot be the same: %q", primary.GetUrl())
	}

	secondaryPXE, err := secondary.PXEBaseURL()
	if err != nil {
		return factoryURLs{}, fmt.Errorf("failed to get secondary image factory PXE base URL: %w", err)
	}

	res.secondaryBase = secondary.GetUrl()
	res.secondaryPXE = secondaryPXE.String()

	return res, nil
}

// setupImageFactoryClients builds the image factory clients for the primary and (optional) secondary factories.
func setupImageFactoryClients(cfg *config.Params, state *omni.State) (*imagefactory.Clients, error) {
	primaryFactory := cfg.Registries.GetPrimaryFactory()

	imageFactoryClient, err := imagefactory.NewClient(
		primaryFactory.GetUrl(),
		primaryFactory.GetUsername(),
		primaryFactory.GetPassword(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set up image factory client: %w", err)
	}

	clients := imagefactory.NewClients(
		state.Default(),
		imageFactoryClient,
	)

	if secondaryFactory, ok := cfg.Registries.GetSecondaryFactory(); ok {
		var secondaryFactoryClient *imagefactory.Client

		secondaryFactoryClient, err = imagefactory.NewClient(
			secondaryFactory.GetUrl(),
			secondaryFactory.GetUsername(),
			secondaryFactory.GetPassword(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to set up secondary image factory client: %w", err)
		}

		clients.SetSecondary(secondaryFactoryClient)
	}

	return clients, nil
}
