// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package cmd represents the base command.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/cmd/omni/pkg/app"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/version"
)

// Execute builds and runs the root command.
func Execute() error {
	rootCmd, err := buildRootCommand()
	if err != nil {
		return fmt.Errorf("failed to build root command: %w", err)
	}

	return rootCmd.Execute()
}

func buildRootCommand() (*cobra.Command, error) {
	configSchema, schemaErr := config.ParseSchema()
	if schemaErr != nil {
		return nil, fmt.Errorf("failed to parse config schema: %w", schemaErr)
	}

	var (
		rootCmdFlagBinder *FlagBinder
		configPaths       []string
		debug             bool
		flagConfig        = &config.Params{}
	)

	rootCmd := &cobra.Command{
		Use:          "omni",
		Short:        "Omni Kubernetes management platform service",
		Long:         "This executable runs both Omni frontend and Omni backend",
		SilenceUsage: true,
		Version:      version.Tag,
		RunE: func(*cobra.Command, []string) error {
			if err := rootCmdFlagBinder.BindAll(); err != nil {
				return fmt.Errorf("failed to bind flags: %w", err)
			}

			var loggerConfig zap.Config

			if constants.IsDebugBuild {
				loggerConfig = zap.NewDevelopmentConfig()
				loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			} else {
				loggerConfig = zap.NewProductionConfig()
			}

			if !debug {
				loggerConfig.Level.SetLevel(zap.InfoLevel)
			} else {
				loggerConfig.Level.SetLevel(zap.DebugLevel)
			}

			// TODO(Utku): for debugging, revert
			loggerConfig.Level.SetLevel(zap.ErrorLevel)

			logger, err := loggerConfig.Build(
				zap.AddStacktrace(zapcore.FatalLevel), // only print stack traces for fatal errors
			)
			if err != nil {
				return fmt.Errorf("failed to set up logging: %w", err)
			}

			signals := make(chan os.Signal, 1)

			signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// do not use signal.NotifyContext as it doesn't support any ways to log the received signal
			panichandler.Go(func() {
				s := <-signals

				logger.Warn("signal received, stopping Omni", zap.String("signal", s.String()))

				cancel()
			}, logger)

			configs := make([]*config.Params, 0, len(configPaths)+1)

			for _, configPath := range configPaths {
				var fileConfig *config.Params

				if fileConfig, err = config.LoadFromFile(configPath); err != nil {
					return fmt.Errorf("failed to load config from file %q: %w", configPath, err)
				}

				configs = append(configs, fileConfig)
			}

			configs = append(configs, flagConfig) // flags have the highest priority

			config, err := config.Init(logger, configSchema, configs...)
			if err != nil {
				return err
			}

			ctx = actor.MarkContextAsInternalActor(ctx)

			state, err := omni.NewState(ctx, config, logger, prometheus.DefaultRegisterer)
			if err != nil {
				return err
			}

			defer func() {
				if err = state.Close(); err != nil {
					logger.Error("failed to close the state gracefully", zap.Error(err))
				}
			}()

			if err = config.ValidateState(ctx, state.Default()); err != nil {
				return err
			}

			if constants.IsDebugBuild {
				logger.Warn("running debug build")
			}

			return app.Run(ctx, state, config, logger)
		},
	}

	rootCmdFlagBinder = NewFlagBinder(rootCmd, configSchema)

	rootCmd.Flags().StringArrayVar(&configPaths, "config-path", nil, "config file(s) to load, can be specified multiple times, merged in order (flags have highest priority)")
	rootCmd.Flags().BoolVar(&debug, "debug", constants.IsDebugBuild, "enable debug logs.")

	rootCmdFlagBinder.StringVar("account.id", &flagConfig.Account.Id)
	rootCmdFlagBinder.StringVar("account.name", &flagConfig.Account.Name)
	rootCmdFlagBinder.StringVar("account.userPilot.appToken", &flagConfig.Account.UserPilot.AppToken)
	rootCmdFlagBinder.Uint32Var("account.maxRegisteredMachines", &flagConfig.Account.MaxRegisteredMachines)

	defineServiceFlags(rootCmdFlagBinder, flagConfig)
	defineAuthFlags(rootCmd, rootCmdFlagBinder, flagConfig)

	if err := defineLogsFlags(rootCmd, rootCmdFlagBinder, flagConfig); err != nil {
		return nil, fmt.Errorf("failed to define logs flags: %w", err)
	}

	defineStorageFlags(rootCmd, rootCmdFlagBinder, flagConfig)
	defineRegistriesFlags(rootCmdFlagBinder, flagConfig)
	defineFeatureFlags(rootCmdFlagBinder, flagConfig)
	defineDebugFlags(rootCmdFlagBinder, flagConfig)
	defineEtcdBackupsFlags(rootCmd, rootCmdFlagBinder, flagConfig)
	defineEulaFlags(rootCmd, rootCmdFlagBinder, flagConfig)

	return rootCmd, nil
}

