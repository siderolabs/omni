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
	"github.com/siderolabs/go-pointer"
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

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          "omni",
	Short:        "Omni Kubernetes management platform service",
	Long:         "This executable runs both Omni frontend and Omni backend",
	SilenceUsage: true,
	Version:      version.Tag,
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
	rootCmd.Flags().BoolVar(&rootCmdArgs.debug, "debug", constants.IsDebugBuild, "enable debug logs.")

	rootCmd.Flags().StringVar(&cmdConfig.Account.ID, "account-id", cmdConfig.Account.ID, "instance account ID, should never be changed.")
	rootCmd.Flags().StringVar(&cmdConfig.Account.Name, "name", cmdConfig.Account.Name, "instance user-facing name.")
	rootCmd.Flags().StringVar(&cmdConfig.Account.UserPilot.AppToken, "user-pilot-app-token", cmdConfig.Account.UserPilot.AppToken, "user pilot app token.")

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
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.API.BindEndpoint,
		"bind-addr",
		cmdConfig.Services.API.BindEndpoint,
		"start HTTP + API server on the defined address.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.API.AdvertisedURL,
		"advertised-api-url",
		cmdConfig.Services.API.AdvertisedURL,
		"advertised API frontend URL.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.API.KeyFile,
		"key",
		cmdConfig.Services.API.KeyFile,
		"TLS key file",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.API.CertFile,
		"cert",
		cmdConfig.Services.API.CertFile,
		"TLS cert file",
	)

	// Metrics
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.Metrics.BindEndpoint,
		"metrics-bind-addr",
		cmdConfig.Services.Metrics.BindEndpoint,
		"start Prometheus HTTP server on the defined address.",
	)

	// KubernetesProxy
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.KubernetesProxy.BindEndpoint,
		"k8s-proxy-bind-addr", cmdConfig.Services.KubernetesProxy.BindEndpoint,
		"start Kubernetes proxy on the defined address.",
	)

	rootCmd.Flags().StringVar(
		&cmdConfig.Services.KubernetesProxy.AdvertisedURL,
		"advertised-kubernetes-proxy-url",
		cmdConfig.Services.KubernetesProxy.AdvertisedURL,
		"advertised Kubernetes proxy URL.",
	)

	// Siderolink
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.Siderolink.WireGuard.BindEndpoint,
		"siderolink-wireguard-bind-addr",
		cmdConfig.Services.Siderolink.WireGuard.BindEndpoint,
		"SideroLink WireGuard bind address.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.Siderolink.WireGuard.AdvertisedEndpoint,
		"siderolink-wireguard-advertised-addr",
		cmdConfig.Services.Siderolink.WireGuard.AdvertisedEndpoint,
		"advertised wireguard address which is passed down to the nodes.",
	)
	rootCmd.Flags().BoolVar(
		&cmdConfig.Services.Siderolink.DisableLastEndpoint,
		"siderolink-disable-last-endpoint",
		cmdConfig.Services.Siderolink.DisableLastEndpoint,
		"do not populate last known peer endpoint for the WireGuard peers",
	)
	rootCmd.Flags().BoolVar(
		&cmdConfig.Services.Siderolink.UseGRPCTunnel,
		"siderolink-use-grpc-tunnel",
		cmdConfig.Services.Siderolink.UseGRPCTunnel,
		"use gRPC tunnel to wrap WireGuard traffic instead of UDP. When enabled, "+
			"the SideroLink connections from Talos machines will be configured to use the tunnel mode, regardless of their individual configuration. ",
	)
	rootCmd.Flags().IntVar(
		&cmdConfig.Services.Siderolink.EventSinkPort,
		"event-sink-port",
		cmdConfig.Services.Siderolink.EventSinkPort,
		"event sink bind port.",
	)
	rootCmd.Flags().IntVar(
		&cmdConfig.Services.Siderolink.LogServerPort,
		"log-server-port",
		cmdConfig.Services.Siderolink.LogServerPort,
		"port for TCP log server",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.Siderolink.JoinTokensMode,
		"join-tokens-mode",
		cmdConfig.Services.Siderolink.JoinTokensMode,
		"configures Talos machine join flow to use secure node tokens",
	)

	// MachineAPI
	for _, prefix := range []string{"siderolink", "machine"} {
		rootCmd.Flags().StringVar(
			&cmdConfig.Services.MachineAPI.BindEndpoint,
			fmt.Sprintf("%s-api-bind-addr", prefix),
			cmdConfig.Services.MachineAPI.BindEndpoint,
			"machine API provision bind address.",
		)
		rootCmd.Flags().StringVar(
			&cmdConfig.Services.MachineAPI.CertFile,
			fmt.Sprintf("%s-api-cert", prefix),
			cmdConfig.Services.MachineAPI.CertFile,
			"machine API TLS cert file path.",
		)
		rootCmd.Flags().StringVar(
			&cmdConfig.Services.MachineAPI.KeyFile,
			fmt.Sprintf("%s-api-key", prefix),
			cmdConfig.Services.MachineAPI.KeyFile,
			"machine API TLS cert file path.",
		)
		rootCmd.Flags().StringVar(
			&cmdConfig.Services.MachineAPI.AdvertisedURL,
			fmt.Sprintf("%s-api-advertised-url", prefix),
			cmdConfig.Services.MachineAPI.AdvertisedURL,
			"machine API advertised API URL.",
		)
	}

	rootCmd.Flags().MarkDeprecated("siderolink-api-bind-addr", "--deprecated, use --machine-api-bind-addr") //nolint:errcheck
	rootCmd.Flags().MarkDeprecated("siderolink-api-cert", "deprecated, use --machine-api-cert")             //nolint:errcheck
	rootCmd.Flags().MarkDeprecated("siderolink-api-key", "deprecated, use --machine-api-key")               //nolint:errcheck

	// LoadBalancer
	rootCmd.Flags().IntVar(
		&cmdConfig.Services.LoadBalancer.MinPort,
		"lb-min-port",
		cmdConfig.Services.LoadBalancer.MinPort,
		"cluster load balancer port range min value.",
	)
	rootCmd.Flags().IntVar(
		&cmdConfig.Services.LoadBalancer.MaxPort,
		"lb-max-port",
		cmdConfig.Services.LoadBalancer.MaxPort,
		"cluster load balancer port range max value.",
	)

	rootCmd.Flags().IntVar(
		&cmdConfig.Services.LocalResourceService.Port,
		"local-resource-server-port",
		cmdConfig.Services.LocalResourceService.Port,
		"port for local read-only public resource server.",
	)

	rootCmd.Flags().BoolVar(
		&cmdConfig.Services.LocalResourceService.Enabled,
		"local-resource-server-enabled",
		cmdConfig.Services.LocalResourceService.Enabled,
		"enable local read-only public resource server.",
	)

	// Embedded discovery service
	rootCmd.Flags().BoolVar(
		&cmdConfig.Services.EmbeddedDiscoveryService.Enabled,
		"embedded-discovery-service-enabled",
		cmdConfig.Services.EmbeddedDiscoveryService.Enabled,
		"enable embedded discovery service, binds only to the SideroLink WireGuard address",
	)

	rootCmd.Flags().IntVar(
		&cmdConfig.Services.EmbeddedDiscoveryService.Port,
		"embedded-discovery-service-endpoint",
		cmdConfig.Services.EmbeddedDiscoveryService.Port,
		"embedded discovery service port to listen on",
	)
	rootCmd.Flags().MarkDeprecated("embedded-discovery-service-endpoint", "use --embedded-discovery-service-port") //nolint:errcheck

	rootCmd.Flags().IntVar(
		&cmdConfig.Services.EmbeddedDiscoveryService.Port,
		"embedded-discovery-service-port",
		cmdConfig.Services.EmbeddedDiscoveryService.Port,
		"embedded discovery service port to listen on",
	)

	rootCmd.Flags().BoolVar(
		&cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsEnabled,
		"embedded-discovery-service-snapshots-enabled",
		cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsEnabled,
		"enable snapshots for the embedded discovery service",
	)

	//nolint:staticcheck
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsPath,
		"embedded-discovery-service-snapshot-path",
		cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsPath,
		"path to the file for storing the embedded discovery service state",
	)

	//nolint:errcheck
	rootCmd.Flags().MarkDeprecated("embedded-discovery-service-snapshot-path", "this flag is kept for the SQLite migration, and will be removed in future versions")

	rootCmd.Flags().DurationVar(
		&cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsInterval,
		"embedded-discovery-service-snapshot-interval",
		cmdConfig.Services.EmbeddedDiscoveryService.SnapshotsInterval,
		"interval for saving the embedded discovery service state",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.EmbeddedDiscoveryService.LogLevel,
		"embedded-discovery-service-log-level",
		cmdConfig.Services.EmbeddedDiscoveryService.LogLevel,
		"log level for the embedded discovery service - it has no effect if it is lower (more verbose) than the main log level",
	)
	rootCmd.Flags().DurationVar(
		&cmdConfig.Services.EmbeddedDiscoveryService.SQLiteTimeout,
		"embedded-discovery-service-sqlite-timeout",
		cmdConfig.Services.EmbeddedDiscoveryService.SQLiteTimeout,
		"SQLite timeout for the embedded discovery service",
	)

	// DevServerProxy
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.DevServerProxy.ProxyTo,
		"frontend-dst",
		cmdConfig.Services.DevServerProxy.ProxyTo,
		"destination address non API requests from proxy server.")
	rootCmd.Flags().StringVar(
		&cmdConfig.Services.DevServerProxy.BindEndpoint,
		"frontend-bind",
		cmdConfig.Services.DevServerProxy.BindEndpoint,
		"proxy server which will redirect all non API requests to the defined frontend server.")
}

