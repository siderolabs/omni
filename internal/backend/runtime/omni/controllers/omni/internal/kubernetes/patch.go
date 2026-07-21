// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"bytes"
	"fmt"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/stdpatches"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// UpgradeStep defines a change to Talos machine configuration which can be applied to fix the Kubernetes version.
type UpgradeStep struct {
	MachineID   string
	Description string
	Node        string
	Component   Component
	Patch       Patcher
	Blocked     bool
}

// Less returns true if the patch should be applied before the other one.
//
// Order is based on the upgrade order. Blocked steps are handled last.
func (p UpgradeStep) Less(other UpgradeStep) bool {
	if p.Component == other.Component {
		switch {
		case p.Blocked && !other.Blocked:
			return false
		case !p.Blocked && other.Blocked:
			return true
		default:
			return p.Node < other.Node
		}
	}

	return p.Component.Less(other.Component)
}

// Patcher holds a machine config patch that upgrades a specific component.
// It also returns the list of images used by it.
type Patcher struct {
	Patch      []byte
	UsedImages []string
}

// MultiPatcher combines several patches.
func MultiPatcher(patchers ...Patcher) (Patcher, error) {
	if len(patchers) == 0 {
		return Patcher{}, fmt.Errorf("at least one patcher is required")
	}

	merged := patchers[0].Patch

	for _, p := range patchers[1:] {
		var err error

		merged, err = MergePatch(merged, p.Patch)
		if err != nil {
			return Patcher{}, err
		}
	}

	return Patcher{
		Patch: merged,
		UsedImages: xslices.FlatMap(patchers, func(p Patcher) []string {
			return p.UsedImages
		}),
	}, nil
}

// MergePatch merges a new patch into an existing patch.
func MergePatch(existing, newPatch []byte) ([]byte, error) {
	if len(bytes.TrimSpace(existing)) == 0 {
		return newPatch, nil
	}

	if len(bytes.TrimSpace(newPatch)) == 0 {
		return existing, nil
	}

	base, err := configloader.NewFromBytes(existing)
	if err != nil {
		return nil, fmt.Errorf("failed to parse existing patch: %w", err)
	}

	patch, err := configpatcher.LoadPatch(newPatch)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new patch: %w", err)
	}

	merged, err := configpatcher.Apply(configpatcher.WithConfig(base), []configpatcher.Patch{patch})
	if err != nil {
		return nil, fmt.Errorf("failed to merge patch: %w", err)
	}

	return merged.Bytes()
}

func patchKubelet(vc *config.VersionContract, version string) (Patcher, error) {
	img := fmt.Sprintf("%s:v%s", constants.KubeletImage, version)

	data, err := stdpatches.WithKubeletImage(vc, img)
	if err != nil {
		return Patcher{}, fmt.Errorf("failed to build kubelet patch: %w", err)
	}

	return Patcher{Patch: data, UsedImages: []string{img}}, nil
}

func patchAPIServer(vc *config.VersionContract, version string) (Patcher, error) {
	img := fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, version)

	data, err := stdpatches.WithKubeAPIServerImage(vc, img)
	if err != nil {
		return Patcher{}, fmt.Errorf("failed to build kube-apiserver patch: %w", err)
	}

	return Patcher{Patch: data, UsedImages: []string{img}}, nil
}

func patchControllerManager(vc *config.VersionContract, version string) (Patcher, error) {
	img := fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, version)

	data, err := stdpatches.WithKubeControllerManagerImage(vc, img)
	if err != nil {
		return Patcher{}, fmt.Errorf("failed to build kube-controller-manager patch: %w", err)
	}

	return Patcher{Patch: data, UsedImages: []string{img}}, nil
}

func patchScheduler(vc *config.VersionContract, version string) (Patcher, error) {
	img := fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, version)

	data, err := stdpatches.WithKubeSchedulerImage(vc, img)
	if err != nil {
		return Patcher{}, fmt.Errorf("failed to build kube-scheduler patch: %w", err)
	}

	return Patcher{Patch: data, UsedImages: []string{img}}, nil
}

func patchKubeProxy(vc *config.VersionContract, version string) (Patcher, error) {
	img := fmt.Sprintf("%s:v%s", constants.KubeProxyImage, version)

	data, err := stdpatches.WithKubeProxyImage(vc, img)
	if err != nil {
		return Patcher{}, fmt.Errorf("failed to build kube-proxy patch: %w", err)
	}

	// UsedImages is deliberately left empty so the kube-proxy image is not pre-pulled.
	return Patcher{Patch: data}, nil
}
