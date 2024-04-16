// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/config"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func ForAllCompatibleVersions(
	talosVersions []string,
	k8sVersions []*compatibility.KubernetesVersion,
	fn func(talosVersion string, k8sVersions []string) error,
) error {
	return forAllCompatibleVersions(talosVersions, k8sVersions, fn)
}

func GetMachineSetNodeSortFunction(machineStatuses map[resource.ID]*omni.MachineStatus) func(a, b *omni.MachineSetNode) int {
	return getSortFunction(machineStatuses)
}

func StripTalosAPIAccessOSAdminRole(cfg config.Provider) (config.Provider, error) {
	return stripTalosAPIAccessOSAdminRole(cfg)
}

func GetDesiredSchematic(ctx context.Context, r controller.Reader, machine *omni.ClusterMachine) (string, error) {
	return getDesiredSchematic(ctx, r, machine)
}