func defineAuthFlags() {
	// Auth0
	rootCmd.Flags().BoolVar(&cmdConfig.Auth.Auth0.Enabled, "auth-auth0-enabled", cmdConfig.Auth.Auth0.Enabled,
		"enable Auth0 authentication. Once set to true, it cannot be set back to false.")
	rootCmd.Flags().StringVar(&cmdConfig.Auth.Auth0.ClientID, "auth-auth0-client-id", cmdConfig.Auth.Auth0.ClientID, "Auth0 application client ID.")
	rootCmd.Flags().StringVar(&cmdConfig.Auth.Auth0.Domain, "auth-auth0-domain", cmdConfig.Auth.Auth0.Domain, "Auth0 application domain.")
	rootCmd.Flags().BoolVar(&cmdConfig.Auth.Auth0.UseFormData, "auth-auth0-use-form-data", cmdConfig.Auth.Auth0.UseFormData,
		"When true, data to the token endpoint is transmitted as x-www-form-urlencoded data instead of JSON. The default is false")

	// Webauthn
	rootCmd.Flags().BoolVar(&cmdConfig.Auth.WebAuthn.Enabled, "auth-webauthn-enabled", cmdConfig.Auth.WebAuthn.Enabled,
		"enable WebAuthn authentication. Once set to true, it cannot be set back to false.")
	rootCmd.Flags().BoolVar(&cmdConfig.Auth.WebAuthn.Required, "auth-webauthn-required", cmdConfig.Auth.WebAuthn.Required,
		"require WebAuthn authentication. Once set to true, it cannot be set back to false.")

	rootCmd.Flags().BoolVar(&cmdConfig.Auth.SAML.Enabled, "auth-saml-enabled", cmdConfig.Auth.SAML.Enabled,
		"enabled SAML authentication.",
	)
	rootCmd.Flags().StringVar(&cmdConfig.Auth.SAML.MetadataURL, "auth-saml-url", cmdConfig.Auth.SAML.MetadataURL, "SAML identity provider metadata URL (mutually exclusive with --auth-saml-metadata")
	rootCmd.Flags().StringVar(&cmdConfig.Auth.SAML.Metadata, "auth-saml-metadata", cmdConfig.Auth.SAML.Metadata,
		"SAML identity provider metadata file path (mutually exclusive with --auth-saml-url).",
	)
	rootCmd.Flags().Var(
		&cmdConfig.Auth.SAML.LabelRules,
		"auth-saml-label-rules",
		"defines mapping of SAML assertion attributes into Omni identity labels",
	)
	rootCmd.Flags().Var(
		&cmdConfig.Auth.SAML.AttributeRules,
		"auth-saml-attribute-rules",
		"defines additional identity, fullname, firstname and lastname mappings",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Auth.SAML.NameIDFormat,
		"auth-saml-name-id-format",
		cmdConfig.Auth.SAML.NameIDFormat,
		"allows setting name ID format for the account",
	)
	rootCmd.Flags().StringSliceVar(
		&cmdConfig.Auth.InitialUsers,
		"initial-users",
		cmdConfig.Auth.InitialUsers,
		"initial set of user emails. these users will be created on startup.",
	)
	rootCmd.Flags().DurationVar(
		&cmdConfig.Auth.KeyPruner.Interval,
		"public-key-pruning-interval",
		cmdConfig.Auth.KeyPruner.Interval,
		"interval between public key pruning runs.",
	)
	rootCmd.Flags().BoolVar(
		&cmdConfig.Auth.Suspended,
		"suspended",
		cmdConfig.Auth.Suspended,
		"start omni in suspended (read-only) mode.",
	)

	rootCmd.Flags().BoolVar(
		&cmdConfig.Auth.InitialServiceAccount.Enabled,
		"create-initial-service-account",
		cmdConfig.Auth.InitialServiceAccount.Enabled,
		"create and dump a service account credentials on the first start of Omni",
	)

	rootCmd.Flags().StringVar(
		&cmdConfig.Auth.InitialServiceAccount.KeyPath,
		"initial-service-account-key-path",
		cmdConfig.Auth.InitialServiceAccount.KeyPath,
		"dump the initial service account key into the path",
	)

	rootCmd.Flags().StringVar(
		&cmdConfig.Auth.InitialServiceAccount.Role,
		"initial-service-account-role",
		cmdConfig.Auth.InitialServiceAccount.Role,
		"the initial service account access role",
	)

	rootCmd.Flags().DurationVar(
		&cmdConfig.Auth.InitialServiceAccount.Lifetime,
		"initial-service-account-lifetime",
		cmdConfig.Auth.InitialServiceAccount.Lifetime,
		"the lifetime duration of the initial service account key",
	)

	rootCmd.MarkFlagsMutuallyExclusive("auth-saml-url", "auth-saml-metadata")

	// OIDC
	rootCmd.Flags().BoolVar(&cmdConfig.Auth.OIDC.Enabled, "auth-oidc-enabled", cmdConfig.Auth.OIDC.Enabled,
		"enable OIDC authentication.")

	rootCmd.Flags().StringVar(&cmdConfig.Auth.OIDC.ProviderURL, "auth-oidc-provider-url", cmdConfig.Auth.OIDC.ProviderURL,
		"OIDC authentication provider URL.")

	rootCmd.Flags().StringVar(&cmdConfig.Auth.OIDC.ClientID, "auth-oidc-client-id", cmdConfig.Auth.OIDC.ClientID,
		"OIDC authentication client ID.")

	rootCmd.Flags().StringVar(&cmdConfig.Auth.OIDC.ClientSecret, "auth-oidc-client-secret", cmdConfig.Auth.OIDC.ClientSecret,
		"OIDC authentication client secret.")

	rootCmd.Flags().StringSliceVar(&cmdConfig.Auth.OIDC.Scopes, "auth-oidc-scopes", cmdConfig.Auth.OIDC.Scopes,
		"OIDC authentication scopes.")

	rootCmd.Flags().StringVar(&cmdConfig.Auth.OIDC.LogoutURL, "auth-oidc-logout-url", cmdConfig.Auth.OIDC.LogoutURL,
		"OIDC logout URL.")

	rootCmd.Flags().BoolVar(&cmdConfig.Auth.OIDC.AllowUnverifiedEmail, "auth-oidc-allow-unverified-email", cmdConfig.Auth.OIDC.AllowUnverifiedEmail,
		"Allow OIDC tokens without email_verified claim.")
}

