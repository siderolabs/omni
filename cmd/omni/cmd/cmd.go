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
	"strings"
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
	"github.com/siderolabs/omni/internal/pkg/jsonschema"
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

	rootCmdFlagBinder = NewFlagBinder(rootCmd)

	rootCmd.Flags().StringArrayVar(&configPaths, "config-path", nil, "config file(s) to load, can be specified multiple times, merged in order (flags have highest priority)")
	rootCmd.Flags().BoolVar(&debug, "debug", constants.IsDebugBuild, "enable debug logs.")

	rootCmdFlagBinder.StringVar("account-id", flagDescription("account.id", configSchema), &flagConfig.Account.Id)
	rootCmdFlagBinder.StringVar("name", flagDescription("account.name", configSchema), &flagConfig.Account.Name)
	rootCmdFlagBinder.StringVar("user-pilot-app-token", flagDescription("account.userPilot.appToken", configSchema), &flagConfig.Account.UserPilot.AppToken)

	if err := defineServiceFlags(rootCmd, rootCmdFlagBinder, flagConfig, configSchema); err != nil {
		return nil, fmt.Errorf("failed to define service flags: %w", err)
	}

	defineAuthFlags(rootCmd, rootCmdFlagBinder, flagConfig, configSchema)

	if err := defineLogsFlags(rootCmd, rootCmdFlagBinder, flagConfig, configSchema); err != nil {
		return nil, fmt.Errorf("failed to define logs flags: %w", err)
	}

	if err := defineStorageFlags(rootCmd, rootCmdFlagBinder, flagConfig, configSchema); err != nil {
		return nil, fmt.Errorf("failed to define storage flags: %w", err)
	}

	defineRegistriesFlags(rootCmd, rootCmdFlagBinder, flagConfig, configSchema)
	defineFeatureFlags(rootCmdFlagBinder, flagConfig, configSchema)
	defineDebugFlags(rootCmdFlagBinder, flagConfig, configSchema)
	defineEtcdBackupsFlags(rootCmd, rootCmdFlagBinder, flagConfig, configSchema)

	return rootCmd, nil
}

