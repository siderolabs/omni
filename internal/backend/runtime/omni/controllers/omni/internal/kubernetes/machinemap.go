// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineMap keeps a relation between Kubernetes node names and Machine IDs.
//
// It helps to connect resources based on Kubernetes data with Omni internals based on machine IDs.
type MachineMap struct {
	// both maps are nodename -> machine ID
	ControlPlanes map[string]string
	Workers       map[string]string
}

// NewMachineMap creates a new MachineMap based on ClusterMachineIdentity resources.
func NewMachineMap(ctx context.Context, r controller.Reader, cluster *omni.Cluster) (*MachineMap, error) {
	nodenameToMachineMap := MachineMap{
		ControlPlanes: map[string]string{},
		Workers:       map[string]string{},
	}

	clusterMachineIdentities, err := safe.ReaderListAll[*omni.ClusterMachineIdentity](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return nil, err
	}

	for cmi := range clusterMachineIdentities.All() {
		if _, controlplane := cmi.Metadata().Labels().Get(omni.LabelControlPlaneRole); controlplane {
			nodenameToMachineMap.ControlPlanes[cmi.TypedSpec().Value.Nodename] = cmi.Metadata().ID()
		} else {
			nodenameToMachineMap.Workers[cmi.TypedSpec().Value.Nodename] = cmi.Metadata().ID()
		}
	}

	return &nodenameToMachineMap, nil
}