//nolint:staticcheck // defineLogsFlags uses deprecated fields for backwards-compatibility
func defineLogsFlags() {
	rootCmd.Flags().DurationVar(&cmdConfig.Logs.Machine.Storage.SQLiteTimeout, "machine-log-sqlite-timeout",
		cmdConfig.Logs.Machine.Storage.SQLiteTimeout, "sqlite timeout for machine logs")
	rootCmd.Flags().DurationVar(&cmdConfig.Logs.Machine.Storage.CleanupInterval, "machine-log-cleanup-interval",
		cmdConfig.Logs.Machine.Storage.CleanupInterval, "interval between machine log cleanup runs")
	rootCmd.Flags().DurationVar(&cmdConfig.Logs.Machine.Storage.CleanupOlderThan, "machine-log-cleanup-older-than",
		cmdConfig.Logs.Machine.Storage.CleanupOlderThan, "age threshold for machine log entries to be cleaned up")
	rootCmd.Flags().IntVar(&cmdConfig.Logs.Machine.Storage.MaxLinesPerMachine, "machine-log-max-lines-per-machine",
		cmdConfig.Logs.Machine.Storage.MaxLinesPerMachine, "maximum number of log lines to keep per machine")
	rootCmd.Flags().Float64Var(&cmdConfig.Logs.Machine.Storage.CleanupProbability, "machine-log-cleanup-probability",
		cmdConfig.Logs.Machine.Storage.CleanupProbability, "probability of running a size-based cleanup after each log write")

	rootCmd.Flags().MarkHidden("machine-log-cleanup-probability") //nolint:errcheck

	rootCmd.Flags().StringSliceVar(&cmdConfig.Logs.ResourceLogger.Types, "log-resource-updates-types",
		cmdConfig.Logs.ResourceLogger.Types, "list of resource types whose updates should be logged")
	rootCmd.Flags().StringVar(&cmdConfig.Logs.ResourceLogger.LogLevel, "log-resource-updates-log-level", cmdConfig.Logs.ResourceLogger.LogLevel, "log level for resource updates")

	rootCmd.Flags().BoolVar(cmdConfig.Logs.Audit.Enabled, "audit-log-enabled", pointer.SafeDeref(cmdConfig.Logs.Audit.Enabled), "enable audit logging")

	rootCmd.Flags().DurationVar(&cmdConfig.Logs.Audit.SQLiteTimeout, "audit-log-sqlite-timeout", cmdConfig.Logs.Audit.SQLiteTimeout, "sqlite timeout for audit logs")

	rootCmd.Flags().BoolVar(&cmdConfig.Logs.Stripe.Enabled, "enable-stripe-reporting", cmdConfig.Logs.Stripe.Enabled, "enable Stripe machine usage reporting")

	rootCmd.Flags().Uint32Var(&cmdConfig.Logs.Stripe.MinCommit, "stripe-minimum-commit", cmdConfig.Logs.Stripe.MinCommit, "Minimum number of machines to report to Stripe for the given account")

	// Deprecated logs flags, kept for backwards-compatibility
	//
	//nolint:staticcheck,errcheck
	{
		rootCmd.Flags().StringVar(&cmdConfig.Logs.Audit.Path, "audit-log-dir", cmdConfig.Logs.Audit.Path, "Directory for audit log storage")
		rootCmd.Flags().IntVar(&cmdConfig.Logs.Machine.BufferInitialCapacity, "machine-log-buffer-capacity",
			cmdConfig.Logs.Machine.BufferInitialCapacity, "initial buffer capacity for machine logs in bytes")
		rootCmd.Flags().IntVar(&cmdConfig.Logs.Machine.BufferMaxCapacity, "machine-log-buffer-max-capacity",
			cmdConfig.Logs.Machine.BufferMaxCapacity, "max buffer capacity for machine logs in bytes")
		rootCmd.Flags().IntVar(&cmdConfig.Logs.Machine.BufferSafetyGap, "machine-log-buffer-safe-gap",
			cmdConfig.Logs.Machine.BufferSafetyGap, "safety gap for machine log buffer in bytes")
		rootCmd.Flags().IntVar(&cmdConfig.Logs.Machine.Storage.NumCompressedChunks, "machine-log-num-compressed-chunks",
			cmdConfig.Logs.Machine.Storage.NumCompressedChunks, "number of compressed log chunks to keep")
		rootCmd.Flags().BoolVar(&cmdConfig.Logs.Machine.Storage.Enabled, "machine-log-storage-enabled",
			cmdConfig.Logs.Machine.Storage.Enabled, "enable machine log storage")
		rootCmd.Flags().StringVar(&cmdConfig.Logs.Machine.Storage.Path, "machine-log-storage-path",
			cmdConfig.Logs.Machine.Storage.Path, "path of the directory for storing machine logs")
		rootCmd.Flags().StringVar(&cmdConfig.Logs.Machine.Storage.Path, "log-storage-path", cmdConfig.Logs.Machine.Storage.Path, "path of the directory for storing machine logs")

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
	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.Default.Kind,
		"storage-kind",
		cmdConfig.Storage.Default.Kind,
		"storage type: etcd|boltdb.",
	)
	rootCmd.Flags().BoolVar(
		&cmdConfig.Storage.Default.Etcd.Embedded,
		"etcd-embedded",
		cmdConfig.Storage.Default.Etcd.Embedded,
		"use embedded etcd server.",
	)
	rootCmd.Flags().BoolVar(
		&cmdConfig.Storage.Default.Etcd.EmbeddedUnsafeFsync,
		"etcd-embedded-unsafe-fsync",
		cmdConfig.Storage.Default.Etcd.EmbeddedUnsafeFsync,
		"disable fsync in the embedded etcd server (dangerous).",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.Default.Etcd.EmbeddedDBPath,
		"etcd-embedded-db-path",
		cmdConfig.Storage.Default.Etcd.EmbeddedDBPath,
		"path to the embedded etcd database.",
	)

	ensure.NoError(rootCmd.Flags().MarkHidden("etcd-embedded-unsafe-fsync"))

	rootCmd.Flags().StringSliceVar(
		&cmdConfig.Storage.Default.Etcd.Endpoints,
		"etcd-endpoints",
		cmdConfig.Storage.Default.Etcd.Endpoints,
		"external etcd endpoints.",
	)
	rootCmd.Flags().DurationVar(
		&cmdConfig.Storage.Default.Etcd.DialKeepAliveTime,
		"etcd-dial-keepalive-time",
		cmdConfig.Storage.Default.Etcd.DialKeepAliveTime,
		"external etcd client keep-alive time (interval).",
	)
	rootCmd.Flags().DurationVar(
		&cmdConfig.Storage.Default.Etcd.DialKeepAliveTimeout,
		"etcd-dial-keepalive-timeout",
		cmdConfig.Storage.Default.Etcd.DialKeepAliveTimeout,
		"external etcd client keep-alive timeout.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.Default.Etcd.CAFile,
		"etcd-ca-path",
		cmdConfig.Storage.Default.Etcd.CAFile,
		"external etcd CA path.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.Default.Etcd.CertFile,
		"etcd-client-cert-path",
		cmdConfig.Storage.Default.Etcd.CertFile,
		"external etcd client cert path.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.Default.Etcd.KeyFile,
		"etcd-client-key-path",
		cmdConfig.Storage.Default.Etcd.KeyFile,
		"external etcd client key path.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.Default.Etcd.PrivateKeySource,
		"private-key-source",
		cmdConfig.Storage.Default.Etcd.PrivateKeySource,
		"file containing private key to use for decrypting master key slot.",
	)

	rootCmd.Flags().StringSliceVar(
		&cmdConfig.Storage.Default.Etcd.PublicKeyFiles,
		"public-key-files",
		cmdConfig.Storage.Default.Etcd.PublicKeyFiles,
		"list of paths to files containing public keys to use for encrypting keys slots.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.Secondary.Path, //nolint:staticcheck // backwards compatibility, remove when migration from boltdb to sqlite is done
		"secondary-storage-path",
		cmdConfig.Storage.Secondary.Path, //nolint:staticcheck // backwards compatibility, remove when migration from boltdb to sqlite is done
		fmt.Sprintf("path of the file for boltdb-backed secondary storage for frequently updated data (deprecated, see --%s).", config.SQLiteStoragePathFlag),
	)

	rootCmd.Flags().MarkDeprecated("secondary-storage-path", "this flag is kept for the SQLite migration, and will be removed in future versions") //nolint:errcheck

	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.SQLite.Path,
		config.SQLiteStoragePathFlag,
		cmdConfig.Storage.SQLite.Path,
		"path of the file for sqlite-backed secondary storage for frequently updated data, machine and audit logs, it is required to be set.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.SQLite.ExperimentalBaseParams,
		"sqlite-storage-experimental-base-params",
		cmdConfig.Storage.SQLite.ExperimentalBaseParams,
		"base DSN (connection string) parameters for sqlite database connection. they must not start with question mark (?). "+
			"this flag is experimental and may be removed in future releases.",
	)

	rootCmd.Flags().StringVar(
		&cmdConfig.Storage.SQLite.ExtraParams,
		"sqlite-storage-extra-params",
		cmdConfig.Storage.SQLite.ExtraParams,
		"extra DSN (connection string) parameters for sqlite database connection. they will be appended to the base params. they must not start with ampersand (&).",
	)
}

