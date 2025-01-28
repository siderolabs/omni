// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package check

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Connection checks that at least one node has s connected available.
func Connection(ctx context.Context, r controller.Reader, clusterName string) error {
	clusterMachineStatuses, err := safe.ReaderListAll[*omni.ClusterMachineStatus](
		ctx,
		r,
		state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
			resource.LabelExists(omni.LabelControlPlaneRole),
		),
	)
	if err != nil {
		return newError(specs.ControlPlaneStatusSpec_Condition_Error, true, "Failed to get the list of machines %s", err.Error())
	}

	for val := range clusterMachineStatuses.All() {
		_, connected := val.Metadata().Labels().Get(omni.MachineStatusLabelConnected)
		if connected {
			return nil
		}
	}

	return newError(specs.ControlPlaneStatusSpec_Condition_Warning, true, "No control plane nodes are connected")
}