func defineServiceFlags(rootCmd *cobra.Command, rootCmdFlagBinder *FlagBinder, flagConfig *config.Params, schema *jsonschema.Schema) error {
	// API
	rootCmdFlagBinder.StringVar("bind-addr", flagDescription("services.api.endpoint", schema), &flagConfig.Services.Api.Endpoint)
	rootCmdFlagBinder.StringVar("advertised-api-url", flagDescription("services.api.advertisedURL", schema), &flagConfig.Services.Api.AdvertisedURL)
	rootCmdFlagBinder.StringVar("key", flagDescription("services.api.keyFile", schema), &flagConfig.Services.Api.KeyFile)
	rootCmdFlagBinder.StringVar("cert", flagDescription("services.api.certFile", schema), &flagConfig.Services.Api.CertFile)

	// Metrics
	rootCmdFlagBinder.StringVar("metrics-bind-addr", flagDescription("services.metrics.endpoint", schema), &flagConfig.Services.Metrics.Endpoint)

	// KubernetesProxy
	rootCmdFlagBinder.StringVar("k8s-proxy-bind-addr", flagDescription("services.kubernetesProxy.endpoint", schema), &flagConfig.Services.KubernetesProxy.Endpoint)

	rootCmdFlagBinder.StringVar("advertised-kubernetes-proxy-url", flagDescription("services.kubernetesProxy.advertisedURL", schema), &flagConfig.Services.KubernetesProxy.AdvertisedURL)

	// Siderolink
	rootCmdFlagBinder.StringVar("siderolink-wireguard-bind-addr", flagDescription("services.siderolink.wireGuard.endpoint", schema), &flagConfig.Services.Siderolink.WireGuard.Endpoint)
	rootCmdFlagBinder.StringVar("siderolink-wireguard-advertised-addr",
		flagDescription("services.siderolink.wireGuard.advertisedEndpoint", schema), &flagConfig.Services.Siderolink.WireGuard.AdvertisedEndpoint)
	rootCmdFlagBinder.BoolVar("siderolink-disable-last-endpoint",
		flagDescription("services.siderolink.disableLastEndpoint", schema), &flagConfig.Services.Siderolink.DisableLastEndpoint)
	rootCmdFlagBinder.BoolVar("siderolink-use-grpc-tunnel",
		flagDescription("services.siderolink.useGRPCTunnel", schema),
		&flagConfig.Services.Siderolink.UseGRPCTunnel,
	)
	rootCmdFlagBinder.IntVar("event-sink-port", flagDescription("services.siderolink.eventSinkPort", schema), &flagConfig.Services.Siderolink.EventSinkPort)
	rootCmdFlagBinder.IntVar("log-server-port", flagDescription("services.siderolink.logServerPort", schema), &flagConfig.Services.Siderolink.LogServerPort)
	EnumVar(rootCmdFlagBinder, "join-tokens-mode", flagDescription("services.siderolink.joinTokensMode", schema), &flagConfig.Services.Siderolink.JoinTokensMode)

	// MachineAPI
	for _, prefix := range []string{"siderolink", "machine"} {
		rootCmdFlagBinder.StringVar(fmt.Sprintf("%s-api-bind-addr", prefix), flagDescription("services.machineAPI.endpoint", schema), &flagConfig.Services.MachineAPI.Endpoint)
		rootCmdFlagBinder.StringVar(fmt.Sprintf("%s-api-cert", prefix), flagDescription("services.machineAPI.certFile", schema), &flagConfig.Services.MachineAPI.CertFile)
		rootCmdFlagBinder.StringVar(fmt.Sprintf("%s-api-key", prefix), flagDescription("services.machineAPI.keyFile", schema), &flagConfig.Services.MachineAPI.KeyFile)
		rootCmdFlagBinder.StringVar(fmt.Sprintf("%s-api-advertised-url", prefix), flagDescription("services.machineAPI.advertisedURL", schema), &flagConfig.Services.MachineAPI.AdvertisedURL)
	}

	if err := rootCmd.Flags().MarkDeprecated("siderolink-api-bind-addr", "--deprecated, use --machine-api-bind-addr"); err != nil {
		return err
	}

	if err := rootCmd.Flags().MarkDeprecated("siderolink-api-cert", "deprecated, use --machine-api-cert"); err != nil {
		return err
	}

	if err := rootCmd.Flags().MarkDeprecated("siderolink-api-key", "deprecated, use --machine-api-key"); err != nil {
		return err
	}

	// LoadBalancer
	rootCmdFlagBinder.IntVar("lb-min-port", flagDescription("services.loadBalancer.minPort", schema), &flagConfig.Services.LoadBalancer.MinPort)
	rootCmdFlagBinder.IntVar("lb-max-port", flagDescription("services.loadBalancer.maxPort", schema), &flagConfig.Services.LoadBalancer.MaxPort)

	rootCmdFlagBinder.IntVar("local-resource-server-port", flagDescription("services.localResourceService.port", schema), &flagConfig.Services.LocalResourceService.Port)

	rootCmdFlagBinder.BoolVar("local-resource-server-enabled", flagDescription("services.localResourceService.enabled", schema), &flagConfig.Services.LocalResourceService.Enabled)

	// Embedded discovery service
	rootCmdFlagBinder.BoolVar("embedded-discovery-service-enabled",
		flagDescription("services.embeddedDiscoveryService.enabled", schema), &flagConfig.Services.EmbeddedDiscoveryService.Enabled)

	rootCmdFlagBinder.IntVar("embedded-discovery-service-endpoint", flagDescription("services.embeddedDiscoveryService.port", schema), &flagConfig.Services.EmbeddedDiscoveryService.Port)

	if err := rootCmd.Flags().MarkDeprecated("embedded-discovery-service-endpoint", "use --embedded-discovery-service-port"); err != nil {
		return err
	}

	rootCmdFlagBinder.IntVar("embedded-discovery-service-port", flagDescription("services.embeddedDiscoveryService.port", schema), &flagConfig.Services.EmbeddedDiscoveryService.Port)

	rootCmdFlagBinder.BoolVar("embedded-discovery-service-snapshots-enabled",
		flagDescription("services.embeddedDiscoveryService.snapshotsEnabled", schema), &flagConfig.Services.EmbeddedDiscoveryService.SnapshotsEnabled)

	//nolint:staticcheck
	rootCmdFlagBinder.StringVar("embedded-discovery-service-snapshot-path",
		flagDescription("services.embeddedDiscoveryService.snapshotsPath", schema),
		&flagConfig.Services.EmbeddedDiscoveryService.SnapshotsPath,
	)

	if err := rootCmd.Flags().MarkDeprecated("embedded-discovery-service-snapshot-path", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
		return err
	}

	rootCmdFlagBinder.DurationVar("embedded-discovery-service-snapshot-interval",
		flagDescription("services.embeddedDiscoveryService.snapshotsInterval", schema), &flagConfig.Services.EmbeddedDiscoveryService.SnapshotsInterval)
	rootCmdFlagBinder.StringVar("embedded-discovery-service-log-level",
		flagDescription("services.embeddedDiscoveryService.logLevel", schema), &flagConfig.Services.EmbeddedDiscoveryService.LogLevel)
	rootCmdFlagBinder.DurationVar("embedded-discovery-service-sqlite-timeout",
		flagDescription("services.embeddedDiscoveryService.sqliteTimeout", schema), &flagConfig.Services.EmbeddedDiscoveryService.SqliteTimeout)

	// DevServerProxy
	rootCmdFlagBinder.StringVar("frontend-dst", flagDescription("services.devServerProxy.proxyTo", schema), &flagConfig.Services.DevServerProxy.ProxyTo)
	rootCmdFlagBinder.StringVar("frontend-bind", flagDescription("services.devServerProxy.endpoint", schema), &flagConfig.Services.DevServerProxy.Endpoint)

	return nil
}

