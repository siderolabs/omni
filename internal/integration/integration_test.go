// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/mattn/go-shellwords"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	clientconsts "github.com/siderolabs/omni/client/pkg/constants"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	_ "github.com/siderolabs/omni/cmd/acompat" // this package should always be imported first for init->set env to work
	"github.com/siderolabs/omni/cmd/omni/pkg/app"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/clientconfig"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

// Flag values.
var (
	omniEndpoint             string
	restartAMachineScript    string
	wipeAMachineScript       string
	freezeAMachineScript     string
	omnictlPath              string
	talosVersion             string
	anotherTalosVersion      string
	kubernetesVersion        string
	anotherKubernetesVersion string
	expectedMachines         int

	// provisioning flags
	provisionMachinesCount int
	infraProvider          string
	providerData           string
	provisionConfigFile    string

	scalingTimeout time.Duration

	cleanupLinks                bool
	runStatsCheck               bool
	skipExtensionsCheckOnCreate bool
	artifactsOutputDir          string

	runEmbeddedOmni bool
	omniConfigPath  string
	omniLogOutput   string
)

func TestIntegration(t *testing.T) {
	machineOptions := MachineOptions{
		TalosVersion:      talosVersion,
		KubernetesVersion: kubernetesVersion,
	}

	options := Options{
		ExpectedMachines:            expectedMachines,
		CleanupLinks:                cleanupLinks,
		RunStatsCheck:               runStatsCheck,
		SkipExtensionsCheckOnCreate: skipExtensionsCheckOnCreate,

		MachineOptions:           machineOptions,
		AnotherTalosVersion:      anotherTalosVersion,
		AnotherKubernetesVersion: anotherKubernetesVersion,
		OmnictlPath:              omnictlPath,
		ScalingTimeout:           scalingTimeout,
		OutputDir:                artifactsOutputDir,
	}

	var serviceAccount string

	if runEmbeddedOmni {
		var err error

		serviceAccount, err = runOmni(t)

		require.NoError(t, err)
	}

	if provisionConfigFile != "" {
		f, err := os.Open(provisionConfigFile)

		require.NoError(t, err, "failed to open provision config file")

		decoder := yaml.NewDecoder(f)

		for {
			var cfg MachineProvisionConfig

			if err = decoder.Decode(&cfg); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				require.NoError(t, err, "failed to parse provision config file")
			}

			options.ProvisionConfigs = append(options.ProvisionConfigs, cfg)
		}
	} else {
		options.ProvisionConfigs = append(options.ProvisionConfigs,
			MachineProvisionConfig{
				MachineCount: provisionMachinesCount,
				Provider: MachineProviderConfig{
					ID:   infraProvider,
					Data: providerData,
				},
			},
		)
	}

	if restartAMachineScript != "" {
		parsedScript, err := shellwords.Parse(restartAMachineScript)
		require.NoError(t, err, "failed to parse restart-a-machine-script file")

		options.RestartAMachineFunc = func(ctx context.Context, uuid string) error {
			return execCmd(ctx, parsedScript, uuid)
		}
	}

	if wipeAMachineScript != "" {
		parsedScript, err := shellwords.Parse(wipeAMachineScript)
		require.NoError(t, err, "failed to parse wipe-a-machine-script file")

		options.WipeAMachineFunc = func(ctx context.Context, uuid string) error {
			return execCmd(ctx, parsedScript, uuid)
		}
	}

	if freezeAMachineScript != "" {
		parsedScript, err := shellwords.Parse(freezeAMachineScript)
		require.NoError(t, err, "failed to parse freeze-a-machine-script file")

		options.FreezeAMachineFunc = func(ctx context.Context, uuid string) error {
			return execCmd(ctx, parsedScript, uuid)
		}
	}

	u, err := url.Parse(omniEndpoint)
	require.NoError(t, err, "error parsing omni endpoint")

	if u.Scheme == "grpc" {
		u.Scheme = "http"
	}

	options.HTTPEndpoint = u.String()

	if serviceAccount == "" {
		serviceAccount = os.Getenv(serviceaccount.OmniServiceAccountKeyEnvVar)
		if serviceAccount == "" {
			t.Fatalf("%s environment variable is not set", serviceaccount.OmniServiceAccountKeyEnvVar)

			return
		}
	}

	// Talos API calls try to use user auth if the service account var is not set
	os.Setenv(serviceaccount.OmniServiceAccountKeyEnvVar, serviceAccount)

	clientConfig := clientconfig.New(omniEndpoint, serviceAccount)

	t.Cleanup(func() {
		clientConfig.Close() //nolint:errcheck
	})

	rootClient, err := clientConfig.GetClient(t.Context())
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rootClient.Close())
	})

	testOptions := &TestOptions{
		omniClient:       rootClient,
		Options:          options,
		machineSemaphore: semaphore.NewWeighted(int64(options.ExpectedMachines)),
		clientConfig:     clientConfig,
	}

	preRunHooks(t, testOptions)

	t.Run("Suites", func(t *testing.T) {
		t.Run("CleanState", testCleanState(testOptions))
		t.Run("TalosImageGeneration", testImageGeneration(testOptions))
		t.Run("CLICommands", testCLICommands(testOptions))
		t.Run("KubernetesNodeAudit", testKubernetesNodeAudit(testOptions))
		t.Run("ForcedMachineRemoval", testForcedMachineRemoval(testOptions))
		t.Run("ImmediateClusterDestruction", testImmediateClusterDestruction(testOptions))
		t.Run("DefaultCluster", testDefaultCluster(testOptions))
		t.Run("EncryptedCluster", testEncryptedCluster(testOptions))
		t.Run("SinglenodeCluster", testSinglenodeCluster(testOptions))
		t.Run("ScaleUpAndDown", testScaleUpAndDown(testOptions))
		t.Run("ScaleUpAndDownMachineClassBasedMachineSets", testScaleUpAndDownMachineClassBasedMachineSets(testOptions))
		t.Run("ScaleUpAndDownAutoProvisionMachineSets", testScaleUpAndDownAutoProvisionMachineSets(testOptions))
		t.Run("RollingUpdateParallelism", testRollingUpdateParallelism(testOptions))
		t.Run("ReplaceControlPlanes", testReplaceControlPlanes(testOptions))
		t.Run("ConfigPatching", testConfigPatching(testOptions))
		t.Run("TalosUpgrades", testTalosUpgrades(testOptions))
		t.Run("KubernetesUpgrades", testKubernetesUpgrades(testOptions))
		t.Run("EtcdBackupAndRestore", testEtcdBackupAndRestore(testOptions))
		t.Run("MaintenanceUpgrade", testMaintenanceUpgrade(testOptions))
		t.Run("Auth", testAuth(testOptions))
		t.Run("ClusterTemplate", testClusterTemplate(testOptions))
		t.Run("WorkloadProxy", testWorkloadProxy(testOptions))
		t.Run("StaticInfraProvider", testStaticInfraProvider(testOptions))
	})

	postRunHooks(t, testOptions)
}

