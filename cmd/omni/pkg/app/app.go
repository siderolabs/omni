// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
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

// PrepareConfig prepare the Omni configuration.
func PrepareConfig(logger *zap.Logger, params ...*config.Params) (*config.Params, error) {
	var err error

	config.Config, err = config.Init(logger, params...)
	if err != nil {
		return nil, err
	}

	return config.Config, nil
}

// Run the Omni service.
func Run(ctx context.Context, state *omni.State, config *config.Params, logger *zap.Logger) error {
	grpc_prometheus.EnableHandlingTimeHistogram(grpc_prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 1, 10, 30, 60, 120, 300, 600}))

	logger.Debug("using config", zap.Any("config", config))

	klog.SetLogger(zapr.NewLogger(logger.WithOptions(zap.IncreaseLevel(zapcore.WarnLevel)).With(logging.Component("kubernetes"))))

	ctx = actor.MarkContextAsInternalActor(ctx)

	talosClientFactory := talos.NewClientFactory(state.Default(), logger)
	prometheus.MustRegister(talosClientFactory)

	dnsService := dns.NewService(state.Default(), logger)
	workloadProxyReconciler := workloadproxy.NewReconciler(logger.With(logging.Component("workload_proxy_reconciler")),
		zapcore.DebugLevel, config.Services.WorkloadProxy.StopLBsAfter)

	var (
		resourceLogger *resourcelogger.Logger
		err            error
	)

	if len(config.Logs.ResourceLogger.Types) > 0 {
		resourceLogger, err = resourcelogger.New(ctx, state.Default(), logger.With(logging.Component("resourcelogger")),
			config.Logs.ResourceLogger.LogLevel, config.Logs.ResourceLogger.Types...)
		if err != nil {
			return fmt.Errorf("failed to set up resource logger: %w", err)
		}
	}

	imageFactoryClient, err := imagefactory.NewClient(state.Default(), config.Registries.ImageFactoryBaseURL)
	if err != nil {
		return fmt.Errorf("failed to set up image factory client: %w", err)
	}

	linkCounterDeltaCh := make(chan siderolink.LinkCounterDeltas)
	siderolinkEventsCh := make(chan *omnires.MachineStatusSnapshot)
	installEventCh := make(chan resource.ID)

	discoveryClientCache := discovery.NewClientCache(logger.With(logging.Component("discovery_client_factory")))
	defer discoveryClientCache.Close()

	prometheus.MustRegister(discoveryClientCache)

	omniRuntime, err := omni.NewRuntime(talosClientFactory, dnsService, workloadProxyReconciler, resourceLogger,
		imageFactoryClient, linkCounterDeltaCh, siderolinkEventsCh, installEventCh, state,
		prometheus.DefaultRegisterer, discoveryClientCache, logger.With(logging.Component("omni_runtime")),
	)
	if err != nil {
		return fmt.Errorf("failed to set up the controller runtime: %w", err)
	}

	talosRuntime := talos.New(talosClientFactory, logger)

	err = user.EnsureInitialResources(ctx, state.Default(), logger, config.Auth.InitialUsers)
	if err != nil {
		return fmt.Errorf("failed to write initial user resources to state: %w", err)
	}

	machineMap := siderolink.NewMachineMap(siderolink.NewStateStorage(state.Default()))

	logHandler, err := siderolink.NewLogHandler(
		machineMap,
		state.Default(),
		&config.Logs.Machine,
		logger.With(logging.Component("siderolink_log_handler")),
	)
	if err != nil {
		return fmt.Errorf("failed to set up log handler: %w", err)
	}

	authConfig, err := auth.EnsureAuthConfigResource(ctx, state.Default(), logger, config.Auth)
	if err != nil {
		return fmt.Errorf("failed to write auth parameters to state: %w", err)
	}

	if err = features.UpdateResources(ctx, state.Default(), logger); err != nil {
		return fmt.Errorf("failed to update features config resources: %w", err)
	}

	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: authres.Enabled(authConfig)})

	server, err := backend.NewServer(
		state,
		dnsService,
		workloadProxyReconciler,
		imageFactoryClient,
		linkCounterDeltaCh,
		siderolinkEventsCh,
		installEventCh,
		omniRuntime,
		talosRuntime,
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
