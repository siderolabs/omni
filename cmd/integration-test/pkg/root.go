// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package pkg provides the root command for the omni-integration-test binary.
package pkg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/pkg/compression"
	clientconsts "github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/cmd/integration-test/pkg/clientconfig"
	"github.com/siderolabs/omni/cmd/integration-test/pkg/tests"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "omni-integration-test",
	Short: "Omni integration test runner.",
	Long:  ``,
	PersistentPreRunE: func(*cobra.Command, []string) error {
		return compression.InitConfig(true)
	},
	RunE: func(*cobra.Command, []string) error {
		return withContext(func(ctx context.Context) error {
			// hacky hack
			os.Args = append(os.Args[0:1], "-test.v", "-test.parallel", strconv.FormatInt(rootCmdFlags.parallel, 10), "-test.timeout", rootCmdFlags.testsTimeout.String())

			testOptions := tests.Options{
				RunTestPattern: rootCmdFlags.runTestPattern,

				ExpectedMachines:            rootCmdFlags.expectedMachines,
				CleanupLinks:                rootCmdFlags.cleanupLinks,
				RunStatsCheck:               rootCmdFlags.runStatsCheck,
				SkipExtensionsCheckOnCreate: rootCmdFlags.skipExtensionsCheckOnCreate,

				MachineOptions:           rootCmdFlags.machineOptions,
				AnotherTalosVersion:      rootCmdFlags.anotherTalosVersion,
				AnotherKubernetesVersion: rootCmdFlags.anotherKubernetesVersion,
				OmnictlPath:              rootCmdFlags.omnictlPath,
				ScalingTimeout:           rootCmdFlags.scalingTimeout,
				OutputDir:                rootCmdFlags.outputDir,
			}

			if rootCmdFlags.provisionConfigFile != "" {
				f, err := os.Open(rootCmdFlags.provisionConfigFile)
				if err != nil {
					return fmt.Errorf("failed to open provision config file %q: %w", rootCmdFlags.provisionConfigFile, err)
				}

				decoder := yaml.NewDecoder(f)

				for {
					var cfg tests.MachineProvisionConfig

					if err = decoder.Decode(&cfg); err != nil {
						if errors.Is(err, io.EOF) {
							break
						}

						return err
					}

					testOptions.ProvisionConfigs = append(testOptions.ProvisionConfigs, cfg)
				}
			} else {
				testOptions.ProvisionConfigs = append(testOptions.ProvisionConfigs,
					tests.MachineProvisionConfig{
						MachineCount: rootCmdFlags.provisionMachinesCount,
						Provider: tests.MachineProviderConfig{
							ID:   rootCmdFlags.infraProvider,
							Data: rootCmdFlags.providerData,
						},
					},
				)
			}

			if rootCmdFlags.restartAMachineScript != "" {
				parsedScript, err := shellwords.Parse(rootCmdFlags.restartAMachineScript)
				if err != nil {
					return fmt.Errorf("error parsing restart a machine script: %w", err)
				}

				testOptions.RestartAMachineFunc = func(ctx context.Context, uuid string) error {
					return execCmd(ctx, parsedScript, uuid)
				}
			}

			if rootCmdFlags.wipeAMachineScript != "" {
				parsedScript, err := shellwords.Parse(rootCmdFlags.wipeAMachineScript)
				if err != nil {
					return fmt.Errorf("error parsing wipe a machine script: %w", err)
				}

				testOptions.WipeAMachineFunc = func(ctx context.Context, uuid string) error {
					return execCmd(ctx, parsedScript, uuid)
				}
			}

			if rootCmdFlags.freezeAMachineScript != "" {
				parsedScript, err := shellwords.Parse(rootCmdFlags.freezeAMachineScript)
				if err != nil {
					return fmt.Errorf("error parsing freeze a machine script: %w", err)
				}

				testOptions.FreezeAMachineFunc = func(ctx context.Context, uuid string) error {
					return execCmd(ctx, parsedScript, uuid)
				}
			}

			u, err := url.Parse(rootCmdFlags.endpoint)
			if err != nil {
				return errors.New("error parsing endpoint")
			}

			if u.Scheme == "grpc" {
				u.Scheme = "http"
			}

			testOptions.HTTPEndpoint = u.String()

			clientConfig := clientconfig.New(rootCmdFlags.endpoint)
			defer clientConfig.Close() //nolint:errcheck

			return tests.Run(ctx, clientConfig, testOptions)
		})
	},
}