func init() {
	flag.StringVar(&omniEndpoint, "omni.endpoint", "grpc://127.0.0.1:8080", "The endpoint of the Omni API.")
	flag.IntVar(&expectedMachines, "omni.expected-machines", 4, "minimum number of machines expected")
	flag.StringVar(&restartAMachineScript, "omni.restart-a-machine-script", "hack/test/restart-a-vm.sh", "a script to run to restart a machine by UUID (optional)")
	flag.StringVar(&wipeAMachineScript, "omni.wipe-a-machine-script", "hack/test/wipe-a-vm.sh", "a script to run to wipe a machine by UUID (optional)")
	flag.StringVar(&freezeAMachineScript, "omni.freeze-a-machine-script", "hack/test/freeze-a-vm.sh", "a script to run to freeze a machine by UUID (optional)")
	flag.StringVar(&omnictlPath, "omni.omnictl-path", "_out/omnictl-linux-amd64", "omnictl CLI script path (optional)")
	flag.StringVar(&anotherTalosVersion, "omni.another-talos-version",
		constants.AnotherTalosVersion,
		"omni.Talos version for upgrade test",
	)
	flag.StringVar(
		&talosVersion,
		"omni.talos-version",
		clientconsts.DefaultTalosVersion,
		"omni.installer version for workload clusters",
	)
	flag.StringVar(&kubernetesVersion, "omni.kubernetes-version", constants.DefaultKubernetesVersion, "Kubernetes version for workload clusters")
	flag.StringVar(&anotherKubernetesVersion, "omni.another-kubernetes-version", constants.AnotherKubernetesVersion, "Kubernetes version for upgrade tests")
	flag.BoolVar(&cleanupLinks, "omni.cleanup-links", false, "remove all links after the tests are complete")
	flag.BoolVar(&runStatsCheck, "omni.run-stats-check", false, "runs stats check after the test is complete")
	flag.IntVar(&provisionMachinesCount, "omni.provision-machines", 0, "provisions machines through the infrastructure provider")
	flag.StringVar(&infraProvider, "omni.infra-provider", "talemu", "use infra provider with the specified ID when provisioning the machines")
	flag.StringVar(&providerData, "omni.provider-data", "{}", "the infra provider machine template data to use")
	flag.DurationVar(&scalingTimeout, "omni.scale-timeout", time.Second*150, "scale up test timeout")
	flag.StringVar(&provisionConfigFile, "omni.provision-config-file", "", "provision machines with the more complicated configuration")
	flag.BoolVar(&skipExtensionsCheckOnCreate, "omni.skip-extensions-check-on-create", false,
		"omni.disables checking for hello-world-service extension on the machine allocation and in the upgrade tests")
	flag.StringVar(&artifactsOutputDir, "omni.output-dir", "/tmp/integration-test", "output directory for the files generated by the test, e.g., the support bundles")
	flag.BoolVar(&runEmbeddedOmni, "omni.embedded", false, "runs embedded Omni in the tests")
	flag.StringVar(&omniConfigPath, "omni.config-path", "", "embedded Omni config path")
	flag.StringVar(&omniLogOutput, "omni.log-output", "_out/omni-test.log", "output logs directory")
}

