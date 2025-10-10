// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clusterimport

import (
	"context"
	"fmt"
	"io"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// Abort stops the import operation for a cluster by validating its locked and tainted statuses, tearing down resources, and cleaning up machine-set node links.
func Abort(ctx context.Context, omniState state.State, clusterID string, logWriter io.Writer) error {
	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, omniState, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster %q: %w", clusterID, err)
	}

	clusterStatus, err := safe.ReaderGetByID[*omni.ClusterStatus](ctx, omniState, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster status for %q: %w", clusterID, err)
	}

	_, importing := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
	_, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked)

	if !importing {
		return fmt.Errorf("cluster %q is not tainted as importing", clusterID)
	}

	if !locked {
		return fmt.Errorf("cluster %q is not locked", clusterID)
	}

	machineSetNodes, err := safe.StateListAll[*omni.MachineSetNode](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
	if err != nil {
		return fmt.Errorf("failed to list machinesetnodes for cluster %q: %w", clusterID, err)
	}

	_, err = omniState.Teardown(ctx, cluster.Metadata())
	if err != nil {
		return err
	}

	_, err = omniState.WatchFor(ctx, cluster.Metadata(), state.WithEventTypes(state.Destroyed))
	if err != nil {
		return err
	}

	for msn := range machineSetNodes.All() {
		fmt.Fprintf(logWriter, "removing link for machine %q from Omni\n", msn.Metadata().ID()) //nolint:errcheck

		_, linkErr := omniState.Teardown(ctx, siderolink.NewLink(resources.DefaultNamespace, msn.Metadata().ID(), nil).Metadata())
		if linkErr != nil {
			return fmt.Errorf("failed to destroy siderolink %q: %w", msn.Metadata().ID(), linkErr)
		}
	}

	return nil
}
