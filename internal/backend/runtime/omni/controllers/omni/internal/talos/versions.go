// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package talos provides helpers for controller Talos operations.
package talos

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// CalculateUpgradeVersions calculates the list of versions available for upgrade.
func CalculateUpgradeVersions(ctx context.Context, r controller.Reader, logger *zap.Logger, initialVersion, currentVersion, kubernetesVersion string) ([]string, error) {
	initialTalosVersion, err := semver.ParseTolerant(initialVersion)
	if err != nil {
		return nil, fmt.Errorf("error parsing initial Talos version: %w", err)
	}

	allTalosVersions, err := safe.ReaderListAll[*omni.TalosVersion](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list Talos versions: %w", err)
	}

	talosVersionByID := xslices.ToMap(slices.Collect(allTalosVersions.All()), func(v *omni.TalosVersion) (string, *omni.TalosVersion) {
		return v.Metadata().ID(), v
	})

	sourceID := strings.TrimPrefix(currentVersion, "v")

	candidates, err := getCandidates(talosVersionByID, sourceID, currentVersion)
	if err != nil {
		// Failed to calculate upgrade candidates. Assume that there are no upgrade versions available.
		logger.Warn("failed to calculate upgrade candidates", zap.Error(err))

		return make([]string, 0), nil //nolint:nilerr
	}

	availableVersions := make(map[string]semver.Version, len(candidates))

	for _, v := range candidates {
		tv, ok := talosVersionByID[v]
		if !ok {
			continue
		}

		if tv.TypedSpec().Value.Unsupported {
			continue
		}

		availableVersions[v], err = semver.ParseTolerant(v)
		if err != nil {
			return nil, fmt.Errorf("error parsing Talos version: %w", err)
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

		if k8sVersion.SupportedWith(candidate) != nil {
			delete(availableVersions, version)
		}
	}

	// skip versions which are not compatible with the initial version of Talos
	for version, candidate := range availableVersions {
		if candidate.Major < initialTalosVersion.Major || (candidate.Major == initialTalosVersion.Major && candidate.Minor < initialTalosVersion.Minor) {
			delete(availableVersions, version)
		}
	}

	availableVersionsList := maps.Keys(availableVersions)
	slices.SortFunc(availableVersionsList, func(a, b string) int {
		return availableVersions[a].Compare(availableVersions[b])
	})

	return availableVersionsList, nil
}

func getCandidates(talosVersionByID map[string]*omni.TalosVersion, sourceID string, currentVersion string) ([]string, error) {
	var candidates []string

	if source, ok := talosVersionByID[sourceID]; ok {
		candidates = source.TypedSpec().Value.UpgradableTalosVersions
	} else {
		// Source TalosVersion is gone (deprecated/denylisted after the cluster was provisioned).
		// Compute targets directly via the compatibility library so the cluster can still migrate away from a withdrawn version.
		sourceCompat, err := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: currentVersion})
		if err != nil {
			return nil, err
		}

		for id := range talosVersionByID {
			if id == sourceID {
				continue
			}

			targetCompat, parseErr := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: id})
			if parseErr != nil {
				continue
			}

			if targetCompat.UpgradeableFrom(sourceCompat) != nil {
				continue
			}

			candidates = append(candidates, id)
		}
	}

	return candidates, nil
}
