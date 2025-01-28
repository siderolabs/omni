// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"fmt"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// UpgradeStep defines a change to Talos machine configuration which can be applied to fix the Kubernetes version.
type UpgradeStep struct {
	MachineID   string
	Description string
	Node        string
	Component   Component
	Patch       Patcher
}

// Less returns true if the patch should be applied before the other one.
//
// Order is based on the upgrade order.
func (p UpgradeStep) Less(other UpgradeStep) bool {
	if p.Component == other.Component {
		return p.Node < other.Node
	}

	return p.Component.Less(other.Component)
}

// Patcher returns the instructions to patch v1alpha1.Config to upgrade a specific component.
// It also returns the list of images used by it.
type Patcher struct {
	Apply      func(*v1alpha1.Config)
	UsedImages []string
}

// MultiPatcher combines several patches.
func MultiPatcher(patchers ...Patcher) Patcher {
	return Patcher{
		Apply: func(config *v1alpha1.Config) {
			for _, patcher := range patchers {
				patcher.Apply(config)
			}
		},
		UsedImages: xslices.FlatMap(patchers, func(p Patcher) []string {
			return p.UsedImages
		}),
	}
}

func patchKubelet(version string) Patcher {
	img := fmt.Sprintf("%s:v%s", constants.KubeletImage, version)

	return Patcher{
		Apply: func(config *v1alpha1.Config) {
			if config.MachineConfig == nil {
				config.MachineConfig = &v1alpha1.MachineConfig{}
			}

			if config.MachineConfig.MachineKubelet == nil {
				config.MachineConfig.MachineKubelet = &v1alpha1.KubeletConfig{}
			}

			config.MachineConfig.MachineKubelet.KubeletImage = img
		},
		UsedImages: []string{img},
	}
}

func patchAPIServer(version string) Patcher {
	img := fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, version)

	return Patcher{
		Apply: func(config *v1alpha1.Config) {
			if config.ClusterConfig == nil {
				config.ClusterConfig = &v1alpha1.ClusterConfig{}
			}

			if config.ClusterConfig.APIServerConfig == nil {
				config.ClusterConfig.APIServerConfig = &v1alpha1.APIServerConfig{}
			}

			config.ClusterConfig.APIServerConfig.ContainerImage = img
		},
		UsedImages: []string{img},
	}
}

func patchControllerManager(version string) Patcher {
	img := fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, version)

	return Patcher{
		Apply: func(config *v1alpha1.Config) {
			if config.ClusterConfig == nil {
				config.ClusterConfig = &v1alpha1.ClusterConfig{}
			}

			if config.ClusterConfig.ControllerManagerConfig == nil {
				config.ClusterConfig.ControllerManagerConfig = &v1alpha1.ControllerManagerConfig{}
			}

			config.ClusterConfig.ControllerManagerConfig.ContainerImage = img
		},
		UsedImages: []string{img},
	}
}

func patchScheduler(version string) Patcher {
	img := fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, version)

	return Patcher{
		Apply: func(config *v1alpha1.Config) {
			if config.ClusterConfig == nil {
				config.ClusterConfig = &v1alpha1.ClusterConfig{}
			}

			if config.ClusterConfig.SchedulerConfig == nil {
				config.ClusterConfig.SchedulerConfig = &v1alpha1.SchedulerConfig{}
			}

			config.ClusterConfig.SchedulerConfig.ContainerImage = img
		},
		UsedImages: []string{img},
	}
}

func patchKubeProxy(version string) Patcher {
	img := fmt.Sprintf("%s:v%s", constants.KubeProxyImage, version)

	return Patcher{
		Apply: func(config *v1alpha1.Config) {
			if config.ClusterConfig == nil {
				config.ClusterConfig = &v1alpha1.ClusterConfig{}
			}

			if config.ClusterConfig.ProxyConfig == nil {
				config.ClusterConfig.ProxyConfig = &v1alpha1.ProxyConfig{}
			}

			config.ClusterConfig.ProxyConfig.ContainerImage = img
		},
		UsedImages: []string{img},
	}
}
