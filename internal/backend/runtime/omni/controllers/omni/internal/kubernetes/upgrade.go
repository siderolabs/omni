// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// UpgradePath defines a path to upgrade Kubernetes components.
type UpgradePath struct {
	// AllNodesToRequiredImages is a map of all nodes used by the upgrade path with the images they require.
	AllNodesToRequiredImages map[string][]string

	// ClusterID is the ID of the cluster.
	ClusterID resource.ID

	// NotReadyStatus is a string containing the status of the unready components (if any).
	NotReadyStatus string

	// Steps is a list of steps to apply to correct the versions of Kubernetes components.
	Steps []UpgradeStep

	// AllNodes is a list of all nodes used by the upgrade path, in the order they appear.
	AllNodes []string

	// AllComponentsReady indicates if all Kubernetes components are ready.
	AllComponentsReady bool
}

// CalculateUpgradePath calculates the upgrade path for the cluster.
//
//nolint:gocyclo,cyclop,gocognit
func CalculateUpgradePath(nodenameToMachineMap *MachineMap, kubernetesStatus *omni.KubernetesStatus, desiredVersion string) *UpgradePath {
	upgradePath := &UpgradePath{
		ClusterID: kubernetesStatus.Metadata().ID(),
	}

	nodeToRequiredImageSet := make(map[string]map[string]struct{})

	addImagesToNode := func(nodeName string, images []string) {
		if _, ok := nodeToRequiredImageSet[nodeName]; !ok {
			nodeToRequiredImageSet[nodeName] = make(map[string]struct{})

			upgradePath.AllNodes = append(upgradePath.AllNodes, nodeName)
		}

		for _, image := range images {
			nodeToRequiredImageSet[nodeName][image] = struct{}{}
		}
	}

	appPatch := func(nodename string, component Component, pending bool) {
		machineID, ok := nodenameToMachineMap.ControlPlanes[nodename]

		if ok && component.Valid() {
			patch := component.Patch(desiredVersion)

			addImagesToNode(nodename, patch.UsedImages)

			if pending {
				upgradePath.Steps = append(upgradePath.Steps, UpgradeStep{
					MachineID:   machineID,
					Patch:       patch,
					Description: fmt.Sprintf("%s: updating %s to %s", nodename, component, desiredVersion),
					Node:        nodename,
					Component:   component,
				})
			}
		}
	}

	kubeletPatch := func(nodename string, pending bool) {
		machineID, ok := nodenameToMachineMap.ControlPlanes[nodename]
		if !ok {
			machineID, ok = nodenameToMachineMap.Workers[nodename]
		}

		if !ok {
			return
		}

		patch := Kubelet.Patch(desiredVersion)

		addImagesToNode(nodename, patch.UsedImages)

		if pending {
			upgradePath.Steps = append(upgradePath.Steps, UpgradeStep{
				MachineID:   machineID,
				Patch:       patch,
				Description: fmt.Sprintf("%s: updating %s to %s", nodename, Kubelet, desiredVersion),
				Node:        nodename,
				Component:   Kubelet,
			})
		}
	}

	unreadyApps := map[pair.Pair[string, Component]]struct{}{}

	for nodename := range nodenameToMachineMap.ControlPlanes {
		for _, app := range AllControlPlaneComponents {
			unreadyApps[pair.MakePair(nodename, app)] = struct{}{}
		}
	}

	for _, controlPlane := range kubernetesStatus.TypedSpec().Value.StaticPods {
		for _, app := range controlPlane.StaticPods {
			if app.Ready {
				delete(unreadyApps, pair.MakePair(controlPlane.Nodename, Component(app.App)))
			}

			appPatch(controlPlane.Nodename, Component(app.App), app.Version != desiredVersion)
		}
	}

	unreadyNodes := xslices.ToSet(append(maps.Keys(nodenameToMachineMap.ControlPlanes), maps.Keys(nodenameToMachineMap.Workers)...))

	for _, nodeInfo := range kubernetesStatus.TypedSpec().Value.Nodes {
		if nodeInfo.Ready {
			delete(unreadyNodes, nodeInfo.Nodename)
		}

		kubeletPatch(nodeInfo.Nodename, nodeInfo.KubeletVersion != desiredVersion)
	}

	switch {
	case len(unreadyNodes) == 0 && len(unreadyApps) == 0:
		upgradePath.AllComponentsReady = true
	case len(unreadyApps) > 0:
		notReadyApps := xslices.Map(maps.Keys(unreadyApps), func(p pair.Pair[string, Component]) string {
			return fmt.Sprintf("%s/%s", p.F1, p.F2)
		})
		slices.Sort(notReadyApps)

		upgradePath.NotReadyStatus = fmt.Sprintf("static pods %q are not ready", notReadyApps)

	case len(unreadyNodes) > 0:
		notReadyNodes := maps.Keys(unreadyNodes)
		slices.Sort(notReadyNodes)

		upgradePath.NotReadyStatus = fmt.Sprintf("nodes %q are not ready", notReadyNodes)
	}

	// try appending patches for not ready apps, as they might not show up, but still require patching
	for notReadyApp := range unreadyApps {
		appPatch(notReadyApp.F1, notReadyApp.F2, true)
	}

	// try appending patches for not ready nodes, as they might not show up, but still require patching
	for notReadyNode := range unreadyNodes {
		kubeletPatch(notReadyNode, true)
	}

	slices.SortFunc(upgradePath.Steps, func(a, b UpgradeStep) int {
		switch {
		case a.Less(b):
			return -1
		case b.Less(a):
			return +1
		default:
			return 0
		}
	})

	upgradePath.AllNodesToRequiredImages = make(map[string][]string, len(nodeToRequiredImageSet))

	// make sure to have deterministic order for the required images
	for node, requiredImageSet := range nodeToRequiredImageSet {
		requiredImages := maps.Keys(requiredImageSet)
		slices.Sort(requiredImages)

		upgradePath.AllNodesToRequiredImages[node] = requiredImages
	}

	return upgradePath
}