func defineRegistriesFlags() {
	rootCmd.Flags().StringVar(
		&cmdConfig.Registries.Talos,
		"talos-installer-registry",
		cmdConfig.Registries.Talos,
		"Talos installer image registry.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Registries.Kubernetes,
		"kubernetes-registry",
		cmdConfig.Registries.Kubernetes,
		"Kubernetes container registry.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Registries.ImageFactoryBaseURL,
		"image-factory-address",
		cmdConfig.Registries.ImageFactoryBaseURL,
		"Image factory base URL to use.",
	)
	rootCmd.Flags().StringVar(
		&cmdConfig.Registries.ImageFactoryPXEBaseURL,
		"image-factory-pxe-address",
		cmdConfig.Registries.ImageFactoryPXEBaseURL,
		"Image factory pxe base URL to use.",
	)

	rootCmd.Flags().StringSliceVar(
		&cmdConfig.Registries.Mirrors,
		"registry-mirror",
		cmdConfig.Registries.Mirrors,
		"list of registry mirrors to use in format: <registry host>=<mirror URL>",
	)
}

func defineFeatureFlags() {
	rootCmd.Flags().BoolVar(
		&cmdConfig.Features.EnableTalosPreReleaseVersions,
		"enable-talos-pre-release-versions",
		cmdConfig.Features.EnableTalosPreReleaseVersions,
		"make Omni version discovery controler include Talos pre-release versions.",
	)

	rootCmd.Flags().BoolVar(
		&cmdConfig.Features.EnableConfigDataCompression,
		"config-data-compression-enabled",
		cmdConfig.Features.EnableConfigDataCompression,
		"enable config data compression.",
	)

	rootCmd.Flags().BoolVar(
		&cmdConfig.Services.WorkloadProxy.Enabled,
		"workload-proxying-enabled",
		cmdConfig.Services.WorkloadProxy.Enabled,
		"enable workload proxying feature.",
	)

	rootCmd.Flags().StringVar(
		&cmdConfig.Services.WorkloadProxy.Subdomain,
		"workload-proxying-subdomain",
		cmdConfig.Services.WorkloadProxy.Subdomain,
		"workload proxying subdomain.",
	)

	rootCmd.Flags().DurationVar(
		&cmdConfig.Services.WorkloadProxy.StopLBsAfter,
		"workload-proxying-stop-lbs-after",
		cmdConfig.Services.WorkloadProxy.StopLBsAfter,
		"stop load balancers after this duration when workload proxying is enabled. Set to 0 to disable.",
	)

	rootCmd.Flags().BoolVar(
		&cmdConfig.Features.EnableBreakGlassConfigs,
		"enable-break-glass-configs",
		cmdConfig.Features.EnableBreakGlassConfigs,
		"Allows downloading admin Talos and Kubernetes configs.",
	)

	rootCmd.Flags().BoolVar(
		&cmdConfig.Features.DisableControllerRuntimeCache,
		"disable-controller-runtime-cache",
		cmdConfig.Features.DisableControllerRuntimeCache,
		"disable watch-based cache for controller-runtime (affects performance)",
	)

	rootCmd.Flags().BoolVar(
		&cmdConfig.Features.EnableClusterImport,
		"enable-cluster-import",
		cmdConfig.Features.EnableClusterImport,
		"enable EXPERIMENTAL cluster import feature.",
	)
}

