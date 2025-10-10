// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clusterimport

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// AbortContext represents the context for aborting an ongoing cluster import operation.
type AbortContext struct {
	cluster       *omni.Cluster
	clusterStatus *omni.ClusterStatus
	omniState     state.State
	input         Input
}

// NewAbortContext creates a new AbortContext using the provided context, input, and omniState.
// It retrieves the cluster and its status based on the input ClusterID.
func NewAbortContext(ctx context.Context, input Input, omniState state.State) (*AbortContext, error) {
	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, omniState, input.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %q: %w", input.ClusterID, err)
	}

	clusterStatus, err := safe.ReaderGetByID[*omni.ClusterStatus](ctx, omniState, input.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster status for %q: %w", input.ClusterID, err)
	}

	_, importing := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
	_, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked)

	if !importing {
		return nil, fmt.Errorf("cluster %q is not tainted as importing", input.ClusterID)
	}

	if !locked {
		return nil, fmt.Errorf("cluster %q is not locked", input.ClusterID)
	}

	return &AbortContext{
		cluster:       cluster,
		clusterStatus: clusterStatus,
		omniState:     omniState,
		input:         input,
	}, nil
}

// Abort stops the import operation for a cluster by validating its locked and tainted statuses, tearing down resources, and cleaning up machine-set node links.
func (c *AbortContext) Abort(ctx context.Context) error {
	machineSetNodes, err := safe.StateListAll[*omni.MachineSetNode](ctx, c.omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, c.input.ClusterID)))
	if err != nil {
		return fmt.Errorf("failed to list machinesetnodes for cluster %q: %w", c.input.ClusterID, err)
	}

	_, err = c.omniState.Teardown(ctx, c.cluster.Metadata())
	if err != nil {
		return err
	}

	_, err = c.omniState.WatchFor(ctx, c.cluster.Metadata(), state.WithEventTypes(state.Destroyed))
	if err != nil {
		return err
	}

	for msn := range machineSetNodes.All() {
		c.input.logf("removing link for machine %q from Omni\n", msn.Metadata().ID())

		_, linkErr := c.omniState.Teardown(ctx, siderolink.NewLink(resources.DefaultNamespace, msn.Metadata().ID(), nil).Metadata())
		if linkErr != nil {
			return fmt.Errorf("failed to destroy siderolink %q: %w", msn.Metadata().ID(), linkErr)
		}
	}

	return nil
}
