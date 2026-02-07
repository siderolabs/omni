// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secret

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/reporter"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var rotateCmdFlags struct {
	wait        bool
	waitTimeout time.Duration
}

// rotateCmd represents the secret rotate command.
var rotateCmd = &cobra.Command{
	Use:     "rotate",
	Short:   "Rotate cluster secrets, wait for the rotation to complete.",
	Long:    `Rotates specified cluster secret, if the terminal supports it, watch the secret update steps. The command waits for the secret to be fully rotated by default.`,
	Example: "",
	Args:    cobra.NoArgs,
}

// talosCACmd represents the secret rotate talos-ca command.
var talosCACmd = &cobra.Command{
	Use:     "talos-ca",
	Short:   "Rotate Talos CA certificate.",
	Long:    `Rotate Talos CA certificate for all nodes in the cluster. Talos CA is used by Talos API.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client) error {
			return rotateCA(ctx, client, omni.NewRotateTalosCA(secretCmdFlags.clusterName), specs.SecretRotationSpec_TALOS_CA, "Talos CA")
		})
	},
}

// kubernetesCACmd represents the secret rotate kubernetes-ca command.
var kubernetesCACmd = &cobra.Command{
	Use:     "kubernetes-ca",
	Short:   "Rotate Kubernetes CA certificate.",
	Long:    `Rotate Kubernetes CA certificate for all nodes in the cluster. Kubernetes CA is used by the Kubernetes API server and kubelet for authenticating clients.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client) error {
			return rotateCA(ctx, client, omni.NewRotateKubernetesCA(secretCmdFlags.clusterName), specs.SecretRotationSpec_KUBERNETES_CA, "Kubernetes CA")
		})
	},
}