func defineAuthFlags(rootCmd *cobra.Command, rootCmdFlagBinder *FlagBinder, flagConfig *config.Params, schema *jsonschema.Schema) {
	// Auth0
	rootCmdFlagBinder.BoolVar("auth-auth0-enabled",
		flagDescription("auth.auth0.enabled", schema), &flagConfig.Auth.Auth0.Enabled)
	rootCmdFlagBinder.StringVar("auth-auth0-client-id", flagDescription("auth.auth0.clientID", schema), &flagConfig.Auth.Auth0.ClientID)
	rootCmdFlagBinder.StringVar("auth-auth0-domain", flagDescription("auth.auth0.domain", schema), &flagConfig.Auth.Auth0.Domain)
	rootCmdFlagBinder.BoolVar("auth-auth0-use-form-data",
		flagDescription("auth.auth0.useFormData", schema), &flagConfig.Auth.Auth0.UseFormData)

	// Webauthn
	rootCmdFlagBinder.BoolVar("auth-webauthn-enabled",
		flagDescription("auth.webauthn.enabled", schema), &flagConfig.Auth.Webauthn.Enabled)
	rootCmdFlagBinder.BoolVar("auth-webauthn-required",
		flagDescription("auth.webauthn.required", schema), &flagConfig.Auth.Webauthn.Required)

	rootCmdFlagBinder.BoolVar("auth-saml-enabled",
		flagDescription("auth.saml.enabled", schema), &flagConfig.Auth.Saml.Enabled,
	)
	rootCmdFlagBinder.StringVar("auth-saml-url", flagDescription("auth.saml.url", schema), &flagConfig.Auth.Saml.Url)
	rootCmdFlagBinder.StringVar("auth-saml-metadata", flagDescription("auth.saml.metadata", schema), &flagConfig.Auth.Saml.Metadata)
	rootCmd.Flags().Var(&flagConfig.Auth.Saml.LabelRules, "auth-saml-label-rules", flagDescription("auth.saml.labelRules", schema))
	rootCmd.Flags().Var(&flagConfig.Auth.Saml.AttributeRules, "auth-saml-attribute-rules", flagDescription("auth.saml.attributeRules", schema))
	rootCmdFlagBinder.StringVar("auth-saml-name-id-format", flagDescription("auth.saml.nameIDFormat", schema), &flagConfig.Auth.Saml.NameIDFormat)
	rootCmd.Flags().StringSliceVar(&flagConfig.Auth.InitialUsers, "initial-users", flagConfig.Auth.InitialUsers, flagDescription("auth.initialUsers", schema))
	rootCmdFlagBinder.DurationVar("public-key-pruning-interval", flagDescription("auth.keyPruner.interval", schema), &flagConfig.Auth.KeyPruner.Interval)
	rootCmdFlagBinder.BoolVar("suspended", flagDescription("auth.suspended", schema), &flagConfig.Auth.Suspended)

	rootCmdFlagBinder.BoolVar("create-initial-service-account",
		flagDescription("auth.initialServiceAccount.enabled", schema), &flagConfig.Auth.InitialServiceAccount.Enabled)

	rootCmdFlagBinder.StringVar("initial-service-account-key-path", flagDescription("auth.initialServiceAccount.keyPath", schema), &flagConfig.Auth.InitialServiceAccount.KeyPath)

	rootCmdFlagBinder.StringVar("initial-service-account-role", flagDescription("auth.initialServiceAccount.role", schema), &flagConfig.Auth.InitialServiceAccount.Role)

	rootCmdFlagBinder.DurationVar("initial-service-account-lifetime",
		flagDescription("auth.initialServiceAccount.lifetime", schema), &flagConfig.Auth.InitialServiceAccount.Lifetime)

	rootCmd.MarkFlagsMutuallyExclusive("auth-saml-url", "auth-saml-metadata")

	// OIDC
	rootCmdFlagBinder.BoolVar("auth-oidc-enabled", flagDescription("auth.oidc.enabled", schema), &flagConfig.Auth.Oidc.Enabled)
	rootCmdFlagBinder.StringVar("auth-oidc-provider-url", flagDescription("auth.oidc.providerURL", schema), &flagConfig.Auth.Oidc.ProviderURL)
	rootCmdFlagBinder.StringVar("auth-oidc-client-id", flagDescription("auth.oidc.clientID", schema), &flagConfig.Auth.Oidc.ClientID)
	rootCmdFlagBinder.StringVar("auth-oidc-client-secret", flagDescription("auth.oidc.clientSecret", schema), &flagConfig.Auth.Oidc.ClientSecret)
	rootCmd.Flags().StringSliceVar(&flagConfig.Auth.Oidc.Scopes, "auth-oidc-scopes", flagConfig.Auth.Oidc.Scopes, flagDescription("auth.oidc.scopes", schema))
	rootCmdFlagBinder.StringVar("auth-oidc-logout-url", flagDescription("auth.oidc.logoutURL", schema), &flagConfig.Auth.Oidc.LogoutURL)
	rootCmdFlagBinder.BoolVar("auth-oidc-allow-unverified-email", flagDescription("auth.oidc.allowUnverifiedEmail", schema), &flagConfig.Auth.Oidc.AllowUnverifiedEmail)
}

