// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
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

var rootCmdFlagBinder *flagBinder

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          "omni",
	Short:        "Omni Kubernetes management platform service",
	Long:         "This executable runs both Omni frontend and Omni backend",
	SilenceUsage: true,
	Version:      version.Tag,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Apply all deferred flag bindings
		if err := rootCmdFlagBinder.bindAll(); err != nil {
			return fmt.Errorf("failed to bind flags: %w", err)
		}

		return nil
	},
	RunE: func(*cobra.Command, []string) error {
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

		config, err := app.PrepareConfig(logger, cmdConfig)
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
	rootCmd.Flags().StringVar(&rootCmdArgs.configPath, "config-path", "", "load the config from the file, flags have bigger priority")

	// Parsing the config path flag early to let it populate the initial configuration
	// ignore all errors, they will be handled in the Execute
	rootCmd.Flags().Parse(os.Args[1:]) //nolint:errcheck

	if rootCmdArgs.configPath == "" {
		return newCommand().Execute()
	}

	cfg, err := config.LoadFromFile(rootCmdArgs.configPath)
	if err != nil {
		return err
	}

	// override the cmdConfig with the config loaded from the file
	if err := merge.Merge(cmdConfig, cfg); err != nil {
		return err
	}

	return newCommand().Execute()
}

var cmdConfig = config.Default()

