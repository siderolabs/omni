// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package layeredresource provides a helper for looking up resources that may be defined on different levels: machine, machine in a cluster, machine set, cluster.
package layeredresource

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Helper provides a way to lookup labeled resources that may be defined on different levels: machine, machine in a cluster, machine set, cluster.
type Helper[T generic.ResourceWithRD] struct {
	allResources safe.List[T]
}

// NewHelper creates a new helper for managing layered resources.
func NewHelper[T generic.ResourceWithRD](ctx context.Context, r controller.Reader) (*Helper[T], error) {
	allResources, err := safe.ReaderListAll[T](ctx, r)
	if err != nil {
		return nil, err
	}

	return &Helper[T]{
		allResources: allResources,
	}, nil
}

// Get collects all machine-level, machine set-level, cluster machine-level, and cluster-level resources.
func (h *Helper[T]) Get(machine resource.Resource, machineSet *omni.MachineSet) ([]T, error) {
	clusterName, ok := machine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, fmt.Errorf("cluster machine %q doesn't have cluster label set", machine.Metadata().ID())
	}

	resourceList := h.allResources.FilterLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName))

	machineLevel := h.allResources.FilterLabelQuery(resource.LabelEqual(omni.LabelMachine, machine.Metadata().ID()))

	clusterLevel := make([]T, 0, resourceList.Len())
	machineSetLevel := make([]T, 0, resourceList.Len())
	clusterMachineLevel := make([]T, 0, resourceList.Len())

	for res := range resourceList.All() {
		machineSetName, machineSetOk := res.Metadata().Labels().Get(omni.LabelMachineSet)
		clusterMachineName, clusterMachineOk := res.Metadata().Labels().Get(omni.LabelClusterMachine)
		_, machineOk := res.Metadata().Labels().Get(omni.LabelMachine)

		switch {
		// skip it, it's a machine level resource (and it also has cluster label)
		case machineOk:
		// machine set resource
		case machineSetOk && machineSetName == machineSet.Metadata().ID():
			machineSetLevel = append(machineSetLevel, res)
		// cluster machine resource
		case clusterMachineOk && clusterMachineName == machine.Metadata().ID():
			clusterMachineLevel = append(clusterMachineLevel, res)
		// cluster resource
		case !machineSetOk && !clusterMachineOk:
			clusterLevel = append(clusterLevel, res)
		}
	}

	resources := make([]T, 0, resourceList.Len()+machineLevel.Len())

	resources = append(resources, clusterLevel...)
	resources = append(resources, machineSetLevel...)
	resources = append(resources, clusterMachineLevel...)
	resources = slices.AppendSeq(resources, machineLevel.All())

	return xslices.Filter(resources, func(res T) bool {
		return res.Metadata().Phase() == resource.PhaseRunning
	}), nil
}
