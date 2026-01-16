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

var rootCmdFlagBinder *FlagBinder

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
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

		if !rootCmdArgs.debug {
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

		configs := make([]*config.Params, 0, 2)

		if rootCmdArgs.configPath != "" {
			var fileConfig *config.Params

			if fileConfig, err = config.LoadFromFile(rootCmdArgs.configPath); err != nil {
				return fmt.Errorf("failed to load config from file: %w", err)
			}

			configs = append(configs, fileConfig)
		}

		configs = append(configs, flagConfig) // flags have the highest priority

		config, err := app.PrepareConfig(logger, configs...)
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

var rootCmdArgs struct {
	configPath string
	debug      bool
}

// Execute the command.
func Execute() error {
	return newCommand().Execute()
}

var flagConfig = &config.Params{}

func newCommand() *cobra.Command {
	rootCmdFlagBinder = NewFlagBinder(rootCmd)

	rootCmd.Flags().StringVar(&rootCmdArgs.configPath, "config-path", "", "load the config from the file, flags have bigger priority")
	rootCmd.Flags().BoolVar(&rootCmdArgs.debug, "debug", constants.IsDebugBuild, "enable debug logs.")

	rootCmdFlagBinder.StringVar("account-id", "instance account ID, should never be changed.", &flagConfig.Account.Id)
	rootCmdFlagBinder.StringVar("name", "instance user-facing name.", &flagConfig.Account.Name)
	rootCmdFlagBinder.StringVar("user-pilot-app-token", "user pilot app token.", &flagConfig.Account.UserPilot.AppToken)

	defineServiceFlags()
	defineAuthFlags()
	defineLogsFlags()
	defineStorageFlags()
	defineRegistriesFlags()
	defineFeatureFlags()
	defineDebugFlags()
	defineEtcdBackupsFlags()

	return rootCmd
}

func defineServiceFlags() {
	// API
	rootCmdFlagBinder.StringVar("bind-addr", "start HTTP + API server on the defined address.", &flagConfig.Services.Api.Endpoint)
	rootCmdFlagBinder.StringVar("advertised-api-url", "advertised API frontend URL.", &flagConfig.Services.Api.AdvertisedURL)
	rootCmdFlagBinder.StringVar("key", "TLS key file", &flagConfig.Services.Api.KeyFile)
	rootCmdFlagBinder.StringVar("cert", "TLS cert file", &flagConfig.Services.Api.CertFile)

	// Metrics
	rootCmdFlagBinder.StringVar("metrics-bind-addr", "start Prometheus HTTP server on the defined address.", &flagConfig.Services.Metrics.Endpoint)

	// KubernetesProxy
	rootCmdFlagBinder.StringVar("k8s-proxy-bind-addr", "start Kubernetes proxy on the defined address.", &flagConfig.Services.KubernetesProxy.Endpoint)

	rootCmdFlagBinder.StringVar("advertised-kubernetes-proxy-url", "advertised Kubernetes proxy URL.", &flagConfig.Services.KubernetesProxy.AdvertisedURL)

	// Siderolink
	rootCmdFlagBinder.StringVar("siderolink-wireguard-bind-addr", "SideroLink WireGuard bind address.", &flagConfig.Services.Siderolink.WireGuard.Endpoint)
	rootCmdFlagBinder.StringVar("siderolink-wireguard-advertised-addr",
		"advertised wireguard address which is passed down to the nodes.", &flagConfig.Services.Siderolink.WireGuard.AdvertisedEndpoint)
	rootCmdFlagBinder.BoolVar("siderolink-disable-last-endpoint",
		"do not populate last known peer endpoint for the WireGuard peers", &flagConfig.Services.Siderolink.DisableLastEndpoint)
	rootCmdFlagBinder.BoolVar("siderolink-use-grpc-tunnel",
		"use gRPC tunnel to wrap WireGuard traffic instead of UDP. When enabled, "+
			"the SideroLink connections from Talos machines will be configured to use the tunnel mode, regardless of their individual configuration. ",
		&flagConfig.Services.Siderolink.UseGRPCTunnel,
	)
	rootCmdFlagBinder.IntVar("event-sink-port", "event sink bind port.", &flagConfig.Services.Siderolink.EventSinkPort)
	rootCmdFlagBinder.IntVar("log-server-port", "port for TCP log server", &flagConfig.Services.Siderolink.LogServerPort)
	EnumVar(rootCmdFlagBinder, "join-tokens-mode", "configures Talos machine join flow to use secure node tokens", &flagConfig.Services.Siderolink.JoinTokensMode)

	// MachineAPI
	for _, prefix := range []string{"siderolink", "machine"} {
		rootCmdFlagBinder.StringVar(fmt.Sprintf("%s-api-bind-addr", prefix), "machine API provision bind address.", &flagConfig.Services.MachineAPI.Endpoint)
		rootCmdFlagBinder.StringVar(fmt.Sprintf("%s-api-cert", prefix), "machine API TLS cert file path.", &flagConfig.Services.MachineAPI.CertFile)
		rootCmdFlagBinder.StringVar(fmt.Sprintf("%s-api-key", prefix), "machine API TLS cert file path.", &flagConfig.Services.MachineAPI.KeyFile)
		rootCmdFlagBinder.StringVar(fmt.Sprintf("%s-api-advertised-url", prefix), "machine API advertised API URL.", &flagConfig.Services.MachineAPI.AdvertisedURL)
	}

	rootCmd.Flags().MarkDeprecated("siderolink-api-bind-addr", "--deprecated, use --machine-api-bind-addr") //nolint:errcheck
	rootCmd.Flags().MarkDeprecated("siderolink-api-cert", "deprecated, use --machine-api-cert")             //nolint:errcheck
	rootCmd.Flags().MarkDeprecated("siderolink-api-key", "deprecated, use --machine-api-key")               //nolint:errcheck

	// LoadBalancer
	rootCmdFlagBinder.IntVar("lb-min-port", "cluster load balancer port range min value.", &flagConfig.Services.LoadBalancer.MinPort)
	rootCmdFlagBinder.IntVar("lb-max-port", "cluster load balancer port range max value.", &flagConfig.Services.LoadBalancer.MaxPort)

	rootCmdFlagBinder.IntVar("local-resource-server-port", "port for local read-only public resource server.", &flagConfig.Services.LocalResourceService.Port)

	rootCmdFlagBinder.BoolVar("local-resource-server-enabled", "enable local read-only public resource server.", &flagConfig.Services.LocalResourceService.Enabled)

	// Embedded discovery service
	rootCmdFlagBinder.BoolVar("embedded-discovery-service-enabled",
		"enable embedded discovery service, binds only to the SideroLink WireGuard address", &flagConfig.Services.EmbeddedDiscoveryService.Enabled)

	rootCmdFlagBinder.IntVar("embedded-discovery-service-endpoint", "embedded discovery service port to listen on", &flagConfig.Services.EmbeddedDiscoveryService.Port)
	rootCmd.Flags().MarkDeprecated("embedded-discovery-service-endpoint", "use --embedded-discovery-service-port") //nolint:errcheck

	rootCmdFlagBinder.IntVar("embedded-discovery-service-port", "embedded discovery service port to listen on", &flagConfig.Services.EmbeddedDiscoveryService.Port)

	rootCmdFlagBinder.BoolVar("embedded-discovery-service-snapshots-enabled",
		"enable snapshots for the embedded discovery service", &flagConfig.Services.EmbeddedDiscoveryService.SnapshotsEnabled)

	//nolint:staticcheck
	rootCmdFlagBinder.StringVar("embedded-discovery-service-snapshot-path",
		"path to the file for storing the embedded discovery service state",
		&flagConfig.Services.EmbeddedDiscoveryService.SnapshotsPath,
	)

	//nolint:errcheck
	rootCmd.Flags().MarkDeprecated("embedded-discovery-service-snapshot-path", "this flag is kept for the SQLite migration, and will be removed in future versions")

	rootCmdFlagBinder.DurationVar("embedded-discovery-service-snapshot-interval",
		"interval for saving the embedded discovery service state", &flagConfig.Services.EmbeddedDiscoveryService.SnapshotsInterval)
	rootCmdFlagBinder.StringVar("embedded-discovery-service-log-level",
		"log level for the embedded discovery service - it has no effect if it is lower (more verbose) than the main log level", &flagConfig.Services.EmbeddedDiscoveryService.LogLevel)
	rootCmdFlagBinder.DurationVar("embedded-discovery-service-sqlite-timeout",
		"SQLite timeout for the embedded discovery service", &flagConfig.Services.EmbeddedDiscoveryService.SqliteTimeout)

	// DevServerProxy
	rootCmdFlagBinder.StringVar("frontend-dst", "destination address non API requests from proxy server.", &flagConfig.Services.DevServerProxy.ProxyTo)
	rootCmdFlagBinder.StringVar("frontend-bind", "proxy server which will redirect all non API requests to the defined frontend server.", &flagConfig.Services.DevServerProxy.Endpoint)
}

func defineAuthFlags() {
	// Auth0
	rootCmdFlagBinder.BoolVar("auth-auth0-enabled",
		"enable Auth0 authentication. Once set to true, it cannot be set back to false.", &flagConfig.Auth.Auth0.Enabled)
	rootCmdFlagBinder.StringVar("auth-auth0-client-id", "Auth0 application client ID.", &flagConfig.Auth.Auth0.ClientID)
	rootCmdFlagBinder.StringVar("auth-auth0-domain", "Auth0 application domain.", &flagConfig.Auth.Auth0.Domain)
	rootCmdFlagBinder.BoolVar("auth-auth0-use-form-data",
		"When true, data to the token endpoint is transmitted as x-www-form-urlencoded data instead of JSON. The default is false", &flagConfig.Auth.Auth0.UseFormData)

	// Webauthn
	rootCmdFlagBinder.BoolVar("auth-webauthn-enabled",
		"enable WebAuthn authentication. Once set to true, it cannot be set back to false.", &flagConfig.Auth.Webauthn.Enabled)
	rootCmdFlagBinder.BoolVar("auth-webauthn-required",
		"require WebAuthn authentication. Once set to true, it cannot be set back to false.", &flagConfig.Auth.Webauthn.Required)

	rootCmdFlagBinder.BoolVar("auth-saml-enabled",
		"enabled SAML authentication.", &flagConfig.Auth.Saml.Enabled,
	)
	rootCmdFlagBinder.StringVar("auth-saml-url", "SAML identity provider metadata URL (mutually exclusive with --auth-saml-metadata", &flagConfig.Auth.Saml.Url)
	rootCmdFlagBinder.StringVar("auth-saml-metadata", "SAML identity provider metadata file path (mutually exclusive with --auth-saml-url).", &flagConfig.Auth.Saml.Metadata)
	rootCmd.Flags().Var(&flagConfig.Auth.Saml.LabelRules, "auth-saml-label-rules", "defines mapping of SAML assertion attributes into Omni identity labels")
	rootCmd.Flags().Var(&flagConfig.Auth.Saml.AttributeRules, "auth-saml-attribute-rules", "defines additional identity, fullname, firstname and lastname mappings")
	rootCmdFlagBinder.StringVar("auth-saml-name-id-format", "allows setting name ID format for the account", &flagConfig.Auth.Saml.NameIDFormat)
	rootCmd.Flags().StringSliceVar(&flagConfig.Auth.InitialUsers, "initial-users", flagConfig.Auth.InitialUsers, "initial set of user emails. these users will be created on startup.")
	rootCmdFlagBinder.DurationVar("public-key-pruning-interval", "interval between public key pruning runs.", &flagConfig.Auth.KeyPruner.Interval)
	rootCmdFlagBinder.BoolVar("suspended", "start omni in suspended (read-only) mode.", &flagConfig.Auth.Suspended)

	rootCmdFlagBinder.BoolVar("create-initial-service-account",
		"create and dump a service account credentials on the first start of Omni", &flagConfig.Auth.InitialServiceAccount.Enabled)

	rootCmdFlagBinder.StringVar("initial-service-account-key-path", "dump the initial service account key into the path", &flagConfig.Auth.InitialServiceAccount.KeyPath)

	rootCmdFlagBinder.StringVar("initial-service-account-role", "the initial service account access role", &flagConfig.Auth.InitialServiceAccount.Role)

	rootCmdFlagBinder.DurationVar("initial-service-account-lifetime",
		"the lifetime duration of the initial service account key", &flagConfig.Auth.InitialServiceAccount.Lifetime)

	rootCmd.MarkFlagsMutuallyExclusive("auth-saml-url", "auth-saml-metadata")

	// OIDC
	rootCmdFlagBinder.BoolVar("auth-oidc-enabled", "enable OIDC authentication.", &flagConfig.Auth.Oidc.Enabled)
	rootCmdFlagBinder.StringVar("auth-oidc-provider-url", "OIDC authentication provider URL.", &flagConfig.Auth.Oidc.ProviderURL)
	rootCmdFlagBinder.StringVar("auth-oidc-client-id", "OIDC authentication client ID.", &flagConfig.Auth.Oidc.ClientID)
	rootCmdFlagBinder.StringVar("auth-oidc-client-secret", "OIDC authentication client secret.", &flagConfig.Auth.Oidc.ClientSecret)
	rootCmd.Flags().StringSliceVar(&flagConfig.Auth.Oidc.Scopes, "auth-oidc-scopes", flagConfig.Auth.Oidc.Scopes, "OIDC authentication scopes.")
	rootCmdFlagBinder.StringVar("auth-oidc-logout-url", "OIDC logout URL.", &flagConfig.Auth.Oidc.LogoutURL)
	rootCmdFlagBinder.BoolVar("auth-oidc-allow-unverified-email", "Allow OIDC tokens without email_verified claim.", &flagConfig.Auth.Oidc.AllowUnverifiedEmail)
}

//nolint:staticcheck // defineLogsFlags uses deprecated fields for backwards-compatibility
func defineLogsFlags() {
	rootCmdFlagBinder.DurationVar("machine-log-sqlite-timeout", "sqlite timeout for machine logs", &flagConfig.Logs.Machine.Storage.SqliteTimeout)
	rootCmdFlagBinder.DurationVar("machine-log-cleanup-interval", "interval between machine log cleanup runs", &flagConfig.Logs.Machine.Storage.CleanupInterval)
	rootCmdFlagBinder.DurationVar("machine-log-cleanup-older-than", "age threshold for machine log entries to be cleaned up", &flagConfig.Logs.Machine.Storage.CleanupOlderThan)
	rootCmdFlagBinder.IntVar("machine-log-max-lines-per-machine", "maximum number of log lines to keep per machine", &flagConfig.Logs.Machine.Storage.MaxLinesPerMachine)
	rootCmdFlagBinder.Float64Var("machine-log-cleanup-probability",
		"probability of running a size-based cleanup after each log write", &flagConfig.Logs.Machine.Storage.CleanupProbability)

	rootCmd.Flags().MarkHidden("machine-log-cleanup-probability") //nolint:errcheck

	rootCmd.Flags().StringSliceVar(&flagConfig.Logs.ResourceLogger.Types, "log-resource-updates-types",
		flagConfig.Logs.ResourceLogger.Types, "list of resource types whose updates should be logged")
	rootCmdFlagBinder.StringVar("log-resource-updates-log-level", "log level for resource updates", &flagConfig.Logs.ResourceLogger.LogLevel)
	rootCmdFlagBinder.BoolVar("audit-log-enabled", "enable audit logging", &flagConfig.Logs.Audit.Enabled)
	rootCmdFlagBinder.DurationVar("audit-log-sqlite-timeout", "sqlite timeout for audit logs", &flagConfig.Logs.Audit.SqliteTimeout)
	rootCmdFlagBinder.BoolVar("enable-stripe-reporting", "enable Stripe machine usage reporting", &flagConfig.Logs.Stripe.Enabled)
	rootCmdFlagBinder.Uint32Var("stripe-minimum-commit", "Minimum number of machines to report to Stripe for the given account", &flagConfig.Logs.Stripe.MinCommit)

	// Deprecated logs flags, kept for backwards-compatibility
	//
	//nolint:staticcheck,errcheck
	{
		rootCmdFlagBinder.StringVar("audit-log-dir", "Directory for audit log storage", &flagConfig.Logs.Audit.Path)
		rootCmdFlagBinder.IntVar("machine-log-buffer-capacity", "initial buffer capacity for machine logs in bytes", &flagConfig.Logs.Machine.BufferInitialCapacity)
		rootCmdFlagBinder.IntVar("machine-log-buffer-max-capacity", "max buffer capacity for machine logs in bytes", &flagConfig.Logs.Machine.BufferMaxCapacity)
		rootCmdFlagBinder.IntVar("machine-log-buffer-safe-gap", "safety gap for machine log buffer in bytes", &flagConfig.Logs.Machine.BufferSafetyGap)
		rootCmdFlagBinder.IntVar("machine-log-num-compressed-chunks", "number of compressed log chunks to keep", &flagConfig.Logs.Machine.Storage.NumCompressedChunks)
		rootCmdFlagBinder.BoolVar("machine-log-storage-enabled", "enable machine log storage", &flagConfig.Logs.Machine.Storage.Enabled)
		rootCmdFlagBinder.StringVar("machine-log-storage-path", "path of the directory for storing machine logs", &flagConfig.Logs.Machine.Storage.Path)
		rootCmdFlagBinder.StringVar("log-storage-path", "path of the directory for storing machine logs", &flagConfig.Logs.Machine.Storage.Path)

		rootCmd.Flags().MarkDeprecated("audit-log-dir", "this flag is kept for the SQLite migration, and will be removed in future versions")
		rootCmd.Flags().MarkDeprecated("machine-log-buffer-capacity", "this flag is kept for the SQLite migration, and will be removed in future versions")
		rootCmd.Flags().MarkDeprecated("machine-log-buffer-max-capacity", "this flag is kept for the SQLite migration, and will be removed in future versions")
		rootCmd.Flags().MarkDeprecated("machine-log-buffer-safe-gap", "this flag is kept for the SQLite migration, and will be removed in future versions")
		rootCmd.Flags().MarkDeprecated("machine-log-num-compressed-chunks", "this flag is kept for the SQLite migration, and will be removed in future versions")
		rootCmd.Flags().MarkDeprecated("machine-log-storage-enabled", "this flag is kept for the SQLite migration, and will be removed in future versions")
		rootCmd.Flags().MarkDeprecated("machine-log-storage-path", "this flag is kept for the SQLite migration, and will be removed in future versions")
		rootCmd.Flags().MarkDeprecated("log-storage-path", "use --machine-log-storage-path")
	}
}

func defineStorageFlags() {
	EnumVar(rootCmdFlagBinder, "storage-kind", "storage type: etcd|boltdb.", &flagConfig.Storage.Default.Kind)
	rootCmdFlagBinder.BoolVar("etcd-embedded", "use embedded etcd server.", &flagConfig.Storage.Default.Etcd.Embedded)
	rootCmdFlagBinder.BoolVar("etcd-embedded-unsafe-fsync", "disable fsync in the embedded etcd server (dangerous).", &flagConfig.Storage.Default.Etcd.EmbeddedUnsafeFsync)
	rootCmdFlagBinder.StringVar("etcd-embedded-db-path", "path to the embedded etcd database.", &flagConfig.Storage.Default.Etcd.EmbeddedDBPath)

	ensure.NoError(rootCmd.Flags().MarkHidden("etcd-embedded-unsafe-fsync"))

	rootCmd.Flags().StringSliceVar(&flagConfig.Storage.Default.Etcd.Endpoints, "etcd-endpoints", flagConfig.Storage.Default.Etcd.Endpoints, "external etcd endpoints.")
	rootCmdFlagBinder.DurationVar("etcd-dial-keepalive-time", "external etcd client keep-alive time (interval).", &flagConfig.Storage.Default.Etcd.DialKeepAliveTime)
	rootCmdFlagBinder.DurationVar("etcd-dial-keepalive-timeout", "external etcd client keep-alive timeout.", &flagConfig.Storage.Default.Etcd.DialKeepAliveTimeout)
	rootCmdFlagBinder.StringVar("etcd-ca-path", "external etcd CA path.", &flagConfig.Storage.Default.Etcd.CaFile)
	rootCmdFlagBinder.StringVar("etcd-client-cert-path", "external etcd client cert path.", &flagConfig.Storage.Default.Etcd.CertFile)
	rootCmdFlagBinder.StringVar("etcd-client-key-path", "external etcd client key path.", &flagConfig.Storage.Default.Etcd.KeyFile)
	rootCmdFlagBinder.StringVar("private-key-source", "file containing private key to use for decrypting master key slot.", &flagConfig.Storage.Default.Etcd.PrivateKeySource)

	rootCmd.Flags().StringSliceVar(&flagConfig.Storage.Default.Etcd.PublicKeyFiles, "public-key-files", flagConfig.Storage.Default.Etcd.PublicKeyFiles,
		"list of paths to files containing public keys to use for encrypting keys slots.")
	rootCmdFlagBinder.StringVar(
		"secondary-storage-path",
		fmt.Sprintf("path of the file for boltdb-backed secondary storage for frequently updated data (deprecated, see --%s).", config.SQLiteStoragePathFlag),
		&flagConfig.Storage.Secondary.Path, //nolint:staticcheck // backwards compatibility, remove when migration from boltdb to sqlite is done
	)

	rootCmd.Flags().MarkDeprecated("secondary-storage-path", "this flag is kept for the SQLite migration, and will be removed in future versions") //nolint:errcheck

	rootCmdFlagBinder.StringVar(config.SQLiteStoragePathFlag,
		"path of the file for sqlite-backed secondary storage for frequently updated data, machine and audit logs, it is required to be set.", &flagConfig.Storage.Sqlite.Path)
	rootCmdFlagBinder.StringVar("sqlite-storage-experimental-base-params",
		"base DSN (connection string) parameters for sqlite database connection. they must not start with question mark (?). "+
			"this flag is experimental and may be removed in future releases.", &flagConfig.Storage.Sqlite.ExperimentalBaseParams)

	rootCmdFlagBinder.StringVar("sqlite-storage-extra-params",
		"extra DSN (connection string) parameters for sqlite database connection. they will be appended to the base params. they must not start with ampersand (&).",
		&flagConfig.Storage.Sqlite.ExtraParams)
}

func defineRegistriesFlags() {
	rootCmdFlagBinder.StringVar("talos-installer-registry", "Talos installer image registry.", &flagConfig.Registries.Talos)
	rootCmdFlagBinder.StringVar("kubernetes-registry", "Kubernetes container registry.", &flagConfig.Registries.Kubernetes)
	rootCmdFlagBinder.StringVar("image-factory-address", "Image factory base URL to use.", &flagConfig.Registries.ImageFactoryBaseURL)
	rootCmdFlagBinder.StringVar("image-factory-pxe-address", "Image factory pxe base URL to use.", &flagConfig.Registries.ImageFactoryPXEBaseURL)

	rootCmd.Flags().StringSliceVar(&flagConfig.Registries.Mirrors, "registry-mirror", flagConfig.Registries.Mirrors,
		"list of registry mirrors to use in format: <registry host>=<mirror URL>")
}

func defineFeatureFlags() {
	rootCmdFlagBinder.BoolVar("enable-talos-pre-release-versions",
		"make Omni version discovery controler include Talos pre-release versions.", &flagConfig.Features.EnableTalosPreReleaseVersions)

	rootCmdFlagBinder.BoolVar("config-data-compression-enabled", "enable config data compression.", &flagConfig.Features.EnableConfigDataCompression)

	rootCmdFlagBinder.BoolVar("workload-proxying-enabled", "enable workload proxying feature.", &flagConfig.Services.WorkloadProxy.Enabled)

	rootCmdFlagBinder.StringVar("workload-proxying-subdomain", "workload proxying subdomain.", &flagConfig.Services.WorkloadProxy.Subdomain)

	rootCmdFlagBinder.DurationVar("workload-proxying-stop-lbs-after",
		"stop load balancers after this duration when workload proxying is enabled. Set to 0 to disable.", &flagConfig.Services.WorkloadProxy.StopLBsAfter)

	rootCmdFlagBinder.BoolVar("enable-break-glass-configs", "Allows downloading admin Talos and Kubernetes configs.", &flagConfig.Features.EnableBreakGlassConfigs)

	rootCmdFlagBinder.BoolVar("disable-controller-runtime-cache",
		"disable watch-based cache for controller-runtime (affects performance)", &flagConfig.Features.DisableControllerRuntimeCache)

	rootCmdFlagBinder.BoolVar("enable-cluster-import", "enable EXPERIMENTAL cluster import feature.", &flagConfig.Features.EnableClusterImport)
}

func defineDebugFlags() {
	rootCmdFlagBinder.StringVar("pprof-bind-addr", "start pprof HTTP server on the defined address (\"\" if disabled).", &flagConfig.Debug.Pprof.Endpoint)
	rootCmdFlagBinder.StringVar("debug-server-endpoint", "start debug HTTP server on the defined address (\"\" if disabled).", &flagConfig.Debug.Server.Endpoint)
}

func defineEtcdBackupsFlags() {
	rootCmdFlagBinder.BoolVar("etcd-backup-s3", "S3 will be used for cluster etcd backups", &flagConfig.EtcdBackup.S3Enabled)
	rootCmdFlagBinder.StringVar("etcd-backup-local-path", "path to local directory for cluster etcd backups", &flagConfig.EtcdBackup.LocalPath)
	rootCmdFlagBinder.DurationVar("etcd-backup-tick-interval",
		"interval between etcd backups ticks (controller events to check if any cluster needs to be backed up)", &flagConfig.EtcdBackup.TickInterval)
	rootCmdFlagBinder.DurationVar("etcd-backup-jitter",
		"jitter for etcd backups, randomly added/subtracted from the interval between automatic etcd backups", &flagConfig.EtcdBackup.Jitter)
	rootCmdFlagBinder.DurationVar("etcd-backup-min-interval", "minimal interval between etcd backups", &flagConfig.EtcdBackup.MinInterval)
	rootCmdFlagBinder.DurationVar("etcd-backup-max-interval", "maximal interval between etcd backups", &flagConfig.EtcdBackup.MaxInterval)
	rootCmdFlagBinder.Uint64Var("etcd-backup-upload-limit-mbps", "throughput limit in Mbps for etcd backup uploads, zero means unlimited", &flagConfig.EtcdBackup.UploadLimitMbps)
	rootCmdFlagBinder.Uint64Var("etcd-backup-download-limit-mbps", "throughput limit in Mbps for etcd backup downloads, zero means unlimited", &flagConfig.EtcdBackup.DownloadLimitMbps)

	rootCmd.MarkFlagsMutuallyExclusive("etcd-backup-s3", "etcd-backup-local-path")
}
