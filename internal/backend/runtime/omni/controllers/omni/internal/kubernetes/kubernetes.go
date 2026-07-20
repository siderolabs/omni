// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package kubernetes provides helpers for controller Kubernetes operations.
package kubernetes

import "github.com/siderolabs/talos/pkg/machinery/config"

// Component is an enumeration of Kubernetes components.
type Component string

// Kubernetes components.
const (
	APIServer         Component = "kube-apiserver"
	ControllerManager Component = "kube-controller-manager"
	Scheduler         Component = "kube-scheduler"
	Kubelet           Component = "kubelet"
)

// AllControlPlaneComponents is a list of all Kubernetes controlplane components.
var AllControlPlaneComponents = []Component{
	APIServer,
	ControllerManager,
	Scheduler,
}

// Patch returns a patcher for a specific component.
func (c Component) Patch(vc *config.VersionContract, version string) (Patcher, error) {
	switch c {
	case APIServer:
		apiServerPatch, err := patchAPIServer(vc, version)
		if err != nil {
			return Patcher{}, err
		}

		kubeProxyPatch, err := patchKubeProxy(vc, version)
		if err != nil {
			return Patcher{}, err
		}

		return MultiPatcher(apiServerPatch, kubeProxyPatch)
	case ControllerManager:
		return patchControllerManager(vc, version)
	case Scheduler:
		return patchScheduler(vc, version)
	case Kubelet:
		return patchKubelet(vc, version)
	default:
		panic("unknown component")
	}
}

// Valid returns true if the component is valid.
func (c Component) Valid() bool {
	switch c {
	case APIServer, ControllerManager, Scheduler, Kubelet:
		return true
	default:
		return false
	}
}

// Less returns true if the component is less than the other.
//
// Order is based on the upgrade order.
func (c Component) Less(other Component) bool {
	switch c {
	case APIServer:
		return true
	case ControllerManager:
		return other != APIServer
	case Scheduler:
		return other == Kubelet
	case Kubelet:
		return false
	default:
		panic("unknown component")
	}
}
