// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes

import (
	"context"
	"fmt"
	"slices"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// CalculateUpgradeVersions calculates the list of versions available for upgrade.
func CalculateUpgradeVersions(ctx context.Context, r controller.Reader, currentVersion, talosTag string) ([]string, error) {
	talosVersion, err := safe.ReaderGetByID[*omni.TalosVersion](ctx, r, talosTag)
	if err != nil {
		return nil, fmt.Errorf("error fetching Talos version: %w", err)
	}

	availableVersions := map[string]semver.Version{}

	for _, k8sVer := range talosVersion.TypedSpec().Value.CompatibleKubernetesVersions {
		if k8sVer != currentVersion {
			availableVersions[k8sVer], err = semver.ParseTolerant(k8sVer)
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

	availableVersionsList := maps.Keys(availableVersions)
	slices.SortFunc(availableVersionsList, func(a, b string) int {
		return availableVersions[a].Compare(availableVersions[b])
	})

	return availableVersionsList, nil
}