func defineDebugFlags() {
	rootCmd.Flags().StringVar(
		&cmdConfig.Debug.Pprof.Endpoint,
		"pprof-bind-addr",
		cmdConfig.Debug.Pprof.Endpoint,
		"start pprof HTTP server on the defined address (\"\" if disabled).",
	)

	rootCmd.Flags().StringVar(
		&cmdConfig.Debug.Server.Endpoint,
		"debug-server-endpoint",
		cmdConfig.Debug.Server.Endpoint,
		"start debug HTTP server on the defined address (\"\" if disabled).",
	)
}

func defineEtcdBackupsFlags() {
	rootCmd.Flags().BoolVar(
		&cmdConfig.EtcdBackup.S3Enabled,
		"etcd-backup-s3",
		cmdConfig.EtcdBackup.S3Enabled,
		"S3 will be used for cluster etcd backups",
	)

	rootCmd.Flags().StringVar(
		&cmdConfig.EtcdBackup.LocalPath,
		"etcd-backup-local-path",
		cmdConfig.EtcdBackup.LocalPath,
		"path to local directory for cluster etcd backups",
	)

	rootCmd.Flags().DurationVar(
		&cmdConfig.EtcdBackup.TickInterval,
		"etcd-backup-tick-interval",
		cmdConfig.EtcdBackup.TickInterval,
		"interval between etcd backups ticks (controller events to check if any cluster needs to be backed up)",
	)

	rootCmd.Flags().DurationVar(
		&cmdConfig.EtcdBackup.Jitter,
		"etcd-backup-jitter",
		cmdConfig.EtcdBackup.Jitter,
		"jitter for etcd backups, randomly added/subtracted from the interval between automatic etcd backups",
	)

	rootCmd.Flags().DurationVar(
		&cmdConfig.EtcdBackup.MinInterval,
		"etcd-backup-min-interval",
		cmdConfig.EtcdBackup.MinInterval,
		"minimal interval between etcd backups",
	)

	rootCmd.Flags().DurationVar(
		&cmdConfig.EtcdBackup.MaxInterval,
		"etcd-backup-max-interval",
		cmdConfig.EtcdBackup.MaxInterval,
		"maximal interval between etcd backups",
	)

	rootCmd.Flags().Uint64Var(
		&cmdConfig.EtcdBackup.UploadLimitMbps,
		"etcd-backup-upload-limit-mbps",
		cmdConfig.EtcdBackup.UploadLimitMbps,
		"throughput limit in Mbps for etcd backup uploads, zero means unlimited",
	)

	rootCmd.Flags().Uint64Var(
		&cmdConfig.EtcdBackup.DownloadLimitMbps,
		"etcd-backup-download-limit-mbps",
		cmdConfig.EtcdBackup.DownloadLimitMbps,
		"throughput limit in Mbps for etcd backup downloads, zero means unlimited",
	)

	rootCmd.MarkFlagsMutuallyExclusive("etcd-backup-s3", "etcd-backup-local-path")
}