func newCommand() *cobra.Command {
	rootCmdFlagBinder = newFlagBinder(rootCmd)

	rootCmd.Flags().BoolVar(&rootCmdArgs.debug, "debug", constants.IsDebugBuild, "enable debug logs.")

	rootCmdFlagBinder.stringVar("account-id", "instance account ID, should never be changed.", &cmdConfig.Account.Id)
	rootCmdFlagBinder.stringVar("name", "instance user-facing name.", &cmdConfig.Account.Name)
	rootCmdFlagBinder.stringVar("user-pilot-app-token", "user pilot app token.", &cmdConfig.Account.UserPilot.AppToken)

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
	rootCmdFlagBinder.stringVar("bind-addr", "start HTTP + API server on the defined address.", &cmdConfig.Services.Api.Endpoint)
	rootCmdFlagBinder.stringVar("advertised-api-url", "advertised API frontend URL.", &cmdConfig.Services.Api.AdvertisedURL)
	rootCmdFlagBinder.stringVar("key", "TLS key file", &cmdConfig.Services.Api.KeyFile)
	rootCmdFlagBinder.stringVar("cert", "TLS cert file", &cmdConfig.Services.Api.CertFile)

	// Metrics
	rootCmdFlagBinder.stringVar("metrics-bind-addr", "start Prometheus HTTP server on the defined address.", &cmdConfig.Services.Metrics.Endpoint)

	// KubernetesProxy
	rootCmdFlagBinder.stringVar("k8s-proxy-bind-addr", "start Kubernetes proxy on the defined address.", &cmdConfig.Services.KubernetesProxy.Endpoint)

	rootCmdFlagBinder.stringVar("advertised-kubernetes-proxy-url", "advertised Kubernetes proxy URL.", &cmdConfig.Services.KubernetesProxy.AdvertisedURL)

	// Siderolink
	rootCmdFlagBinder.stringVar("siderolink-wireguard-bind-addr", "SideroLink WireGuard bind address.", &cmdConfig.Services.Siderolink.WireGuard.Endpoint)
	rootCmdFlagBinder.stringVar("siderolink-wireguard-advertised-addr",
		"advertised wireguard address which is passed down to the nodes.", &cmdConfig.Services.Siderolink.WireGuard.AdvertisedEndpoint)
	rootCmdFlagBinder.boolVar("siderolink-disable-last-endpoint",
		"do not populate last known peer endpoint for the WireGuard peers", &cmdConfig.Services.Siderolink.DisableLastEndpoint)
	rootCmdFlagBinder.boolVar("siderolink-use-grpc-tunnel",
		"use gRPC tunnel to wrap WireGuard traffic instead of UDP. When enabled, "+
			"the SideroLink connections from Talos machines will be configured to use the tunnel mode, regardless of their individual configuration. ",
		&cmdConfig.Services.Siderolink.UseGRPCTunnel,
	)
	rootCmdFlagBinder.intVar("event-sink-port", "event sink bind port.", &cmdConfig.Services.Siderolink.EventSinkPort)
	rootCmdFlagBinder.intVar("log-server-port", "port for TCP log server", &cmdConfig.Services.Siderolink.LogServerPort)
	enumVar(rootCmdFlagBinder, "join-tokens-mode", "configures Talos machine join flow to use secure node tokens", &cmdConfig.Services.Siderolink.JoinTokensMode)

	// MachineAPI
	for _, prefix := range []string{"siderolink", "machine"} {
		rootCmdFlagBinder.stringVar(fmt.Sprintf("%s-api-bind-addr", prefix), "machine API provision bind address.", &cmdConfig.Services.MachineAPI.Endpoint)
		rootCmdFlagBinder.stringVar(fmt.Sprintf("%s-api-cert", prefix), "machine API TLS cert file path.", &cmdConfig.Services.MachineAPI.CertFile)
		rootCmdFlagBinder.stringVar(fmt.Sprintf("%s-api-key", prefix), "machine API TLS cert file path.", &cmdConfig.Services.MachineAPI.KeyFile)
		rootCmdFlagBinder.stringVar(fmt.Sprintf("%s-api-advertised-url", prefix), "machine API advertised API URL.", &cmdConfig.Services.MachineAPI.AdvertisedURL)
	}

	rootCmd.Flags().MarkDeprecated("siderolink-api-bind-addr", "--deprecated, use --machine-api-bind-addr") //nolint:errcheck
	rootCmd.Flags().MarkDeprecated("siderolink-api-cert", "deprecated, use --machine-api-cert")             //nolint:errcheck
	rootCmd.Flags().MarkDeprecated("siderolink-api-key", "deprecated, use --machine-api-key")               //nolint:errcheck

	// LoadBalancer
	rootCmdFlagBinder.intVar("lb-min-port", "cluster load balancer port range min value.", &cmdConfig.Services.LoadBalancer.MinPort)
	rootCmdFlagBinder.intVar("lb-max-port", "cluster load balancer port range max value.", &cmdConfig.Services.LoadBalancer.MaxPort)

	rootCmdFlagBinder.intVar("local-resource-server-port", "port for local read-only public resource server.", &cmdConfig.Services.LocalResourceService.Port)

	rootCmdFlagBinder.boolVar("local-resource-server-enabled", "enable local read-only public resource server.", &cmdConfig.Services.LocalResourceService.Enabled)

	// Embedded discovery service
	rootCmdFlagBinder.boolVar("embedded-discovery-service-enabled",
		"enable embedded discovery service, binds only to the SideroLink WireGuard address", &cmdConfig.Services.EmbeddedDiscoveryService.Enabled)

	rootCmdFlagBinder.intVar("embedded-discovery-service-endpoint", "embedded discovery service port to listen on", &cmdConfig.Services.EmbeddedDiscoveryService.Port)
	rootCmd.Flags().MarkDeprecated("embedded-discovery-service-endpoint", "use --embedded-discovery-service-port") //nolint:errcheck

	rootCmdFlagBinder.intVar("embedded-discovery-service-port", "embedded discovery service port to listen on", &cmdConfig.Services.EmbeddedDiscoveryService.Port)

	rootCmdFlagBinder.boolVar("embedded-discovery-service-snapshots-enabled",
		"enable snapshots for the embedded discovery service", &cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsEnabled)

	//nolint:staticcheck
	rootCmdFlagBinder.stringVar("embedded-discovery-service-snapshot-path",
		"path to the file for storing the embedded discovery service state",
		&cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsPath,
	)

	//nolint:errcheck
	rootCmd.Flags().MarkDeprecated("embedded-discovery-service-snapshot-path", "this flag is kept for the SQLite migration, and will be removed in future versions")

	rootCmdFlagBinder.durationVar("embedded-discovery-service-snapshot-interval",
		"interval for saving the embedded discovery service state", &cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsInterval)
	rootCmdFlagBinder.stringVar("embedded-discovery-service-log-level",
		"log level for the embedded discovery service - it has no effect if it is lower (more verbose) than the main log level", &cmdConfig.Services.EmbeddedDiscoveryService.LogLevel)
	rootCmdFlagBinder.durationVar("embedded-discovery-service-sqlite-timeout",
		"SQLite timeout for the embedded discovery service", &cmdConfig.Services.EmbeddedDiscoveryService.SqliteTimeout)

	// DevServerProxy
	rootCmdFlagBinder.stringVar("frontend-dst", "destination address non API requests from proxy server.", &cmdConfig.Services.DevServerProxy.ProxyTo)
	rootCmdFlagBinder.stringVar("frontend-bind", "proxy server which will redirect all non API requests to the defined frontend server.", &cmdConfig.Services.DevServerProxy.Endpoint)
}