//nolint:staticcheck // defineLogsFlags uses deprecated fields for backwards-compatibility
func defineLogsFlags(rootCmd *cobra.Command, rootCmdFlagBinder *FlagBinder, flagConfig *config.Params, schema *jsonschema.Schema) error {
	rootCmdFlagBinder.DurationVar("machine-log-sqlite-timeout", flagDescription("logs.machine.storage.sqliteTimeout", schema), &flagConfig.Logs.Machine.Storage.SqliteTimeout)
	rootCmdFlagBinder.DurationVar("machine-log-cleanup-interval", flagDescription("logs.machine.storage.cleanupInterval", schema), &flagConfig.Logs.Machine.Storage.CleanupInterval)
	rootCmdFlagBinder.DurationVar("machine-log-cleanup-older-than", flagDescription("logs.machine.storage.cleanupOlderThan", schema), &flagConfig.Logs.Machine.Storage.CleanupOlderThan)
	rootCmdFlagBinder.IntVar("machine-log-max-lines-per-machine", flagDescription("logs.machine.storage.maxLinesPerMachine", schema), &flagConfig.Logs.Machine.Storage.MaxLinesPerMachine)
	rootCmdFlagBinder.Float64Var("machine-log-cleanup-probability",
		flagDescription("logs.machine.storage.cleanupProbability", schema), &flagConfig.Logs.Machine.Storage.CleanupProbability)

	if err := rootCmd.Flags().MarkHidden("machine-log-cleanup-probability"); err != nil {
		return err
	}

	rootCmd.Flags().StringSliceVar(&flagConfig.Logs.ResourceLogger.Types, "log-resource-updates-types",
		flagConfig.Logs.ResourceLogger.Types, flagDescription("logs.resourceLogger.types", schema))
	rootCmdFlagBinder.StringVar("log-resource-updates-log-level", flagDescription("logs.resourceLogger.logLevel", schema), &flagConfig.Logs.ResourceLogger.LogLevel)
	rootCmdFlagBinder.BoolVar("audit-log-enabled", flagDescription("logs.audit.enabled", schema), &flagConfig.Logs.Audit.Enabled)
	rootCmdFlagBinder.DurationVar("audit-log-sqlite-timeout", flagDescription("logs.audit.sqliteTimeout", schema), &flagConfig.Logs.Audit.SqliteTimeout)
	rootCmdFlagBinder.DurationVar("audit-log-retention-period", flagDescription("logs.audit.retentionPeriod", schema), &flagConfig.Logs.Audit.RetentionPeriod)
	rootCmdFlagBinder.Uint64Var("audit-log-max-size", flagDescription("logs.audit.maxSize", schema), &flagConfig.Logs.Audit.MaxSize)
	rootCmdFlagBinder.Float64Var("audit-log-cleanup-probability", flagDescription("logs.audit.cleanupProbability", schema), &flagConfig.Logs.Audit.CleanupProbability)
	rootCmdFlagBinder.BoolVar("enable-stripe-reporting", flagDescription("logs.stripe.enabled", schema), &flagConfig.Logs.Stripe.Enabled)
	rootCmdFlagBinder.Uint32Var("stripe-minimum-commit", flagDescription("logs.stripe.minCommit", schema), &flagConfig.Logs.Stripe.MinCommit)

	// Deprecated logs flags, kept for backwards-compatibility
	//
	//nolint:staticcheck,errcheck
	{
		rootCmdFlagBinder.StringVar("audit-log-dir", flagDescription("logs.audit.path", schema), &flagConfig.Logs.Audit.Path)
		rootCmdFlagBinder.IntVar("machine-log-buffer-capacity", flagDescription("logs.machine.bufferInitialCapacity", schema), &flagConfig.Logs.Machine.BufferInitialCapacity)
		rootCmdFlagBinder.IntVar("machine-log-buffer-max-capacity", flagDescription("logs.machine.bufferMaxCapacity", schema), &flagConfig.Logs.Machine.BufferMaxCapacity)
		rootCmdFlagBinder.IntVar("machine-log-buffer-safe-gap", flagDescription("logs.machine.bufferSafetyGap", schema), &flagConfig.Logs.Machine.BufferSafetyGap)
		rootCmdFlagBinder.IntVar("machine-log-num-compressed-chunks", flagDescription("logs.machine.storage.numCompressedChunks", schema), &flagConfig.Logs.Machine.Storage.NumCompressedChunks)
		rootCmdFlagBinder.BoolVar("machine-log-storage-enabled", flagDescription("logs.machine.storage.enabled", schema), &flagConfig.Logs.Machine.Storage.Enabled)
		rootCmdFlagBinder.StringVar("machine-log-storage-path", flagDescription("logs.machine.storage.path", schema), &flagConfig.Logs.Machine.Storage.Path)
		rootCmdFlagBinder.StringVar("log-storage-path", flagDescription("logs.machine.storage.path", schema), &flagConfig.Logs.Machine.Storage.Path)

		if err := rootCmd.Flags().MarkDeprecated("audit-log-dir", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
			return err
		}

		if err := rootCmd.Flags().MarkDeprecated("machine-log-buffer-capacity", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
			return err
		}

		if err := rootCmd.Flags().MarkDeprecated("machine-log-buffer-max-capacity", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
			return err
		}

		if err := rootCmd.Flags().MarkDeprecated("machine-log-buffer-safe-gap", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
			return err
		}

		if err := rootCmd.Flags().MarkDeprecated("machine-log-num-compressed-chunks", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
			return err
		}

		if err := rootCmd.Flags().MarkDeprecated("machine-log-storage-enabled", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
			return err
		}

		if err := rootCmd.Flags().MarkDeprecated("machine-log-storage-path", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
			return err
		}

		if err := rootCmd.Flags().MarkDeprecated("log-storage-path", "use --machine-log-storage-path"); err != nil {
			return err
		}
	}

	return nil
}

