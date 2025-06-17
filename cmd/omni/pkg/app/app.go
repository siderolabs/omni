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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/go-debug"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend"
	"github.com/siderolabs/omni/internal/backend/discovery"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/resourcelogger"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
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

func runDebugServer(ctx context.Context, logger *zap.Logger, bindEndpoint string) {
	debugLogFunc := func(msg string) {
		logger.Info(msg)
	}

	if err := debug.ListenAndServe(ctx, bindEndpoint, debugLogFunc); err != nil {
		logger.Panic("failed to start debug server", zap.Error(err))
	}
}

// Run the Omni service.
func Run(ctx context.Context, cfg *config.Params, logger *zap.Logger) error {
	if cfg.Debug.Server.Endpoint != "" {
		panichandler.Go(func() {
			runDebugServer(ctx, logger, cfg.Debug.Server.Endpoint)
		}, logger)
	}

	// this global context propagates into all controllers and any other background activities
	ctx = actor.MarkContextAsInternalActor(ctx)

	err := omni.NewState(ctx, config.Config, logger, prometheus.DefaultRegisterer, runWithState(logger))
	if err != nil {
		return fmt.Errorf("failed to run Omni: %w", err)
	}

	return nil
}

func runWithState(logger *zap.Logger) func(context.Context, state.State, *virtual.State) error {
	return func(ctx context.Context, resourceState state.State, virtualState *virtual.State) error {
		auditWrap, auditErr := omni.NewAuditWrap(resourceState, config.Config, logger)
		if auditErr != nil {
			return auditErr
		}

		resourceState = auditWrap.WrapState(resourceState)

		talosClientFactory := talos.NewClientFactory(resourceState, logger)
		prometheus.MustRegister(talosClientFactory)

		dnsService := dns.NewService(resourceState, logger)
		workloadProxyReconciler := workloadproxy.NewReconciler(logger.With(logging.Component("workload_proxy_reconciler")), zapcore.DebugLevel)

		var resourceLogger *resourcelogger.Logger

		if len(config.Config.Logs.ResourceLogger.Types) > 0 {
			var err error

			resourceLogger, err = resourcelogger.New(ctx, resourceState, logger.With(logging.Component("resourcelogger")),
				config.Config.Logs.ResourceLogger.LogLevel, config.Config.Logs.ResourceLogger.Types...)
			if err != nil {
				return fmt.Errorf("failed to set up resource logger: %w", err)
			}
		}

		imageFactoryClient, err := imagefactory.NewClient(resourceState, config.Config.Registries.ImageFactoryBaseURL)
		if err != nil {
			return fmt.Errorf("failed to set up image factory client: %w", err)
		}

		linkCounterDeltaCh := make(chan siderolink.LinkCounterDeltas)
		siderolinkEventsCh := make(chan *omnires.MachineStatusSnapshot)
		installEventCh := make(chan resource.ID)

		discoveryClientCache := discovery.NewClientCache(logger.With(logging.Component("discovery_client_factory")))
		defer discoveryClientCache.Close()

		prometheus.MustRegister(discoveryClientCache)

		omniRuntime, err := omni.New(talosClientFactory, dnsService, workloadProxyReconciler, resourceLogger,
			imageFactoryClient, linkCounterDeltaCh, siderolinkEventsCh, installEventCh, resourceState, virtualState,
			prometheus.DefaultRegisterer, discoveryClientCache, logger.With(logging.Component("omni_runtime")))
		if err != nil {
			return fmt.Errorf("failed to set up the controller runtime: %w", err)
		}

		machineMap := siderolink.NewMachineMap(siderolink.NewStateStorage(omniRuntime.State()))

		logHandler, err := siderolink.NewLogHandler(
			machineMap,
			resourceState,
			&config.Config.Logs.Machine,
			logger.With(logging.Component("siderolink_log_handler")),
		)
		if err != nil {
			return fmt.Errorf("failed to set up log handler: %w", err)
		}

		talosRuntime := talos.New(talosClientFactory, logger)

		err = user.EnsureInitialResources(ctx, omniRuntime.State(), logger, config.Config.Auth.Auth0.InitialUsers)
		if err != nil {
			return fmt.Errorf("failed to write initial user resources to state: %w", err)
		}

		authConfig, err := auth.EnsureAuthConfigResource(ctx, omniRuntime.State(), logger, config.Config.Auth)
		if err != nil {
			return fmt.Errorf("failed to write Auth0 parameters to state: %w", err)
		}

		if err = features.UpdateResources(ctx, omniRuntime.State(), logger); err != nil {
			return fmt.Errorf("failed to update features config resources: %w", err)
		}

		ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: authres.Enabled(authConfig)})

		server, err := backend.NewServer(
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
			auditWrap,
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
}