func defineAuthFlags() {
	// Auth0
	rootCmdFlagBinder.boolVar("auth-auth0-enabled",
		"enable Auth0 authentication. Once set to true, it cannot be set back to false.", &cmdConfig.Auth.Auth0.Enabled)
	rootCmdFlagBinder.stringVar("auth-auth0-client-id", "Auth0 application client ID.", &cmdConfig.Auth.Auth0.ClientID)
	rootCmdFlagBinder.stringVar("auth-auth0-domain", "Auth0 application domain.", &cmdConfig.Auth.Auth0.Domain)
	rootCmdFlagBinder.boolVar("auth-auth0-use-form-data",
		"When true, data to the token endpoint is transmitted as x-www-form-urlencoded data instead of JSON. The default is false", &cmdConfig.Auth.Auth0.UseFormData)

	// Webauthn
	rootCmdFlagBinder.boolVar("auth-webauthn-enabled",
		"enable WebAuthn authentication. Once set to true, it cannot be set back to false.", &cmdConfig.Auth.Webauthn.Enabled)
	rootCmdFlagBinder.boolVar("auth-webauthn-required",
		"require WebAuthn authentication. Once set to true, it cannot be set back to false.", &cmdConfig.Auth.Webauthn.Required)

	rootCmdFlagBinder.boolVar("auth-saml-enabled",
		"enabled SAML authentication.", &cmdConfig.Auth.Saml.Enabled,
	)
	rootCmdFlagBinder.stringVar("auth-saml-url", "SAML identity provider metadata URL (mutually exclusive with --auth-saml-metadata", &cmdConfig.Auth.Saml.Url)
	rootCmdFlagBinder.stringVar("auth-saml-metadata", "SAML identity provider metadata file path (mutually exclusive with --auth-saml-url).", &cmdConfig.Auth.Saml.Metadata)
	rootCmd.Flags().Var(&cmdConfig.Auth.Saml.LabelRules, "auth-saml-label-rules", "defines mapping of SAML assertion attributes into Omni identity labels")
	rootCmd.Flags().Var(&cmdConfig.Auth.Saml.AttributeRules, "auth-saml-attribute-rules", "defines additional identity, fullname, firstname and lastname mappings")
	rootCmdFlagBinder.stringVar("auth-saml-name-id-format", "allows setting name ID format for the account", &cmdConfig.Auth.Saml.NameIDFormat)
	rootCmd.Flags().StringSliceVar(&cmdConfig.Auth.InitialUsers, "initial-users", cmdConfig.Auth.InitialUsers, "initial set of user emails. these users will be created on startup.")
	rootCmdFlagBinder.durationVar("public-key-pruning-interval", "interval between public key pruning runs.", &cmdConfig.Auth.KeyPruner.Interval)
	rootCmdFlagBinder.boolVar("suspended", "start omni in suspended (read-only) mode.", &cmdConfig.Auth.Suspended)

	rootCmdFlagBinder.boolVar("create-initial-service-account",
		"create and dump a service account credentials on the first start of Omni", &cmdConfig.Auth.InitialServiceAccount.Enabled)

	rootCmdFlagBinder.stringVar("initial-service-account-key-path", "dump the initial service account key into the path", &cmdConfig.Auth.InitialServiceAccount.KeyPath)

	rootCmdFlagBinder.stringVar("initial-service-account-role", "the initial service account access role", &cmdConfig.Auth.InitialServiceAccount.Role)

	rootCmdFlagBinder.durationVar("initial-service-account-lifetime",
		"the lifetime duration of the initial service account key", &cmdConfig.Auth.InitialServiceAccount.Lifetime)

	rootCmd.MarkFlagsMutuallyExclusive("auth-saml-url", "auth-saml-metadata")

	// OIDC
	rootCmdFlagBinder.boolVar("auth-oidc-enabled", "enable OIDC authentication.", &cmdConfig.Auth.Oidc.Enabled)
	rootCmdFlagBinder.stringVar("auth-oidc-provider-url", "OIDC authentication provider URL.", &cmdConfig.Auth.Oidc.ProviderURL)
	rootCmdFlagBinder.stringVar("auth-oidc-client-id", "OIDC authentication client ID.", &cmdConfig.Auth.Oidc.ClientID)
	rootCmdFlagBinder.stringVar("auth-oidc-client-secret", "OIDC authentication client secret.", &cmdConfig.Auth.Oidc.ClientSecret)
	rootCmd.Flags().StringSliceVar(&cmdConfig.Auth.Oidc.Scopes, "auth-oidc-scopes", cmdConfig.Auth.Oidc.Scopes, "OIDC authentication scopes.")
	rootCmdFlagBinder.stringVar("auth-oidc-logout-url", "OIDC logout URL.", &cmdConfig.Auth.Oidc.LogoutURL)
	rootCmdFlagBinder.boolVar("auth-oidc-allow-unverified-email", "Allow OIDC tokens without email_verified claim.", &cmdConfig.Auth.Oidc.AllowUnverifiedEmail)
}