func defineServiceFlags(b *FlagBinder, flagConfig *config.Params) {
	// API
	b.StringVar("services.api.endpoint", &flagConfig.Services.Api.Endpoint)
	b.StringVar("services.api.advertisedURL", &flagConfig.Services.Api.AdvertisedURL)
	b.StringVar("services.api.keyFile", &flagConfig.Services.Api.KeyFile)
	b.StringVar("services.api.certFile", &flagConfig.Services.Api.CertFile)

	// Metrics
	b.StringVar("services.metrics.endpoint", &flagConfig.Services.Metrics.Endpoint)

	// KubernetesProxy
	b.StringVar("services.kubernetesProxy.endpoint", &flagConfig.Services.KubernetesProxy.Endpoint)
	b.StringVar("services.kubernetesProxy.advertisedURL", &flagConfig.Services.KubernetesProxy.AdvertisedURL)
	b.StringVar("services.kubernetesProxy.oidcCacheBaseDir", &flagConfig.Services.KubernetesProxy.OidcCacheBaseDir)
	b.BoolVar("services.kubernetesProxy.oidcCacheIsolation", &flagConfig.Services.KubernetesProxy.OidcCacheIsolation)

	// Siderolink
	b.StringVar("services.siderolink.wireGuard.endpoint", &flagConfig.Services.Siderolink.WireGuard.Endpoint)
	b.StringVar("services.siderolink.wireGuard.advertisedEndpoint", &flagConfig.Services.Siderolink.WireGuard.AdvertisedEndpoint)
	b.BoolVar("services.siderolink.disableLastEndpoint", &flagConfig.Services.Siderolink.DisableLastEndpoint)
	b.BoolVar("services.siderolink.useGRPCTunnel", &flagConfig.Services.Siderolink.UseGRPCTunnel)
	b.IntVar("services.siderolink.eventSinkPort", &flagConfig.Services.Siderolink.EventSinkPort)
	b.IntVar("services.siderolink.logServerPort", &flagConfig.Services.Siderolink.LogServerPort)
	EnumVar(b, "services.siderolink.joinTokensMode", &flagConfig.Services.Siderolink.JoinTokensMode)
	b.Uint64Var("services.siderolink.bandwidthLimitMbps", &flagConfig.Services.Siderolink.BandwidthLimitMbps)
	b.Uint64Var("services.siderolink.bandwidthLimitBurstBytes", &flagConfig.Services.Siderolink.BandwidthLimitBurstBytes)

	// MachineAPI — deprecated aliases registered first so canonical flags take precedence in BindAll()
	b.deprecatedStringAlias("siderolink-api-bind-addr", "use --machine-api-bind-addr", &flagConfig.Services.MachineAPI.Endpoint)
	b.deprecatedStringAlias("siderolink-api-cert", "use --machine-api-cert", &flagConfig.Services.MachineAPI.CertFile)
	b.deprecatedStringAlias("siderolink-api-key", "use --machine-api-key", &flagConfig.Services.MachineAPI.KeyFile)
	b.deprecatedStringAlias("siderolink-api-advertised-url", "use --machine-api-advertised-url", &flagConfig.Services.MachineAPI.AdvertisedURL)

	b.StringVar("services.machineAPI.endpoint", &flagConfig.Services.MachineAPI.Endpoint)
	b.StringVar("services.machineAPI.certFile", &flagConfig.Services.MachineAPI.CertFile)
	b.StringVar("services.machineAPI.keyFile", &flagConfig.Services.MachineAPI.KeyFile)
	b.StringVar("services.machineAPI.advertisedURL", &flagConfig.Services.MachineAPI.AdvertisedURL)

	// LoadBalancer
	b.IntVar("services.loadBalancer.minPort", &flagConfig.Services.LoadBalancer.MinPort)
	b.IntVar("services.loadBalancer.maxPort", &flagConfig.Services.LoadBalancer.MaxPort)

	// LocalResourceService
	b.IntVar("services.localResourceService.port", &flagConfig.Services.LocalResourceService.Port)
	b.BoolVar("services.localResourceService.enabled", &flagConfig.Services.LocalResourceService.Enabled)

	// Embedded discovery service
	b.BoolVar("services.embeddedDiscoveryService.enabled", &flagConfig.Services.EmbeddedDiscoveryService.Enabled)
	b.deprecatedIntAlias("embedded-discovery-service-endpoint", "use --embedded-discovery-service-port", &flagConfig.Services.EmbeddedDiscoveryService.Port)
	b.IntVar("services.embeddedDiscoveryService.port", &flagConfig.Services.EmbeddedDiscoveryService.Port)
	b.BoolVar("services.embeddedDiscoveryService.snapshotsEnabled", &flagConfig.Services.EmbeddedDiscoveryService.SnapshotsEnabled)
	b.DurationVar("services.embeddedDiscoveryService.snapshotsInterval", &flagConfig.Services.EmbeddedDiscoveryService.SnapshotsInterval)
	b.StringVar("services.embeddedDiscoveryService.logLevel", &flagConfig.Services.EmbeddedDiscoveryService.LogLevel)
	b.DurationVar("services.embeddedDiscoveryService.sqliteTimeout", &flagConfig.Services.EmbeddedDiscoveryService.SqliteTimeout)

	// DevServerProxy
	b.StringVar("services.devServerProxy.proxyTo", &flagConfig.Services.DevServerProxy.ProxyTo)
	b.StringVar("services.devServerProxy.endpoint", &flagConfig.Services.DevServerProxy.Endpoint)
}

