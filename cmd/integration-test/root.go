// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"time"

	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"

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
	RunE: func(*cobra.Command, []string) error {
		return withContext(func(ctx context.Context) error {
			// hacky hack
			os.Args = append(os.Args[0:1], "-test.v", "-test.parallel", strconv.FormatInt(rootCmdFlags.parallel, 10), "-test.timeout", rootCmdFlags.testsTimeout.String())

			testOptions := tests.Options{
				RunTestPattern: rootCmdFlags.runTestPattern,

				ExpectedMachines: rootCmdFlags.expectedMachines,
				CleanupLinks:     rootCmdFlags.cleanupLinks,
				RunStatsCheck:    rootCmdFlags.runStatsCheck,

				MachineOptions:           rootCmdFlags.machineOptions,
				AnotherTalosVersion:      rootCmdFlags.anotherTalosVersion,
				AnotherKubernetesVersion: rootCmdFlags.anotherKubernetesVersion,
				OmnictlPath:              rootCmdFlags.omnictlPath,
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

	expectedMachines int
	parallel         int64
	cleanupLinks     bool
	runStatsCheck    bool

	testsTimeout time.Duration

	restartAMachineScript    string
	wipeAMachineScript       string
	freezeAMachineScript     string
	anotherTalosVersion      string
	anotherKubernetesVersion string
	omnictlPath              string

	machineOptions tests.MachineOptions
}

func init() {
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
}

// withContext wraps with CLI context.
func withContext(f func(ctx context.Context) error) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return f(ctx)
}