//nolint:staticcheck // defineLogsFlags uses deprecated fields for backwards-compatibility
func defineLogsFlags() {
	rootCmdFlagBinder.durationVar("machine-log-sqlite-timeout", "sqlite timeout for machine logs", &cmdConfig.Logs.Machine.Storage.SqliteTimeout)
	rootCmdFlagBinder.durationVar("machine-log-cleanup-interval", "interval between machine log cleanup runs", &cmdConfig.Logs.Machine.Storage.CleanupInterval)
	rootCmdFlagBinder.durationVar("machine-log-cleanup-older-than", "age threshold for machine log entries to be cleaned up", &cmdConfig.Logs.Machine.Storage.CleanupOlderThan)
	rootCmdFlagBinder.intVar("machine-log-max-lines-per-machine", "maximum number of log lines to keep per machine", &cmdConfig.Logs.Machine.Storage.MaxLinesPerMachine)
	rootCmdFlagBinder.float64Var("machine-log-cleanup-probability",
		"probability of running a size-based cleanup after each log write", &cmdConfig.Logs.Machine.Storage.CleanupProbability)

	rootCmd.Flags().MarkHidden("machine-log-cleanup-probability") //nolint:errcheck

	rootCmd.Flags().StringSliceVar(&cmdConfig.Logs.ResourceLogger.Types, "log-resource-updates-types",
		cmdConfig.Logs.ResourceLogger.Types, "list of resource types whose updates should be logged")
	rootCmdFlagBinder.stringVar("log-resource-updates-log-level", "log level for resource updates", &cmdConfig.Logs.ResourceLogger.LogLevel)
	rootCmdFlagBinder.boolVar("audit-log-enabled", "enable audit logging", &cmdConfig.Logs.Audit.Enabled)
	rootCmdFlagBinder.durationVar("audit-log-sqlite-timeout", "sqlite timeout for audit logs", &cmdConfig.Logs.Audit.SqliteTimeout)
	rootCmdFlagBinder.boolVar("enable-stripe-reporting", "enable Stripe machine usage reporting", &cmdConfig.Logs.Stripe.Enabled)
	rootCmdFlagBinder.uint32Var("stripe-minimum-commit", "Minimum number of machines to report to Stripe for the given account", &cmdConfig.Logs.Stripe.MinCommit)

	// Deprecated logs flags, kept for backwards-compatibility
	//
	//nolint:staticcheck,errcheck
	{
		rootCmdFlagBinder.stringVar("audit-log-dir", "Directory for audit log storage", &cmdConfig.Logs.Audit.Path)
		rootCmdFlagBinder.intVar("machine-log-buffer-capacity", "initial buffer capacity for machine logs in bytes", &cmdConfig.Logs.Machine.BufferInitialCapacity)
		rootCmdFlagBinder.intVar("machine-log-buffer-max-capacity", "max buffer capacity for machine logs in bytes", &cmdConfig.Logs.Machine.BufferMaxCapacity)
		rootCmdFlagBinder.intVar("machine-log-buffer-safe-gap", "safety gap for machine log buffer in bytes", &cmdConfig.Logs.Machine.BufferSafetyGap)
		rootCmdFlagBinder.intVar("machine-log-num-compressed-chunks", "number of compressed log chunks to keep", &cmdConfig.Logs.Machine.Storage.NumCompressedChunks)
		rootCmdFlagBinder.boolVar("machine-log-storage-enabled", "enable machine log storage", &cmdConfig.Logs.Machine.Storage.Enabled)
		rootCmdFlagBinder.stringVar("machine-log-storage-path", "path of the directory for storing machine logs", &cmdConfig.Logs.Machine.Storage.Path)
		rootCmdFlagBinder.stringVar("log-storage-path", "path of the directory for storing machine logs", &cmdConfig.Logs.Machine.Storage.Path)

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
	enumVar(rootCmdFlagBinder, "storage-kind", "storage type: etcd|boltdb.", &cmdConfig.Storage.Default.Kind)
	rootCmdFlagBinder.boolVar("etcd-embedded", "use embedded etcd server.", &cmdConfig.Storage.Default.Etcd.Embedded)
	rootCmdFlagBinder.boolVar("etcd-embedded-unsafe-fsync", "disable fsync in the embedded etcd server (dangerous).", &cmdConfig.Storage.Default.Etcd.EmbeddedUnsafeFsync)
	rootCmdFlagBinder.stringVar("etcd-embedded-db-path", "path to the embedded etcd database.", &cmdConfig.Storage.Default.Etcd.EmbeddedDBPath)

	ensure.NoError(rootCmd.Flags().MarkHidden("etcd-embedded-unsafe-fsync"))

	rootCmd.Flags().StringSliceVar(&cmdConfig.Storage.Default.Etcd.Endpoints, "etcd-endpoints", cmdConfig.Storage.Default.Etcd.Endpoints, "external etcd endpoints.")
	rootCmdFlagBinder.durationVar("etcd-dial-keepalive-time", "external etcd client keep-alive time (interval).", &cmdConfig.Storage.Default.Etcd.DialKeepAliveTime)
	rootCmdFlagBinder.durationVar("etcd-dial-keepalive-timeout", "external etcd client keep-alive timeout.", &cmdConfig.Storage.Default.Etcd.DialKeepAliveTimeout)
	rootCmdFlagBinder.stringVar("etcd-ca-path", "external etcd CA path.", &cmdConfig.Storage.Default.Etcd.CaFile)
	rootCmdFlagBinder.stringVar("etcd-client-cert-path", "external etcd client cert path.", &cmdConfig.Storage.Default.Etcd.CertFile)
	rootCmdFlagBinder.stringVar("etcd-client-key-path", "external etcd client key path.", &cmdConfig.Storage.Default.Etcd.KeyFile)
	rootCmdFlagBinder.stringVar("private-key-source", "file containing private key to use for decrypting master key slot.", &cmdConfig.Storage.Default.Etcd.PrivateKeySource)

	rootCmd.Flags().StringSliceVar(&cmdConfig.Storage.Default.Etcd.PublicKeyFiles, "public-key-files", cmdConfig.Storage.Default.Etcd.PublicKeyFiles,
		"list of paths to files containing public keys to use for encrypting keys slots.")
	rootCmdFlagBinder.stringVar(
		"secondary-storage-path",
		fmt.Sprintf("path of the file for boltdb-backed secondary storage for frequently updated data (deprecated, see --%s).", config.SQLiteStoragePathFlag),
		&cmdConfig.Storage.Secondary.Path, //nolint:staticcheck // backwards compatibility, remove when migration from boltdb to sqlite is done
	)

	rootCmd.Flags().MarkDeprecated("secondary-storage-path", "this flag is kept for the SQLite migration, and will be removed in future versions") //nolint:errcheck

	rootCmdFlagBinder.stringVar(config.SQLiteStoragePathFlag,
		"path of the file for sqlite-backed secondary storage for frequently updated data, machine and audit logs, it is required to be set.", &cmdConfig.Storage.Sqlite.Path)
	rootCmdFlagBinder.stringVar("sqlite-storage-experimental-base-params",
		"base DSN (connection string) parameters for sqlite database connection. they must not start with question mark (?). "+
			"this flag is experimental and may be removed in future releases.", &cmdConfig.Storage.Sqlite.ExperimentalBaseParams)

	rootCmdFlagBinder.stringVar("sqlite-storage-extra-params",
		"extra DSN (connection string) parameters for sqlite database connection. they will be appended to the base params. they must not start with ampersand (&).",
		&cmdConfig.Storage.Sqlite.ExtraParams)
}

func defineRegistriesFlags() {
	rootCmdFlagBinder.stringVar("talos-installer-registry", "Talos installer image registry.", &cmdConfig.Registries.Talos)
	rootCmdFlagBinder.stringVar("kubernetes-registry", "Kubernetes container registry.", &cmdConfig.Registries.Kubernetes)
	rootCmdFlagBinder.stringVar("image-factory-address", "Image factory base URL to use.", &cmdConfig.Registries.ImageFactoryBaseURL)
	rootCmdFlagBinder.stringVar("image-factory-pxe-address", "Image factory pxe base URL to use.", &cmdConfig.Registries.ImageFactoryPXEBaseURL)

	rootCmd.Flags().StringSliceVar(&cmdConfig.Registries.Mirrors, "registry-mirror", cmdConfig.Registries.Mirrors,
		"list of registry mirrors to use in format: <registry host>=<mirror URL>")
}

func defineFeatureFlags() {
	rootCmdFlagBinder.boolVar("enable-talos-pre-release-versions",
		"make Omni version discovery controler include Talos pre-release versions.", &cmdConfig.Features.EnableTalosPreReleaseVersions)

	rootCmdFlagBinder.boolVar("config-data-compression-enabled", "enable config data compression.", &cmdConfig.Features.EnableConfigDataCompression)

	rootCmdFlagBinder.boolVar("workload-proxying-enabled", "enable workload proxying feature.", &cmdConfig.Services.WorkloadProxy.Enabled)

	rootCmdFlagBinder.stringVar("workload-proxying-subdomain", "workload proxying subdomain.", &cmdConfig.Services.WorkloadProxy.Subdomain)

	rootCmdFlagBinder.durationVar("workload-proxying-stop-lbs-after",
		"stop load balancers after this duration when workload proxying is enabled. Set to 0 to disable.", &cmdConfig.Services.WorkloadProxy.StopLBsAfter)

	rootCmdFlagBinder.boolVar("enable-break-glass-configs", "Allows downloading admin Talos and Kubernetes configs.", &cmdConfig.Features.EnableBreakGlassConfigs)

	rootCmdFlagBinder.boolVar("disable-controller-runtime-cache",
		"disable watch-based cache for controller-runtime (affects performance)", &cmdConfig.Features.DisableControllerRuntimeCache)

	rootCmdFlagBinder.boolVar("enable-cluster-import", "enable EXPERIMENTAL cluster import feature.", &cmdConfig.Features.EnableClusterImport)
}

func defineDebugFlags() {
	rootCmdFlagBinder.stringVar("pprof-bind-addr", "start pprof HTTP server on the defined address (\"\" if disabled).", &cmdConfig.Debug.Pprof.Endpoint)
	rootCmdFlagBinder.stringVar("debug-server-endpoint", "start debug HTTP server on the defined address (\"\" if disabled).", &cmdConfig.Debug.Server.Endpoint)
}

func defineEtcdBackupsFlags() {
	rootCmdFlagBinder.boolVar("etcd-backup-s3", "S3 will be used for cluster etcd backups", &cmdConfig.EtcdBackup.S3Enabled)
	rootCmdFlagBinder.stringVar("etcd-backup-local-path", "path to local directory for cluster etcd backups", &cmdConfig.EtcdBackup.LocalPath)
	rootCmdFlagBinder.durationVar("etcd-backup-tick-interval",
		"interval between etcd backups ticks (controller events to check if any cluster needs to be backed up)", &cmdConfig.EtcdBackup.TickInterval)
	rootCmdFlagBinder.durationVar("etcd-backup-jitter",
		"jitter for etcd backups, randomly added/subtracted from the interval between automatic etcd backups", &cmdConfig.EtcdBackup.Jitter)
	rootCmdFlagBinder.durationVar("etcd-backup-min-interval", "minimal interval between etcd backups", &cmdConfig.EtcdBackup.MinInterval)
	rootCmdFlagBinder.durationVar("etcd-backup-max-interval", "maximal interval between etcd backups", &cmdConfig.EtcdBackup.MaxInterval)
	rootCmdFlagBinder.uint64Var("etcd-backup-upload-limit-mbps", "throughput limit in Mbps for etcd backup uploads, zero means unlimited", &cmdConfig.EtcdBackup.UploadLimitMbps)
	rootCmdFlagBinder.uint64Var("etcd-backup-download-limit-mbps", "throughput limit in Mbps for etcd backup downloads, zero means unlimited", &cmdConfig.EtcdBackup.DownloadLimitMbps)

	rootCmd.MarkFlagsMutuallyExclusive("etcd-backup-s3", "etcd-backup-local-path")
}