func defineAuthFlags(rootCmd *cobra.Command, b *FlagBinder, flagConfig *config.Params) {
	// Auth0
	b.BoolVar("auth.auth0.enabled", &flagConfig.Auth.Auth0.Enabled)
	b.StringVar("auth.auth0.clientID", &flagConfig.Auth.Auth0.ClientID)
	b.StringVar("auth.auth0.domain", &flagConfig.Auth.Auth0.Domain)
	b.BoolVar("auth.auth0.useFormData", &flagConfig.Auth.Auth0.UseFormData)

	// Webauthn
	b.BoolVar("auth.webauthn.enabled", &flagConfig.Auth.Webauthn.Enabled)
	b.BoolVar("auth.webauthn.required", &flagConfig.Auth.Webauthn.Required)

	// SAML
	b.BoolVar("auth.saml.enabled", &flagConfig.Auth.Saml.Enabled)
	b.StringVar("auth.saml.url", &flagConfig.Auth.Saml.Url)
	b.StringVar("auth.saml.metadata", &flagConfig.Auth.Saml.Metadata)
	b.ValueVar("auth.saml.labelRules", &flagConfig.Auth.Saml.LabelRules)
	b.ValueVar("auth.saml.attributeRules", &flagConfig.Auth.Saml.AttributeRules)
	b.StringVar("auth.saml.nameIDFormat", &flagConfig.Auth.Saml.NameIDFormat)

	b.StringSliceVar("auth.initialUsers", &flagConfig.Auth.InitialUsers, flagConfig.Auth.InitialUsers)

	b.DurationVar("auth.keyPruner.interval", &flagConfig.Auth.KeyPruner.Interval)
	b.BoolVar("auth.suspended", &flagConfig.Auth.Suspended)

	// Initial service account
	b.BoolVar("auth.initialServiceAccount.enabled", &flagConfig.Auth.InitialServiceAccount.Enabled)
	b.StringVar("auth.initialServiceAccount.keyPath", &flagConfig.Auth.InitialServiceAccount.KeyPath)
	b.StringVar("auth.initialServiceAccount.role", &flagConfig.Auth.InitialServiceAccount.Role)
	b.DurationVar("auth.initialServiceAccount.lifetime", &flagConfig.Auth.InitialServiceAccount.Lifetime)

	rootCmd.MarkFlagsMutuallyExclusive(b.mustFlagName("auth.saml.url"), b.mustFlagName("auth.saml.metadata"))

	// Limits
	b.Uint32Var("auth.limits.maxUsers", &flagConfig.Auth.Limits.MaxUsers)
	b.Uint32Var("auth.limits.maxServiceAccounts", &flagConfig.Auth.Limits.MaxServiceAccounts)

	// OIDC
	b.BoolVar("auth.oidc.enabled", &flagConfig.Auth.Oidc.Enabled)
	b.StringVar("auth.oidc.providerURL", &flagConfig.Auth.Oidc.ProviderURL)
	b.StringVar("auth.oidc.clientID", &flagConfig.Auth.Oidc.ClientID)
	b.StringVar("auth.oidc.clientSecret", &flagConfig.Auth.Oidc.ClientSecret)
	b.StringSliceVar("auth.oidc.scopes", &flagConfig.Auth.Oidc.Scopes, flagConfig.Auth.Oidc.Scopes)
	b.StringVar("auth.oidc.logoutURL", &flagConfig.Auth.Oidc.LogoutURL)
	b.BoolVar("auth.oidc.allowUnverifiedEmail", &flagConfig.Auth.Oidc.AllowUnverifiedEmail)
}