func defineStorageFlags(rootCmd *cobra.Command, rootCmdFlagBinder *FlagBinder, flagConfig *config.Params, schema *jsonschema.Schema) error {
	EnumVar(rootCmdFlagBinder, "storage-kind", flagDescription("storage.default.kind", schema), &flagConfig.Storage.Default.Kind)
	rootCmdFlagBinder.BoolVar("etcd-embedded", flagDescription("storage.default.etcd.embedded", schema), &flagConfig.Storage.Default.Etcd.Embedded)
	rootCmdFlagBinder.BoolVar("etcd-embedded-unsafe-fsync", flagDescription("storage.default.etcd.embeddedUnsafeFsync", schema), &flagConfig.Storage.Default.Etcd.EmbeddedUnsafeFsync)
	rootCmdFlagBinder.StringVar("etcd-embedded-db-path", flagDescription("storage.default.etcd.embeddedDBPath", schema), &flagConfig.Storage.Default.Etcd.EmbeddedDBPath)

	ensure.NoError(rootCmd.Flags().MarkHidden("etcd-embedded-unsafe-fsync"))

	rootCmd.Flags().StringSliceVar(&flagConfig.Storage.Default.Etcd.Endpoints, "etcd-endpoints", flagConfig.Storage.Default.Etcd.Endpoints, flagDescription("storage.default.etcd.endpoints", schema))
	rootCmdFlagBinder.DurationVar("etcd-dial-keepalive-time", flagDescription("storage.default.etcd.dialKeepAliveTime", schema), &flagConfig.Storage.Default.Etcd.DialKeepAliveTime)
	rootCmdFlagBinder.DurationVar("etcd-dial-keepalive-timeout", flagDescription("storage.default.etcd.dialKeepAliveTimeout", schema), &flagConfig.Storage.Default.Etcd.DialKeepAliveTimeout)
	rootCmdFlagBinder.StringVar("etcd-ca-path", flagDescription("storage.default.etcd.caFile", schema), &flagConfig.Storage.Default.Etcd.CaFile)
	rootCmdFlagBinder.StringVar("etcd-client-cert-path", flagDescription("storage.default.etcd.certFile", schema), &flagConfig.Storage.Default.Etcd.CertFile)
	rootCmdFlagBinder.StringVar("etcd-client-key-path", flagDescription("storage.default.etcd.keyFile", schema), &flagConfig.Storage.Default.Etcd.KeyFile)
	rootCmdFlagBinder.StringVar("private-key-source", flagDescription("storage.default.etcd.privateKeySource", schema), &flagConfig.Storage.Default.Etcd.PrivateKeySource)

	rootCmd.Flags().StringSliceVar(&flagConfig.Storage.Default.Etcd.PublicKeyFiles, "public-key-files", flagConfig.Storage.Default.Etcd.PublicKeyFiles,
		flagDescription("storage.default.etcd.publicKeyFiles", schema))
	rootCmdFlagBinder.StringVar(
		"secondary-storage-path",
		flagDescription("storage.secondary.path", schema),
		&flagConfig.Storage.Secondary.Path, //nolint:staticcheck // backwards compatibility, remove when migration from boltdb to sqlite is done
	)

	if err := rootCmd.Flags().MarkDeprecated("secondary-storage-path", "this flag is kept for the SQLite migration, and will be removed in future versions"); err != nil {
		return err
	}

	rootCmdFlagBinder.StringVar(config.SQLiteStoragePathFlag,
		flagDescription("storage.sqlite.path", schema), &flagConfig.Storage.Sqlite.Path)

	rootCmdFlagBinder.StringVar("sqlite-storage-experimental-base-params",
		flagDescription("storage.sqlite.experimentalBaseParams", schema), &flagConfig.Storage.Sqlite.ExperimentalBaseParams)

	rootCmdFlagBinder.StringVar("sqlite-storage-extra-params",
		flagDescription("storage.sqlite.extraParams", schema),
		&flagConfig.Storage.Sqlite.ExtraParams)

	rootCmdFlagBinder.StringVar("vault-k8s-auth-mount-path",
		flagDescription("storage.vault.k8sAuthMountPath", schema), &flagConfig.Storage.Vault.K8SAuthMountPath)

	return nil
}