func execCmd(ctx context.Context, parsedScript []string, args ...string) error {
	cmd := exec.CommandContext(ctx, parsedScript[0], append(parsedScript[1:], args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (opts *TestOptions) claimMachines(t *testing.T, count int) {
	require.GreaterOrEqual(t, expectedMachines, count)

	t.Logf("attempting to acquire semaphore for %d machines", count)

	if err := opts.machineSemaphore.Acquire(t.Context(), int64(count)); err != nil {
		t.Fatalf("failed to acquire machine semaphore: %s", err)
	}

	t.Logf("acquired semaphore for %d machines", count)

	t.Cleanup(func() {
		t.Logf("releasing semaphore for %d machines", count)

		opts.machineSemaphore.Release(int64(count))
	})
}

func runTests(t *testing.T, tests []subTest) {
	for _, tt := range tests {
		t.Run(tt.Name, tt.F)
	}
}

func preRunHooks(t *testing.T, options *TestOptions) {
	if !options.provisionMachines() {
		return
	}

	for i, cfg := range options.ProvisionConfigs {
		if cfg.Provider.Static {
			infraMachinesAcceptHook(t, options.omniClient.Omni().State(), cfg.Provider.ID, cfg.MachineCount, true)

			continue
		}

		t.Logf("provision %d machines using provider %q, machine request set name provisioned%d",
			cfg.MachineCount,
			cfg.Provider.ID,
			i,
		)

		machineProvisionHook(
			t,
			options.omniClient,
			cfg,
			fmt.Sprintf("provisioned%d", i),
			options.MachineOptions.TalosVersion,
		)
	}
}

func postRunHooks(t *testing.T, options *TestOptions) {
	if options.provisionMachines() {
		for i, cfg := range options.ProvisionConfigs {
			if cfg.Provider.Static {
				infraMachinesDestroyHook(t, options.omniClient.Omni().State(), cfg.Provider.ID, cfg.MachineCount)

				continue
			}

			machineDeprovisionHook(t, options.omniClient, fmt.Sprintf("provisioned%d", i))
		}
	}

	if options.RunStatsCheck {
		t.Log("checking controller stats for the write and read spikes")

		statsLimitsHook(t)
	}

	if options.CleanupLinks {
		require.NoError(t, cleanupLinksFunc(t.Context(), options.omniClient.Omni().State()))
	}
}

func cleanupLinksFunc(ctx context.Context, st state.State) error {
	links, err := safe.ReaderListAll[*siderolink.Link](ctx, st)
	if err != nil {
		return err
	}

	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()

	return links.ForEachErr(func(r *siderolink.Link) error {
		err := st.TeardownAndDestroy(ctx, r.Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		return nil
	})
}

func runOmni(t *testing.T) (string, error) {
	if omniConfigPath == "" {
		return "", errors.New("omni.config-path must be set when running embedded Omni")
	}

	params, err := config.LoadFromFile(omniConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to load omni config %w", err)
	}

	var (
		eg      errgroup.Group
		logFile *os.File
	)

	t.Cleanup(func() {
		require.NoError(t, eg.Wait())

		if logFile != nil {
			require.NoError(t, logFile.Close())
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var logger *zap.Logger

	switch {
	case omniLogOutput == "inline":
		logger = zaptest.NewLogger(t)
	case omniLogOutput == "":
		logger = zap.NewNop()

		t.Log("discard Omni log")
	default:
		t.Logf("write Omni logs to the file %s", omniLogOutput)

		logFile, err = os.OpenFile(omniLogOutput, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0o664)
		require.NoError(t, err)

		encoder := zap.NewDevelopmentEncoderConfig()

		fileEncoder := zapcore.NewConsoleEncoder(encoder)

		core := zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), zap.DebugLevel)

		logger = zap.New(core)
	}

	config, err := app.PrepareConfig(logger, params)
	if err != nil {
		return "", err
	}

	omniCtx := actor.MarkContextAsInternalActor(t.Context())

	state, err := omni.NewState(omniCtx, config, logger, prometheus.DefaultRegisterer)
	if err != nil {
		return "", err
	}

	t.Cleanup(func() {
		require.NoError(t, state.Close())
	})

	eg.Go(func() error {
		defer cancel()

		return app.Run(omniCtx, state, config, logger)
	})

	t.Log("waiting for Omni to start")

	rtestutils.AssertResources(ctx, t, state.Default(), []string{talosVersion}, func(*omnires.TalosVersion, *assert.Assertions) {})

	sa, err := clientconfig.CreateServiceAccount(omniCtx, "root", state.Default())
	if err != nil {
		return "", err
	}

	omniEndpoint = params.Services.API.URL()
	t.Logf("running integration tests using embedded Omni at %q", omniEndpoint)

	return sa, nil
}
