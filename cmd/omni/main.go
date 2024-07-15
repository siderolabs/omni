// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package main ...
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/go-debug"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"

	"github.com/siderolabs/omni/client/pkg/constants"
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
	"github.com/siderolabs/omni/internal/version"
)

func runDebugServer(ctx context.Context, logger *zap.Logger) {
	const debugAddr = ":9980"

	debugLogFunc := func(msg string) {
		logger.Info(msg)
	}

	if err := debug.ListenAndServe(ctx, debugAddr, debugLogFunc); err != nil {
		logger.Panic("failed to start debug server", zap.Error(err))
	}
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          "omni",
	Short:        "Talos and Sidero frontend",
	Long:         ``,
	SilenceUsage: true,
	Version:      version.Tag,
	RunE: func(*cobra.Command, []string) error {
		if config.Config.Auth.SAML.URL != "" && config.Config.Auth.SAML.Metadata != "" {
			return errors.New("flags --auth-saml-url and --auth-saml-metadata are mutually exclusive")
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

		// set kubernetes logger to use warn log level and use zap
		klog.SetLogger(zapr.NewLogger(logger.WithOptions(zap.IncreaseLevel(zapcore.WarnLevel)).With(logging.Component("kubernetes"))))

		if constants.IsDebugBuild {
			logger.Warn("running debug build")
		}

		for _, registryMirror := range rootCmdArgs.registryMirrors {
			hostname, endpoint, ok := strings.Cut(registryMirror, "=")
			if !ok {
				return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
			}

			config.Config.DefaultConfigGenOptions = append(config.Config.DefaultConfigGenOptions, generate.WithRegistryMirror(hostname, endpoint))
		}

		logger.Info("starting Omni", zap.String("version", version.Tag))

		logger.Debug("using config", zap.Any("config", config.Config))

		signals := make(chan os.Signal, 1)

		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

		ctx, stop := context.WithCancel(context.Background())
		defer stop()

		// do not use signal.NotifyContext as it doesn't support any ways to log the received signal
		panichandler.Go(func() {
			s := <-signals

			logger.Warn("signal received, stopping Omni", zap.String("signal", s.String()))

			stop()
		}, logger)

		panichandler.Go(func() {
			runDebugServer(ctx, logger)
		}, logger)

		// this global context propagates into all controllers and any other background activities
		ctx = actor.MarkContextAsInternalActor(ctx)

		err = omni.NewState(ctx, config.Config, logger, prometheus.DefaultRegisterer, runWithState(logger))
		if err != nil {
			return fmt.Errorf("failed to run Omni: %w", err)
		}

		return nil
	},
}

//nolint:gocognit
func runWithState(logger *zap.Logger) func(context.Context, state.State, *virtual.State) error {
	return func(ctx context.Context, resourceState state.State, virtualState *virtual.State) error {
		talosClientFactory := talos.NewClientFactory(resourceState, logger)
		prometheus.MustRegister(talosClientFactory)

		dnsService := dns.NewService(resourceState, logger)
		workloadProxyReconciler := workloadproxy.NewReconciler(logger.With(logging.Component("workload_proxy_reconciler")), zapcore.DebugLevel)

		var resourceLogger *resourcelogger.Logger

		if len(config.Config.LogResourceUpdatesTypes) > 0 {
			var err error

			resourceLogger, err = resourcelogger.New(ctx, resourceState, logger.With(logging.Component("resourcelogger")),
				config.Config.LogResourceUpdatesLogLevel, config.Config.LogResourceUpdatesTypes...)
			if err != nil {
				return fmt.Errorf("failed to set up resource logger: %w", err)
			}
		}

		imageFactoryClient, err := imagefactory.NewClient(resourceState, config.Config.ImageFactoryBaseURL)
		if err != nil {
			return fmt.Errorf("failed to set up image factory client: %w", err)
		}

		linkCounterDeltaCh := make(chan siderolink.LinkCounterDeltas)
		siderolinkEventsCh := make(chan *omnires.MachineStatusSnapshot)

		defaultDiscoveryClient, err := discovery.NewClient(discovery.Options{
			UseEmbeddedDiscoveryService: false,
		})
		if err != nil {
			return fmt.Errorf("failed to create default discovery client: %w", err)
		}

		var embeddedDiscoveryClient *discovery.Client

		if config.Config.EmbeddedDiscoveryService.Enabled {
			if embeddedDiscoveryClient, err = discovery.NewClient(discovery.Options{
				UseEmbeddedDiscoveryService:  true,
				EmbeddedDiscoveryServicePort: config.Config.EmbeddedDiscoveryService.Port,
			}); err != nil {
				return fmt.Errorf("failed to create embedded discovery client: %w", err)
			}
		}

		defer func() {
			if closeErr := defaultDiscoveryClient.Close(); closeErr != nil {
				logger.Error("failed to close discovery client", zap.Error(closeErr))
			}
		}()

		omniRuntime, err := omni.New(talosClientFactory, dnsService, workloadProxyReconciler, resourceLogger,
			imageFactoryClient, linkCounterDeltaCh, siderolinkEventsCh, resourceState, virtualState,
			prometheus.DefaultRegisterer, defaultDiscoveryClient, embeddedDiscoveryClient, logger.With(logging.Component("omni_runtime")))
		if err != nil {
			return fmt.Errorf("failed to set up the controller runtime: %w", err)
		}

		machineMap := siderolink.NewMachineMap(siderolink.NewStateStorage(omniRuntime.State()))

		logHandler, err := siderolink.NewLogHandler(
			machineMap,
			resourceState,
			&config.Config.MachineLogConfig,
			logger.With(logging.Component("siderolink_log_handler")),
		)
		if err != nil {
			return fmt.Errorf("failed to set up log handler: %w", err)
		}

		talosRuntime := talos.New(talosClientFactory, logger)

		err = user.EnsureInitialResources(ctx, omniRuntime.State(), logger, config.Config.InitialUsers)
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

		handler, err := backend.NewFrontendHandler(rootCmdArgs.frontendDst, logger)
		if err != nil {
			return fmt.Errorf("failed to set up frontend handler: %w", err)
		}

		server, err := backend.NewServer(
			rootCmdArgs.bindAddress,
			rootCmdArgs.metricsBindAddress,
			rootCmdArgs.k8sProxyBindAddress,
			rootCmdArgs.pprofBindAddress,
			dnsService,
			workloadProxyReconciler,
			imageFactoryClient,
			linkCounterDeltaCh,
			siderolinkEventsCh,
			omniRuntime,
			talosRuntime,
			logHandler,
			authConfig,
			rootCmdArgs.keyFile,
			rootCmdArgs.certFile,
			backend.NewProxyServer(rootCmdArgs.frontendBind, handler, rootCmdArgs.keyFile, rootCmdArgs.certFile),
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

var rootCmdArgs struct {
	bindAddress         string
	frontendBind        string
	frontendDst         string
	k8sProxyBindAddress string
	metricsBindAddress  string
	pprofBindAddress    string
	keyFile             string
	certFile            string
	registryMirrors     []string

	debug bool
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

//nolint:maintidx
func init() {
	rootCmd.Flags().BoolVar(&rootCmdArgs.debug, "debug", false, "enable debug logs.")
	rootCmd.Flags().StringVar(&rootCmdArgs.bindAddress, "bind-addr", "0.0.0.0:8080", "start HTTP server on the defined address.")
	rootCmd.Flags().StringVar(&rootCmdArgs.frontendDst, "frontend-dst", "", "destination address non API requests from proxy server.")
	rootCmd.Flags().StringVar(&rootCmdArgs.frontendBind, "frontend-bind", "", "proxy server which will redirect all non API requests to the definied frontend server.")
	rootCmd.Flags().StringVar(&rootCmdArgs.metricsBindAddress, "metrics-bind-addr", "0.0.0.0:2122", "start Prometheus HTTP server on the defined address.")
	rootCmd.Flags().StringVar(&rootCmdArgs.pprofBindAddress, "pprof-bind-addr", "", "start pprof HTTP server on the defined address (\"\" if disabled).")
	rootCmd.Flags().StringVar(&rootCmdArgs.k8sProxyBindAddress, "k8s-proxy-bind-addr", "0.0.0.0:8095", "start Kubernetes workload proxy on the defined address.")
	rootCmd.Flags().StringSliceVar(&rootCmdArgs.registryMirrors, "registry-mirror", []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")

	rootCmd.Flags().StringVar(&config.Config.AccountID, "account-id", config.Config.AccountID, "instance account ID, should never be changed.")
	rootCmd.Flags().StringVar(&config.Config.Name, "name", config.Config.Name, "instance user-facing name.")
	rootCmd.Flags().StringVar(&config.Config.APIURL, "advertised-api-url", config.Config.APIURL, "advertised API frontend URL.")
	rootCmd.Flags().StringVar(&config.Config.KubernetesProxyURL, "advertised-kubernetes-proxy-url", config.Config.KubernetesProxyURL, "advertised Kubernetes proxy URL.")
	rootCmd.Flags().BoolVar(&config.Config.SiderolinkDisableLastEndpoint, "siderolink-disable-last-endpoint", false, "do not populate last known peer endpoint for the wireguard peers")
	rootCmd.Flags().StringVar(
		&config.Config.SiderolinkWireguardAdvertisedAddress,
		"siderolink-wireguard-advertised-addr",
		config.Config.SiderolinkWireguardAdvertisedAddress,
		"advertised wireguard address which is passed down to the nodes.")
	rootCmd.Flags().StringVar(&config.Config.SiderolinkWireguardBindAddress, "siderolink-wireguard-bind-addr", config.Config.SiderolinkWireguardBindAddress, "Siderolink wireguard bind address.")
	rootCmd.Flags().BoolVar(&config.Config.SiderolinkUseGRPCTunnel, "siderolink-use-grpc-tunnel", false, "use gRPC tunnel to wrap wireguard traffic instead of UDP")

	rootCmd.Flags().StringVar(&config.Config.MachineAPIBindAddress, "siderolink-api-bind-addr", config.Config.MachineAPIBindAddress, "SideroLink provision bind address.")
	rootCmd.Flags().StringVar(&config.Config.MachineAPICertFile, "siderolink-api-cert", config.Config.MachineAPICertFile, "SideroLink TLS cert file path.")
	rootCmd.Flags().StringVar(&config.Config.MachineAPIKeyFile, "siderolink-api-key", config.Config.MachineAPIKeyFile, "SideroLink TLS key file path.")

	rootCmd.Flags().MarkDeprecated("siderolink-api-bind-addr", "--deprecated, use --machine-api-bind-addr") //nolint:errcheck
	rootCmd.Flags().MarkDeprecated("siderolink-api-cert", "deprecated, use --machine-api-cert")             //nolint:errcheck
	rootCmd.Flags().MarkDeprecated("siderolink-api-key", "deprecated, use --machine-api-key")               //nolint:errcheck

	rootCmd.Flags().StringVar(&config.Config.MachineAPIBindAddress, "machine-api-bind-addr", config.Config.MachineAPIBindAddress, "machine API bind address.")
	rootCmd.Flags().StringVar(&config.Config.MachineAPICertFile, "machine-api-cert", config.Config.MachineAPICertFile, "machine API TLS cert file path.")
	rootCmd.Flags().StringVar(&config.Config.MachineAPIKeyFile, "machine-api-key", config.Config.MachineAPIKeyFile, "machine API TLS key file path.")

	rootCmd.Flags().IntVar(&config.Config.EventSinkPort, "event-sink-port", config.Config.EventSinkPort, "event sink bind port.")
	rootCmd.Flags().StringVar(&config.Config.SideroLinkAPIURL, "siderolink-api-advertised-url", config.Config.SideroLinkAPIURL, "SideroLink advertised API URL.")
	rootCmd.Flags().IntVar(&config.Config.LoadBalancer.MinPort, "lb-min-port", config.Config.LoadBalancer.MinPort, "cluster load balancer port range min value.")
	rootCmd.Flags().IntVar(&config.Config.LoadBalancer.MaxPort, "lb-max-port", config.Config.LoadBalancer.MaxPort, "cluster load balancer port range max value.")
	rootCmd.Flags().IntVar(&config.Config.LogServerPort, "log-server-port", config.Config.LogServerPort, "port for TCP log server")

	rootCmd.Flags().IntVar(&config.Config.MachineLogConfig.BufferInitialCapacity, "machine-log-buffer-capacity",
		config.Config.MachineLogConfig.BufferInitialCapacity, "initial buffer capacity for machine logs in bytes")
	rootCmd.Flags().IntVar(&config.Config.MachineLogConfig.BufferMaxCapacity, "machine-log-buffer-max-capacity",
		config.Config.MachineLogConfig.BufferMaxCapacity, "max buffer capacity for machine logs in bytes")
	rootCmd.Flags().IntVar(&config.Config.MachineLogConfig.BufferSafetyGap, "machine-log-buffer-safe-gap",
		config.Config.MachineLogConfig.BufferSafetyGap, "safety gap for machine log buffer in bytes")
	rootCmd.Flags().IntVar(&config.Config.MachineLogConfig.NumCompressedChunks, "machine-log-num-compressed-chunks",
		config.Config.MachineLogConfig.NumCompressedChunks, "number of compressed log chunks to keep")
	rootCmd.Flags().BoolVar(&config.Config.MachineLogConfig.StorageEnabled, "machine-log-storage-enabled",
		config.Config.MachineLogConfig.StorageEnabled, "enable machine log storage")
	rootCmd.Flags().StringVar(&config.Config.MachineLogConfig.StoragePath, "machine-log-storage-path",
		config.Config.MachineLogConfig.StoragePath, "path of the directory for storing machine logs")
	rootCmd.Flags().DurationVar(&config.Config.MachineLogConfig.StorageFlushPeriod, "machine-log-storage-flush-period",
		config.Config.MachineLogConfig.StorageFlushPeriod, "period for flushing machine logs to disk")
	rootCmd.Flags().Float64Var(&config.Config.MachineLogConfig.StorageFlushJitter, "machine-log-storage-flush-jitter",
		config.Config.MachineLogConfig.StorageFlushJitter, "jitter for the machine log storage flush period")

	// keep the old flags for backwards-compatibility
	{
		rootCmd.Flags().BoolVar(&config.Config.MachineLogConfig.StorageEnabled, "log-storage-enabled", config.Config.MachineLogConfig.StorageEnabled, "enable machine log storage")
		rootCmd.Flags().StringVar(&config.Config.MachineLogConfig.StoragePath, "log-storage-path", config.Config.MachineLogConfig.StoragePath,
			"path of the directory for storing machine logs")
		rootCmd.Flags().DurationVar(&config.Config.MachineLogConfig.StorageFlushPeriod, "log-storage-flush-period", config.Config.MachineLogConfig.StorageFlushPeriod,
			"period for flushing machine logs to disk")

		rootCmd.Flags().MarkDeprecated("log-storage-enabled", "use --machine-log-storage-enabled")           //nolint:errcheck
		rootCmd.Flags().MarkDeprecated("log-storage-path", "use --machine-log-storage-path")                 //nolint:errcheck
		rootCmd.Flags().MarkDeprecated("log-storage-flush-period", "use --machine-log-storage-flush-period") //nolint:errcheck
	}

	rootCmd.Flags().BoolVar(&config.Config.Auth.Auth0.Enabled, "auth-auth0-enabled", config.Config.Auth.Auth0.Enabled,
		"enable Auth0 authentication. Once set to true, it cannot be set back to false.")
	rootCmd.Flags().StringVar(&config.Config.Auth.Auth0.ClientID, "auth-auth0-client-id", config.Config.Auth.Auth0.ClientID, "Auth0 application client ID.")
	rootCmd.Flags().StringVar(&config.Config.Auth.Auth0.Domain, "auth-auth0-domain", config.Config.Auth.Auth0.Domain, "Auth0 application domain.")
	rootCmd.Flags().BoolVar(&config.Config.Auth.Auth0.UseFormData, "auth-auth0-use-form-data", config.Config.Auth.Auth0.UseFormData,
		"When true, data to the token endpoint is transmitted as x-www-form-urlencoded data instead of JSON. The default is false")

	rootCmd.Flags().BoolVar(&config.Config.Auth.WebAuthn.Enabled, "auth-webauthn-enabled", config.Config.Auth.WebAuthn.Enabled,
		"enable WebAuthn authentication. Once set to true, it cannot be set back to false.")
	rootCmd.Flags().BoolVar(&config.Config.Auth.WebAuthn.Required, "auth-webauthn-required", config.Config.Auth.WebAuthn.Required,
		"require WebAuthn authentication. Once set to true, it cannot be set back to false.")

	rootCmd.Flags().BoolVar(&config.Config.Auth.SAML.Enabled, "auth-saml-enabled", config.Config.Auth.SAML.Enabled,
		"enabled SAML authentication.",
	)
	rootCmd.Flags().StringVar(&config.Config.Auth.SAML.URL, "auth-saml-url", config.Config.Auth.SAML.URL, "SAML identity provider metadata URL (mutually exclusive with --auth-saml-metadata")
	rootCmd.Flags().StringVar(&config.Config.Auth.SAML.Metadata, "auth-saml-metadata", config.Config.Auth.SAML.Metadata,
		"SAML identity provider metadata file path (mutually exclusive with --auth-saml-url).",
	)
	rootCmd.Flags().Var(&config.Config.Auth.SAML.LabelRules, "auth-saml-label-rules", "defines mapping of SAML assertion attributes into Omni identity labels")

	rootCmd.Flags().StringSliceVar(&config.Config.InitialUsers, "initial-users", config.Config.InitialUsers, "initial set of user emails. these users will be created on startup.")

	rootCmd.Flags().StringVar(&config.Config.Storage.Kind, "storage-kind", config.Config.Storage.Kind, "storage type: etcd|boltdb.")
	rootCmd.Flags().BoolVar(&config.Config.Storage.Etcd.Embedded, "etcd-embedded", config.Config.Storage.Etcd.Embedded, "use embedded etcd server.")
	rootCmd.Flags().BoolVar(&config.Config.Storage.Etcd.EmbeddedUnsafeFsync, "etcd-embedded-unsafe-fsync", config.Config.Storage.Etcd.EmbeddedUnsafeFsync,
		"disable fsync in the embedded etcd server (dangerous).")
	rootCmd.Flags().StringSliceVar(&config.Config.Storage.Etcd.Endpoints, "etcd-endpoints", config.Config.Storage.Etcd.Endpoints, "external etcd endpoints.")
	rootCmd.Flags().DurationVar(&config.Config.Storage.Etcd.DialKeepAliveTime,
		"etcd-dial-keepalive-time", config.Config.Storage.Etcd.DialKeepAliveTime, "external etcd client keep-alive time (interval).")
	rootCmd.Flags().DurationVar(&config.Config.Storage.Etcd.DialKeepAliveTimeout,
		"etcd-dial-keepalive-timeout", config.Config.Storage.Etcd.DialKeepAliveTimeout, "external etcd client keep-alive timeout.")
	rootCmd.Flags().StringVar(&config.Config.Storage.Etcd.CAPath, "etcd-ca-path", config.Config.Storage.Etcd.CAPath, "external etcd CA path.")
	rootCmd.Flags().StringVar(&config.Config.Storage.Etcd.CertPath, "etcd-client-cert-path", config.Config.Storage.Etcd.CertPath, "external etcd client cert path.")
	rootCmd.Flags().StringVar(&config.Config.Storage.Etcd.KeyPath, "etcd-client-key-path", config.Config.Storage.Etcd.KeyPath, "external etcd client key path.")

	rootCmd.Flags().StringVar(&config.Config.SecondaryStorage.Path, "secondary-storage-path", config.Config.SecondaryStorage.Path,
		"path of the file for boltdb-backed secondary storage for frequently updated data.")

	rootCmd.Flags().StringVar(&config.Config.TalosRegistry, "talos-installer-registry", config.Config.TalosRegistry, "Talos installer image registry.")
	rootCmd.Flags().StringVar(&config.Config.KubernetesRegistry, "kubernetes-registry", config.Config.KubernetesRegistry, "Kubernetes container registry.")
	rootCmd.Flags().StringVar(&config.Config.ImageFactoryBaseURL, "image-factory-address", config.Config.ImageFactoryBaseURL, "Image factory base URL to use.")
	rootCmd.Flags().StringVar(&config.Config.ImageFactoryPXEBaseURL, "image-factory-pxe-address", config.Config.ImageFactoryPXEBaseURL, "Image factory pxe base URL to use.")

	rootCmd.Flags().StringVar(
		&config.Config.Storage.Etcd.PrivateKeySource,
		"private-key-source",
		config.Config.Storage.Etcd.PrivateKeySource,
		"file containing private key to use for decrypting master key slot.",
	)
	rootCmd.Flags().StringSliceVar(
		&config.Config.Storage.Etcd.PublicKeyFiles,
		"public-key-files",
		config.Config.Storage.Etcd.PublicKeyFiles,
		"list of paths to files containing public keys to use for encrypting keys slots.",
	)

	rootCmd.Flags().DurationVar(
		&config.Config.KeyPruner.Interval,
		"public-key-pruning-interval",
		config.Config.KeyPruner.Interval,
		"interval between public key pruning runs.",
	)

	rootCmd.Flags().BoolVar(&config.Config.Auth.Suspended, "suspended", config.Config.Auth.Suspended, "start omni in suspended (read-only) mode.")

	rootCmd.Flags().BoolVar(&config.Config.EnableTalosPreReleaseVersions, "enable-talos-pre-release-versions", config.Config.EnableTalosPreReleaseVersions,
		"make Omni version discovery controler include Talos pre-release versions.")

	rootCmd.Flags().BoolVar(&config.Config.WorkloadProxying.Enabled, "workload-proxying-enabled", config.Config.WorkloadProxying.Enabled, "enable workload proxying feature.")
	rootCmd.Flags().StringVar(&config.Config.WorkloadProxying.Subdomain, "workload-proxying-subdomain", config.Config.WorkloadProxying.Subdomain, "workload proxying subdomain.")

	rootCmd.Flags().IntVar(&config.Config.LocalResourceServerPort, "local-resource-server-port", config.Config.LocalResourceServerPort, "port for local read-only public resource server.")

	ensure.NoError(rootCmd.MarkFlagRequired("private-key-source"))
	ensure.NoError(rootCmd.Flags().MarkHidden("etcd-embedded-unsafe-fsync"))

	rootCmd.Flags().StringVar(&rootCmdArgs.keyFile, "key", "", "TLS key file")
	rootCmd.Flags().StringVar(&rootCmdArgs.certFile, "cert", "", "TLS cert file")

	rootCmd.Flags().BoolVar(
		&config.Config.EtcdBackup.S3Enabled,
		"etcd-backup-s3",
		config.Config.EtcdBackup.S3Enabled,
		"S3 will be used for cluster etcd backups",
	)

	rootCmd.Flags().StringVar(
		&config.Config.EtcdBackup.LocalPath,
		"etcd-backup-local-path",
		config.Config.EtcdBackup.LocalPath,
		"path to local directory for cluster etcd backups",
	)

	rootCmd.MarkFlagsMutuallyExclusive("etcd-backup-s3", "etcd-backup-local-path")

	rootCmd.Flags().DurationVar(
		&config.Config.EtcdBackup.TickInterval,
		"etcd-backup-tick-interval",
		config.Config.EtcdBackup.TickInterval,
		"interval between etcd backups ticks (controller events to check if any cluster needs to be backed up)",
	)

	rootCmd.Flags().DurationVar(
		&config.Config.EtcdBackup.MinInterval,
		"etcd-backup-min-interval",
		config.Config.EtcdBackup.MinInterval,
		"minimal interval between etcd backups",
	)

	rootCmd.Flags().DurationVar(
		&config.Config.EtcdBackup.MaxInterval,
		"etcd-backup-max-interval",
		config.Config.EtcdBackup.MaxInterval,
		"maximal interval between etcd backups",
	)

	rootCmd.Flags().StringSliceVar(&config.Config.LogResourceUpdatesTypes,
		"log-resource-updates-types",
		config.Config.LogResourceUpdatesTypes,
		"list of resource types whose updates should be logged",
	)
	rootCmd.Flags().StringVar(&config.Config.LogResourceUpdatesLogLevel,
		"log-resource-updates-log-level",
		config.Config.LogResourceUpdatesLogLevel,
		"log level for resource updates",
	)

	rootCmd.Flags().BoolVar(&config.Config.DisableControllerRuntimeCache,
		"disable-controller-runtime-cache",
		config.Config.DisableControllerRuntimeCache,
		"disable watch-based cache for controller-runtime (affects performance)",
	)

	rootCmd.Flags().BoolVar(
		&config.Config.EmbeddedDiscoveryService.Enabled,
		"embedded-discovery-service-enabled",
		config.Config.EmbeddedDiscoveryService.Enabled,
		"enable embedded discovery service, binds only to the siderolink wireguard address",
	)
	rootCmd.Flags().IntVar(
		&config.Config.EmbeddedDiscoveryService.Port,
		"embedded-discovery-service-endpoint",
		config.Config.EmbeddedDiscoveryService.Port,
		"embedded discovery service port to listen on",
	)
	rootCmd.Flags().BoolVar(
		&config.Config.EmbeddedDiscoveryService.SnapshotsEnabled,
		"embedded-discovery-service-snapshots-enabled",
		config.Config.EmbeddedDiscoveryService.SnapshotsEnabled,
		"enable snapshots for the embedded discovery service",
	)
	rootCmd.Flags().StringVar(
		&config.Config.EmbeddedDiscoveryService.SnapshotPath,
		"embedded-discovery-service-snapshot-path",
		config.Config.EmbeddedDiscoveryService.SnapshotPath,
		"path to the file for storing the embedded discovery service state",
	)
	rootCmd.Flags().DurationVar(
		&config.Config.EmbeddedDiscoveryService.SnapshotInterval,
		"embedded-discovery-service-snapshot-interval",
		config.Config.EmbeddedDiscoveryService.SnapshotInterval,
		"interval for saving the embedded discovery service state",
	)
	rootCmd.Flags().StringVar(
		&config.Config.EmbeddedDiscoveryService.LogLevel,
		"embedded-discovery-service-log-level",
		config.Config.EmbeddedDiscoveryService.LogLevel,
		"log level for the embedded discovery service - it has no effect if it is lower (more verbose) than the main log level",
	)

	rootCmd.Flags().BoolVar(
		&config.Config.EnableBreakGlassConfigs,
		"enable-break-glass-configs",
		config.Config.EnableBreakGlassConfigs,
		"Allows downloading admin Talos and Kubernetes configs.",
	)
}