func execCmd(ctx context.Context, parsedScript []string, args ...string) error {
	cmd := exec.CommandContext(ctx, parsedScript[0], append(parsedScript[1:], args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

//nolint:govet
var rootCmdFlags struct {
	endpoint       string
	runTestPattern string
	infraProvider  string
	providerData   string

	provisionMachinesCount      int
	expectedMachines            int
	expectedBareMetalMachines   int
	parallel                    int64
	cleanupLinks                bool
	runStatsCheck               bool
	skipExtensionsCheckOnCreate bool

	testsTimeout   time.Duration
	scalingTimeout time.Duration

	restartAMachineScript    string
	wipeAMachineScript       string
	freezeAMachineScript     string
	anotherTalosVersion      string
	anotherKubernetesVersion string
	omnictlPath              string
	provisionConfigFile      string
	outputDir                string

	machineOptions tests.MachineOptions
}

// RootCmd returns the root command.
func RootCmd() *cobra.Command { return onceInit() }

var onceInit = sync.OnceValue(func() *cobra.Command {
	rootCmd.PersistentFlags().StringVar(&rootCmdFlags.endpoint, "endpoint", "grpc://127.0.0.1:8080", "The endpoint of the Omni API.")
	rootCmd.Flags().StringVar(&rootCmdFlags.runTestPattern, "test.run", "", "tests to run (regular expression)")
	rootCmd.Flags().IntVar(&rootCmdFlags.expectedMachines, "expected-machines", 4, "minimum number of machines expected")
	rootCmd.Flags().StringVar(&rootCmdFlags.restartAMachineScript, "restart-a-machine-script", "hack/test/restart-a-vm.sh", "a script to run to restart a machine by UUID (optional)")
	rootCmd.Flags().StringVar(&rootCmdFlags.wipeAMachineScript, "wipe-a-machine-script", "hack/test/wipe-a-vm.sh", "a script to run to wipe a machine by UUID (optional)")
	rootCmd.Flags().StringVar(&rootCmdFlags.freezeAMachineScript, "freeze-a-machine-script", "hack/test/freeze-a-vm.sh", "a script to run to freeze a machine by UUID (optional)")
	rootCmd.Flags().StringVar(&rootCmdFlags.omnictlPath, "omnictl-path", "", "omnictl CLI script path (optional)")
	rootCmd.Flags().StringVar(&rootCmdFlags.anotherTalosVersion, "another-talos-version",
		constants.AnotherTalosVersion,
		"Talos version for upgrade test",
	)
	rootCmd.Flags().StringVar(
		&rootCmdFlags.machineOptions.TalosVersion,
		"talos-version",
		clientconsts.DefaultTalosVersion,
		"installer version for workload clusters",
	)
	rootCmd.Flags().StringVar(&rootCmdFlags.machineOptions.KubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "Kubernetes version for workload clusters")
	rootCmd.Flags().StringVar(&rootCmdFlags.anotherKubernetesVersion, "another-kubernetes-version", constants.AnotherKubernetesVersion, "Kubernetes version for upgrade tests")
	rootCmd.Flags().Int64VarP(&rootCmdFlags.parallel, "parallel", "p", 4, "tests parallelism")
	rootCmd.Flags().DurationVarP(&rootCmdFlags.testsTimeout, "timeout", "t", time.Hour, "tests global timeout")
	rootCmd.Flags().BoolVar(&rootCmdFlags.cleanupLinks, "cleanup-links", false, "remove all links after the tests are complete")
	rootCmd.Flags().BoolVar(&rootCmdFlags.runStatsCheck, "run-stats-check", false, "runs stats check after the test is complete")
	rootCmd.Flags().IntVar(&rootCmdFlags.provisionMachinesCount, "provision-machines", 0, "provisions machines through the infrastructure provider")
	rootCmd.Flags().StringVar(&rootCmdFlags.infraProvider, "infra-provider", "talemu", "use infra provider with the specified ID when provisioning the machines")
	rootCmd.Flags().StringVar(&rootCmdFlags.providerData, "provider-data", "{}", "the infra provider machine template data to use")
	rootCmd.Flags().DurationVar(&rootCmdFlags.scalingTimeout, "scale-timeout", time.Second*150, "scale up test timeout")
	rootCmd.Flags().StringVar(&rootCmdFlags.provisionConfigFile, "provision-config-file", "", "provision machines with the more complicated configuration")
	rootCmd.Flags().BoolVar(&rootCmdFlags.skipExtensionsCheckOnCreate, "skip-extensions-check-on-create", false,
		"disables checking for hello-world-service extension on the machine allocation and in the upgrade tests")
	rootCmd.Flags().StringVar(&rootCmdFlags.outputDir, "output-dir", "/tmp/integration-test", "output directory for the files generated by the test, e.g., the support bundles")

	rootCmd.MarkFlagsMutuallyExclusive("provision-machines", "provision-config-file")

	return rootCmd
})

// withContext wraps with CLI context.
func withContext(f func(ctx context.Context) error) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return f(ctx)
}