var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show secret rotation status, wait for the rotation to complete.",
	Long:    `Shows current secret rotation status, if the terminal supports it, watch the status as it updates. The command waits for the secret to be fully rotated by default.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(status)
	},
}

func rotateCA[T resource.Resource](ctx context.Context, client *client.Client, r T, component specs.SecretRotationSpec_Component, componentName string) error {
	ctx, cancel := context.WithTimeout(ctx, rotateCmdFlags.waitTimeout)
	defer cancel()

	st := client.Omni().State()

	cluster, rotationStatus, err := getClusterAndRotationStatus(ctx, st)
	if err != nil {
		return err
	}

	talosVersion, err := semver.ParseTolerant(cluster.TypedSpec().Value.TalosVersion)
	if err != nil {
		return err
	}

	if talosVersion.LT(semver.MustParse("1.7.0")) {
		return fmt.Errorf("rotating %s is only supported for clusters running Talos v1.7.0 or newer, current version: %q", componentName, talosVersion.String())
	}

	if rotationStatus.TypedSpec().Value.Phase != specs.SecretRotationSpec_OK {
		if rotationStatus.TypedSpec().Value.Component != component {
			return fmt.Errorf("another rotation is already in progress for %s", rotationStatus.TypedSpec().Value.Component.String())
		}

		return printSecretRotationStatus(ctx, st, cluster.Metadata().ID(), rotationStatus)
	}

	fmt.Printf("starting to rotate %s for the cluster %q\n", componentName, cluster.Metadata().ID())

	if err = safe.StateModify[T](ctx, st, r,
		func(res T) error {
			res.Metadata().Annotations().Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))

			return nil
		},
	); err != nil {
		return err
	}

	if rotateCmdFlags.wait {
		rotationStatus, err = safe.StateWatchFor[*omni.ClusterSecretsRotationStatus](ctx, st, rotationStatus.Metadata(), func(cond *state.WatchForCondition) error {
			cond.Condition = func(res resource.Resource) (bool, error) {
				resTyped, ok := res.(*omni.ClusterSecretsRotationStatus)
				if !ok {
					return false, fmt.Errorf("unexpected resource type: %T", res)
				}

				if resTyped.TypedSpec().Value.Phase != specs.SecretRotationSpec_OK {
					return true, nil
				}

				return false, nil
			}

			return nil
		})
		if err != nil {
			return err
		}

		return printSecretRotationStatus(ctx, st, cluster.Metadata().ID(), rotationStatus)
	}

	return nil
}

func printSecretRotationStatus(ctx context.Context, st state.State, clusterName string, rotationStatus *omni.ClusterSecretsRotationStatus) error {
	if rotationStatus.TypedSpec().Value.Phase == specs.SecretRotationSpec_OK {
		fmt.Printf("no ongoing secret rotation for the cluster %q\n", clusterName)

		return nil
	}

	if !rotateCmdFlags.wait {
		fmt.Printf("rotation for %q is in progress(%q) for the cluster %q\n", rotationStatus.TypedSpec().Value.Component.String(), rotationStatus.TypedSpec().Value.Phase.String(), clusterName)

		return nil
	}

	report := reporter.New()
	report.Report(reporter.Update{Message: fmt.Sprintf("rotation for %s is in progress ...\n", rotationStatus.TypedSpec().Value.Component.String()), Status: reporter.StatusRunning})

	watchCh := make(chan safe.WrappedStateEvent[*omni.ClusterSecretsRotationStatus])

	if err := safe.StateWatch(ctx, st, rotationStatus.Metadata(), watchCh); err != nil {
		return fmt.Errorf("failed to establish a watch on %s %s: %w", omni.ClusterSecretsRotationStatusType, clusterName, err)
	}

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				report.Report(reporter.Update{Message: fmt.Sprintf("timed out waiting rotation for %s to complete\n",
					rotationStatus.TypedSpec().Value.Component.String()), Status: reporter.StatusError})
			}

			return ctx.Err()
		case event := <-watchCh:
			if err := event.Error(); err != nil {
				return fmt.Errorf("error watching for %s: %w", omni.ClusterSecretsRotationStatusType, err)
			}

			var res *omni.ClusterSecretsRotationStatus

			res, err := event.Resource()
			if err != nil {
				return err
			}

			if res.TypedSpec().Value.Phase == specs.SecretRotationSpec_OK {
				report.Report(reporter.Update{Message: fmt.Sprintf("rotation for %s is completed\n", rotationStatus.TypedSpec().Value.Component.String()), Status: reporter.StatusSucceeded})

				return nil
			}

			report.Report(reporter.Update{Message: fmt.Sprintf("%s: %s\n", res.TypedSpec().Value.Status, res.TypedSpec().Value.Step), Status: reporter.StatusRunning})
		}
	}
}

func status(ctx context.Context, client *client.Client) error {
	ctx, cancel := context.WithTimeout(ctx, rotateCmdFlags.waitTimeout)
	defer cancel()

	st := client.Omni().State()

	cluster, rotationStatus, err := getClusterAndRotationStatus(ctx, st)
	if err != nil {
		return err
	}

	return printSecretRotationStatus(ctx, st, cluster.Metadata().ID(), rotationStatus)
}

func getClusterAndRotationStatus(ctx context.Context, st state.State) (*omni.Cluster, *omni.ClusterSecretsRotationStatus, error) {
	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, secretCmdFlags.clusterName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	rotationStatus, err := safe.StateGetByID[*omni.ClusterSecretsRotationStatus](ctx, st, cluster.Metadata().ID())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster secrets rotation status: %w", err)
	}

	return cluster, rotationStatus, nil
}

func init() {
	rotateCmd.PersistentFlags().BoolVarP(&rotateCmdFlags.wait, "wait", "w", true, "wait for the secret rotation to complete")
	rotateCmd.PersistentFlags().DurationVarP(&rotateCmdFlags.waitTimeout, "wait-timeout", "t", 5*time.Minute, "wait timeout for secret rotation, if zero, wait indefinitely")
	rotateCmd.AddCommand(talosCACmd)
	rotateCmd.AddCommand(kubernetesCACmd)
	rotateCmd.AddCommand(statusCmd)
	secretCmd.AddCommand(rotateCmd)
}