func defineRegistriesFlags(rootCmd *cobra.Command, rootCmdFlagBinder *FlagBinder, flagConfig *config.Params, schema *jsonschema.Schema) {
	rootCmdFlagBinder.StringVar("talos-installer-registry", flagDescription("registries.talos", schema), &flagConfig.Registries.Talos)
	rootCmdFlagBinder.StringVar("kubernetes-registry", flagDescription("registries.kubernetes", schema), &flagConfig.Registries.Kubernetes)
	rootCmdFlagBinder.StringVar("image-factory-address", flagDescription("registries.imageFactoryBaseURL", schema), &flagConfig.Registries.ImageFactoryBaseURL)
	rootCmdFlagBinder.StringVar("image-factory-pxe-address", flagDescription("registries.imageFactoryPXEBaseURL", schema), &flagConfig.Registries.ImageFactoryPXEBaseURL)

	rootCmd.Flags().StringSliceVar(&flagConfig.Registries.Mirrors, "registry-mirror", flagConfig.Registries.Mirrors,
		flagDescription("registries.mirrors", schema))
}

func defineFeatureFlags(rootCmdFlagBinder *FlagBinder, flagConfig *config.Params, schema *jsonschema.Schema) {
	rootCmdFlagBinder.BoolVar("enable-talos-pre-release-versions",
		flagDescription("features.enableTalosPreReleaseVersions", schema), &flagConfig.Features.EnableTalosPreReleaseVersions)

	rootCmdFlagBinder.BoolVar("config-data-compression-enabled", flagDescription("features.enableConfigDataCompression", schema), &flagConfig.Features.EnableConfigDataCompression)

	rootCmdFlagBinder.BoolVar("workload-proxying-enabled", flagDescription("services.workloadProxy.enabled", schema), &flagConfig.Services.WorkloadProxy.Enabled)

	rootCmdFlagBinder.StringVar("workload-proxying-subdomain", flagDescription("services.workloadProxy.subdomain", schema), &flagConfig.Services.WorkloadProxy.Subdomain)

	rootCmdFlagBinder.DurationVar("workload-proxying-stop-lbs-after",
		flagDescription("services.workloadProxy.stopLBsAfter", schema), &flagConfig.Services.WorkloadProxy.StopLBsAfter)

	rootCmdFlagBinder.BoolVar("enable-break-glass-configs", flagDescription("features.enableBreakGlassConfigs", schema), &flagConfig.Features.EnableBreakGlassConfigs)

	rootCmdFlagBinder.BoolVar("disable-controller-runtime-cache",
		flagDescription("features.disableControllerRuntimeCache", schema), &flagConfig.Features.DisableControllerRuntimeCache)

	rootCmdFlagBinder.BoolVar("enable-cluster-import", flagDescription("features.enableClusterImport", schema), &flagConfig.Features.EnableClusterImport)
}

