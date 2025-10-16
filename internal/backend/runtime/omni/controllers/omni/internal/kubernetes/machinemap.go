// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"context"
	"fmt"

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
	// maps are nodename -> machine ID
	ControlPlanes map[string]string
	Workers       map[string]string
	Locked        map[string]string
}

// NewMachineMap creates a new MachineMap based on ClusterMachineStatus resources.
func NewMachineMap(ctx context.Context, r controller.Reader, cluster *omni.Cluster) (*MachineMap, error) {
	nodenameToMachineMap := MachineMap{
		ControlPlanes: map[string]string{},
		Workers:       map[string]string{},
		Locked:        map[string]string{},
	}

	clusterMachineStatuses, err := safe.ReaderListAll[*omni.ClusterMachineStatus](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return nil, err
	}

	for cms := range clusterMachineStatuses.All() {
		nodename, ok := cms.Metadata().Labels().Get(omni.ClusterMachineStatusLabelNodeName)
		if !ok {
			return nil, fmt.Errorf("failed to fetch node name from %s ClusterMachineStatus", cms.Metadata().ID())
		}

		if _, locked := cms.Metadata().Annotations().Get(omni.MachineLocked); locked {
			nodenameToMachineMap.Locked[nodename] = cms.Metadata().ID()
		}

		if _, controlplane := cms.Metadata().Labels().Get(omni.LabelControlPlaneRole); controlplane {
			nodenameToMachineMap.ControlPlanes[nodename] = cms.Metadata().ID()
		} else {
			nodenameToMachineMap.Workers[nodename] = cms.Metadata().ID()
		}
	}

	return &nodenameToMachineMap, nil
}
