// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package options

import "github.com/siderolabs/omni/client/pkg/omni/resources/omni"

func LabelCluster(cluster *omni.Cluster) MockOption {
	return func(mo *MockOptions) {
		mo.AddLabel(omni.LabelCluster, cluster.Metadata().ID())
	}
}

func LabelMachineSet(machineSet *omni.MachineSet) MockOption {
	return func(mo *MockOptions) {
		mo.AddLabel(omni.LabelMachineSet, machineSet.Metadata().ID())
	}
}

func EmptyLabel(role string) MockOption {
	return func(mo *MockOptions) {
		mo.AddLabel(role, "")
	}
}