func defineLogsFlags(rootCmd *cobra.Command, b *FlagBinder, flagConfig *config.Params) error {
	b.DurationVar("logs.machine.storage.sqliteTimeout", &flagConfig.Logs.Machine.Storage.SqliteTimeout)
	b.DurationVar("logs.machine.storage.cleanupInterval", &flagConfig.Logs.Machine.Storage.CleanupInterval)
	b.DurationVar("logs.machine.storage.cleanupOlderThan", &flagConfig.Logs.Machine.Storage.CleanupOlderThan)
	b.IntVar("logs.machine.storage.maxLinesPerMachine", &flagConfig.Logs.Machine.Storage.MaxLinesPerMachine)
	b.Uint64Var("logs.machine.storage.maxSize", &flagConfig.Logs.Machine.Storage.MaxSize)
	b.Float64Var("logs.machine.storage.cleanupProbability", &flagConfig.Logs.Machine.Storage.CleanupProbability)

	if err := rootCmd.Flags().MarkHidden(b.mustFlagName("logs.machine.storage.cleanupProbability")); err != nil {
		return err
	}

	b.StringSliceVar("logs.resourceLogger.types", &flagConfig.Logs.ResourceLogger.Types, flagConfig.Logs.ResourceLogger.Types)
	b.StringVar("logs.resourceLogger.logLevel", &flagConfig.Logs.ResourceLogger.LogLevel)
	b.BoolVar("logs.audit.enabled", &flagConfig.Logs.Audit.Enabled)
	b.DurationVar("logs.audit.sqliteTimeout", &flagConfig.Logs.Audit.SqliteTimeout)
	b.DurationVar("logs.audit.retentionPeriod", &flagConfig.Logs.Audit.RetentionPeriod)
	b.Uint64Var("logs.audit.maxSize", &flagConfig.Logs.Audit.MaxSize)
	b.Float64Var("logs.audit.cleanupProbability", &flagConfig.Logs.Audit.CleanupProbability)
	b.BoolVar("logs.stripe.enabled", &flagConfig.Logs.Stripe.Enabled)
	b.Uint32Var("logs.stripe.minCommit", &flagConfig.Logs.Stripe.MinCommit)

	if err := rootCmd.Flags().MarkHidden(b.mustFlagName("logs.audit.cleanupProbability")); err != nil {
		return err
	}

	return nil
}

func defineStorageFlags(rootCmd *cobra.Command, b *FlagBinder, flagConfig *config.Params) {
	EnumVar(b, "storage.default.kind", &flagConfig.Storage.Default.Kind)
	b.BoolVar("storage.default.etcd.embedded", &flagConfig.Storage.Default.Etcd.Embedded)
	b.BoolVar("storage.default.etcd.embeddedUnsafeFsync", &flagConfig.Storage.Default.Etcd.EmbeddedUnsafeFsync)
	b.StringVar("storage.default.etcd.embeddedDBPath", &flagConfig.Storage.Default.Etcd.EmbeddedDBPath)

	ensure.NoError(rootCmd.Flags().MarkHidden(b.mustFlagName("storage.default.etcd.embeddedUnsafeFsync")))

	b.StringSliceVar("storage.default.etcd.endpoints", &flagConfig.Storage.Default.Etcd.Endpoints, flagConfig.Storage.Default.Etcd.Endpoints)
	b.DurationVar("storage.default.etcd.dialKeepAliveTime", &flagConfig.Storage.Default.Etcd.DialKeepAliveTime)
	b.DurationVar("storage.default.etcd.dialKeepAliveTimeout", &flagConfig.Storage.Default.Etcd.DialKeepAliveTimeout)
	b.StringVar("storage.default.etcd.caFile", &flagConfig.Storage.Default.Etcd.CaFile)
	b.StringVar("storage.default.etcd.certFile", &flagConfig.Storage.Default.Etcd.CertFile)
	b.StringVar("storage.default.etcd.keyFile", &flagConfig.Storage.Default.Etcd.KeyFile)
	b.StringVar("storage.default.etcd.privateKeySource", &flagConfig.Storage.Default.Etcd.PrivateKeySource)

	b.StringSliceVar("storage.default.etcd.publicKeyFiles", &flagConfig.Storage.Default.Etcd.PublicKeyFiles, flagConfig.Storage.Default.Etcd.PublicKeyFiles)
	b.StringVar("storage.sqlite.path", &flagConfig.Storage.Sqlite.Path)
	b.StringVar("storage.sqlite.experimentalBaseParams", &flagConfig.Storage.Sqlite.ExperimentalBaseParams)
	b.StringVar("storage.sqlite.extraParams", &flagConfig.Storage.Sqlite.ExtraParams)
	b.DurationVar("storage.sqlite.metrics.refreshInterval", &flagConfig.Storage.Sqlite.Metrics.RefreshInterval)
	b.DurationVar("storage.sqlite.metrics.refreshTimeout", &flagConfig.Storage.Sqlite.Metrics.RefreshTimeout)
	b.StringVar("storage.vault.k8sAuthMountPath", &flagConfig.Storage.Vault.K8SAuthMountPath)
}

