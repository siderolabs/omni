// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var manifestSyncCmdFlags struct {
	dryRun bool
}

// manifestSyncCmd represents the cluster kubernetes manifest-sync command.
var manifestSyncCmd = &cobra.Command{
	Use:   "manifest-sync cluster-name",
	Short: "Sync Kubernetes bootstrap manifests from Talos controlplane nodes to Kubernetes API.",
	Long: `Sync Kubernetes bootstrap manifests from Talos controlplane nodes to Kubernetes API.
Bootstrap manifests might be updated with Talos version update, Kubernetes upgrade, and config patching.
Talos never updates or deletes Kubernetes manifests, so this command fills the gap to keep manifests up-to-date.`,
	Example: "",
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		fmt.Printf("running manifest-sync with dryRun=%t\n", manifestSyncCmdFlags.dryRun)

		return access.WithClient(func(ctx context.Context, client *client.Client) error { return syncManifests(ctx, client, args[0]) })
	},
}

func syncManifests(ctx context.Context, client *client.Client, clusterName string) error {
	st := client.Omni().State()

	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterName)
	if err != nil {
		return fmt.Errorf("failed to fetch cluster data: %w", err)
	}

	talosVersion := cluster.TypedSpec().Value.TalosVersion

	talosVersionCompatibility, err := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: talosVersion})
	if err != nil {
		return err
	}

	useSSA := talosVersionCompatibility.SupportsSSAManifestSync()

	if useSSA {
		return syncSSA(ctx, client, clusterName)
	}

	// todo: remove once support for Talos versions older than v1.13 is dropped.
	return syncLegacy(ctx, client, clusterName)
}

func syncLegacy(ctx context.Context, client *client.Client, clusterName string) error {
	handler := func(resp *management.KubernetesSyncManifestResponse) error {
		switch resp.ResponseType {
		case management.KubernetesSyncManifestResponse_UNKNOWN:
		case management.KubernetesSyncManifestResponse_MANIFEST:
			fmt.Printf(" > processing manifest %s\n", resp.Path)

			switch {
			case resp.Skipped:
				fmt.Println(" < no changes")
			case manifestSyncCmdFlags.dryRun:
				fmt.Println(resp.Diff)
				fmt.Println(" < dry run, change skipped")
			case !manifestSyncCmdFlags.dryRun:
				fmt.Println(resp.Diff)
				fmt.Println(" < applied successfully")
			}
		case management.KubernetesSyncManifestResponse_ROLLOUT:
			fmt.Printf(" > waiting for %s\n", resp.Path)
		}

		return nil
	}

	return client.Management().WithCluster(clusterName).KubernetesSyncManifests(ctx, manifestSyncCmdFlags.dryRun, handler)
}

func syncSSA(ctx context.Context, client *client.Client, clusterName string) error {
	logFunc := func(line string, args ...any) {
		fmt.Printf(line+"\n", args...)
	}

	fmt.Println("comparing with live objects")

	defaultSSAOps := manifests.DefaultSSApplyBehaviorOptions()
	ssaOps := &management.KubernetesSSAOptions{
		InventoryPolicy:  management.KubernetesSSAOptions_ADOPT_IF_NO_INVENTORY,
		ReconcileTimeout: durationpb.New(defaultSSAOps.ReconcileTimeout),
		PruneTimeout:     durationpb.New(defaultSSAOps.PruneTimeout),
		ForceConflicts:   true,
		DryRun:           manifestSyncCmdFlags.dryRun,
		NoPrune:          false,
	}

	diffResult, err := client.Management().WithCluster(clusterName).KubernetesDiffManifests(ctx, ssaOps)
	if err != nil {
		return fmt.Errorf("failed to compare bootstrap manifests with live objects: %w", err)
	}

	convertedManifests := []manifests.DiffResult{}

	for _, d := range diffResult.Diffs {
		converted, err := helpers.DeconvertProtoDiffResult(d)
		if err != nil {
			return err
		}

		convertedManifests = append(convertedManifests, converted)
	}

	manifests.LogSSADiff(convertedManifests, logFunc)

	syncCh := make(chan *management.KubernetesBootstrapManifestSyncResponse)
	errCh := make(chan error, 1)

	fmt.Println("applying manifests")

	go func() {
		errCh <- client.Management().WithCluster(clusterName).KubernetesSyncManifestsSSA(
			ctx,
			ssaOps,
			syncCh,
		)
	}()

	eventLogger := manifests.NewSyncEventLogger(logFunc)

syncLoop:
	for {
		select {
		case e := <-syncCh:
			convertedEvent, err := helpers.DeconvertProtoSyncEvent(e)
			if err != nil {
				return err
			}

			eventLogger.LogSyncEvent(convertedEvent)

		case err := <-errCh:
			if err == nil {
				break syncLoop
			}

			return err
		}
	}

	return nil
}

func init() {
	manifestSyncCmd.Flags().BoolVar(&manifestSyncCmdFlags.dryRun, "dry-run", true, "don't actually sync manifests, just print what would be done")
	kubernetesCmd.AddCommand(manifestSyncCmd)
}