func defineDebugFlags(rootCmdFlagBinder *FlagBinder, flagConfig *config.Params, schema *jsonschema.Schema) {
	rootCmdFlagBinder.StringVar("pprof-bind-addr", flagDescription("debug.pprof.endpoint", schema), &flagConfig.Debug.Pprof.Endpoint)
	rootCmdFlagBinder.StringVar("debug-server-endpoint", flagDescription("debug.server.endpoint", schema), &flagConfig.Debug.Server.Endpoint)
}

func defineEtcdBackupsFlags(rootCmd *cobra.Command, rootCmdFlagBinder *FlagBinder, flagConfig *config.Params, schema *jsonschema.Schema) {
	rootCmdFlagBinder.BoolVar("etcd-backup-s3", flagDescription("etcdBackup.s3Enabled", schema), &flagConfig.EtcdBackup.S3Enabled)
	rootCmdFlagBinder.StringVar("etcd-backup-local-path", flagDescription("etcdBackup.localPath", schema), &flagConfig.EtcdBackup.LocalPath)
	rootCmdFlagBinder.DurationVar("etcd-backup-tick-interval",
		flagDescription("etcdBackup.tickInterval", schema), &flagConfig.EtcdBackup.TickInterval)
	rootCmdFlagBinder.DurationVar("etcd-backup-jitter",
		flagDescription("etcdBackup.jitter", schema), &flagConfig.EtcdBackup.Jitter)
	rootCmdFlagBinder.DurationVar("etcd-backup-min-interval", flagDescription("etcdBackup.minInterval", schema), &flagConfig.EtcdBackup.MinInterval)
	rootCmdFlagBinder.DurationVar("etcd-backup-max-interval", flagDescription("etcdBackup.maxInterval", schema), &flagConfig.EtcdBackup.MaxInterval)
	rootCmdFlagBinder.Uint64Var("etcd-backup-upload-limit-mbps", flagDescription("etcdBackup.uploadLimitMbps", schema), &flagConfig.EtcdBackup.UploadLimitMbps)
	rootCmdFlagBinder.Uint64Var("etcd-backup-download-limit-mbps", flagDescription("etcdBackup.downloadLimitMbps", schema), &flagConfig.EtcdBackup.DownloadLimitMbps)

	rootCmd.MarkFlagsMutuallyExclusive("etcd-backup-s3", "etcd-backup-local-path")
}

func flagDescription(fieldPath string, schema *jsonschema.Schema) string {
	description := schema.Description(fieldPath)
	if description == "" {
		panic("no description for " + fieldPath)
	}

	// remove the first two words like "XYZ is" from the description
	parts := strings.SplitN(description, " ", 3)
	if len(parts) < 3 {
		return description
	}

	return strings.TrimSpace(parts[2])
}