func defineRegistriesFlags(b *FlagBinder, flagConfig *config.Params) {
	b.StringVar("registries.talos", &flagConfig.Registries.Talos)
	b.StringVar("registries.kubernetes", &flagConfig.Registries.Kubernetes)
	b.StringVar("registries.imageFactoryBaseURL", &flagConfig.Registries.ImageFactoryBaseURL)
	b.StringVar("registries.imageFactoryPXEBaseURL", &flagConfig.Registries.ImageFactoryPXEBaseURL)

	b.StringSliceVar("registries.mirrors", &flagConfig.Registries.Mirrors, flagConfig.Registries.Mirrors)
}

func defineFeatureFlags(b *FlagBinder, flagConfig *config.Params) {
	b.BoolVar("features.enableTalosPreReleaseVersions", &flagConfig.Features.EnableTalosPreReleaseVersions)
	b.BoolVar("features.enableConfigDataCompression", &flagConfig.Features.EnableConfigDataCompression)
	b.BoolVar("services.workloadProxy.enabled", &flagConfig.Services.WorkloadProxy.Enabled)
	b.StringVar("services.workloadProxy.subdomain", &flagConfig.Services.WorkloadProxy.Subdomain)
	b.BoolVar("services.workloadProxy.useOmniSubdomain", &flagConfig.Services.WorkloadProxy.UseOmniSubdomain)
	b.DurationVar("services.workloadProxy.stopLBsAfter", &flagConfig.Services.WorkloadProxy.StopLBsAfter)
	b.BoolVar("features.enableBreakGlassConfigs", &flagConfig.Features.EnableBreakGlassConfigs)
	b.BoolVar("features.disableControllerRuntimeCache", &flagConfig.Features.DisableControllerRuntimeCache)
	b.BoolVar("features.enableClusterImport", &flagConfig.Features.EnableClusterImport)
}

func defineDebugFlags(b *FlagBinder, flagConfig *config.Params) {
	b.StringVar("debug.pprof.endpoint", &flagConfig.Debug.Pprof.Endpoint)
	b.StringVar("debug.server.endpoint", &flagConfig.Debug.Server.Endpoint)
}

func defineEtcdBackupsFlags(rootCmd *cobra.Command, b *FlagBinder, flagConfig *config.Params) {
	b.BoolVar("etcdBackup.s3Enabled", &flagConfig.EtcdBackup.S3Enabled)
	b.StringVar("etcdBackup.localPath", &flagConfig.EtcdBackup.LocalPath)
	b.DurationVar("etcdBackup.tickInterval", &flagConfig.EtcdBackup.TickInterval)
	b.DurationVar("etcdBackup.jitter", &flagConfig.EtcdBackup.Jitter)
	b.DurationVar("etcdBackup.minInterval", &flagConfig.EtcdBackup.MinInterval)
	b.DurationVar("etcdBackup.maxInterval", &flagConfig.EtcdBackup.MaxInterval)
	b.Uint64Var("etcdBackup.uploadLimitMbps", &flagConfig.EtcdBackup.UploadLimitMbps)
	b.Uint64Var("etcdBackup.downloadLimitMbps", &flagConfig.EtcdBackup.DownloadLimitMbps)

	rootCmd.MarkFlagsMutuallyExclusive(b.mustFlagName("etcdBackup.s3Enabled"), b.mustFlagName("etcdBackup.localPath"))
}

func defineEulaFlags(rootCmd *cobra.Command, b *FlagBinder, flagConfig *config.Params) {
	b.StringVar("eulaAccept.name", &flagConfig.EulaAccept.Name)
	b.StringVar("eulaAccept.email", &flagConfig.EulaAccept.Email)

	rootCmd.MarkFlagsRequiredTogether(b.mustFlagName("eulaAccept.name"), b.mustFlagName("eulaAccept.email"))
}
