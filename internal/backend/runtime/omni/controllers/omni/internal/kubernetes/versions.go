// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"context"
	"fmt"
	"slices"

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// CalculateUpgradeVersions calculates the list of versions available for upgrade.
func CalculateUpgradeVersions(ctx context.Context, r controller.Reader, currentVersion, talosTag string) ([]string, error) {
	talosVersion, err := compatibility.ParseTalosVersion(&machine.VersionInfo{
		Tag: talosTag,
	})
	if err != nil {
		return nil, fmt.Errorf("error parsing Talos version: %w", err)
	}

	k8sVersions, err := safe.ReaderListAll[*omni.KubernetesVersion](ctx, r)
	if err != nil {
		return nil, err
	}

	availableVersions := map[string]semver.Version{}

	for iter := k8sVersions.Iterator(); iter.Next(); {
		if iter.Value().TypedSpec().Value.Version != currentVersion {
			availableVersions[iter.Value().TypedSpec().Value.Version], err = semver.ParseTolerant(iter.Value().TypedSpec().Value.Version)
			if err != nil {
				return nil, err
			}
		}
	}

	// skip versions which do not have an upgrade path from the current version
	for version := range availableVersions {
		path, err := upgrade.NewPath(currentVersion, version)
		if err != nil {
			return nil, err
		}

		if !path.IsSupported() {
			delete(availableVersions, version)
		}
	}

	// skip versions which are not supported with the current version of Talos
	for version := range availableVersions {
		k8sVersion, err := compatibility.ParseKubernetesVersion(version)
		if err != nil {
			return nil, err
		}

		if k8sVersion.SupportedWith(talosVersion) != nil {
			delete(availableVersions, version)
		}
	}

	availableVersionsList := maps.Keys(availableVersions)
	slices.SortFunc(availableVersionsList, func(a, b string) int {
		return availableVersions[a].Compare(availableVersions[b])
	})

	return availableVersionsList, nil
}
