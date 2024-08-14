// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package talos provides helpers for controller Talos operations.
package talos

import (
	"context"
	"fmt"
	"slices"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// CalculateUpgradeVersions calculates the list of versions available for upgrade.
func CalculateUpgradeVersions(ctx context.Context, r controller.Reader, currentVersion, kubernetesVersion string) ([]string, error) {
	currentTalosVersion, err := compatibility.ParseTalosVersion(&machine.VersionInfo{
		Tag: currentVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("error parsing Talos version: %w", err)
	}

	talosVersions, err := safe.ReaderListAll[*omni.TalosVersion](ctx, r)
	if err != nil {
		return nil, err
	}

	availableVersions := map[string]semver.Version{}

	for val := range talosVersions.All() {
		if val.TypedSpec().Value.Version != currentVersion {
			availableVersions[val.TypedSpec().Value.Version], err = semver.ParseTolerant(val.TypedSpec().Value.Version)
			if err != nil {
				return nil, err
			}
		}
	}

	k8sVersion, err := compatibility.ParseKubernetesVersion(kubernetesVersion)
	if err != nil {
		return nil, err
	}

	// skip versions which are not supported with the current version of Kubernetes
	for version := range availableVersions {
		var candidate *compatibility.TalosVersion

		candidate, err = compatibility.ParseTalosVersion(&machine.VersionInfo{
			Tag: version,
		})
		if err != nil {
			return nil, fmt.Errorf("error parsing Talos version: %w", err)
		}

		if candidate.UpgradeableFrom(currentTalosVersion) != nil || k8sVersion.SupportedWith(candidate) != nil {
			delete(availableVersions, version)
		}
	}

	availableVersionsList := maps.Keys(availableVersions)
	slices.SortFunc(availableVersionsList, func(a, b string) int {
		return availableVersions[a].Compare(availableVersions[b])
	})

	return availableVersionsList, nil
}
