// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
)

// LifecycleServiceMinTalosVersion is the minimum Talos version that supports the LifecycleService API
// tsgen:LifecycleServiceMinTalosVersion
const LifecycleServiceMinTalosVersion = "1.13"

// MachineCompatibleWithCluster reports whether a machine can join a cluster running clusterVersion.
func MachineCompatibleWithCluster(
	ms *MachineStatus,
	clusterVersion semver.Version,
) (ok bool, willAutoInstall bool, reason string) {
	if ms.TypedSpec().Value.GetSchematic().GetInAgentMode() {
		return true, false, ""
	}

	return MachineLabelsCompatibleWithCluster(ms.Metadata().Labels(), clusterVersion)
}

// MachineLabelsCompatibleWithCluster is a labels-only variant of MachineCompatibleWithCluster for callers with only a machine's labels (e.g. the MachineSetNode allocator).
func MachineLabelsCompatibleWithCluster(labels *resource.Labels, clusterVersion semver.Version) (ok bool, willAutoInstall bool, reason string) {
	versionStr, hasVersion := labels.Get(MachineStatusLabelTalosVersion)
	if !hasVersion {
		return false, false, "the machine has no Talos version label"
	}

	machineVersion, err := semver.Parse(strings.TrimPrefix(versionStr, "v"))
	if err != nil {
		return false, false, "the machine's Talos version could not be determined"
	}

	_, installed := labels.Get(MachineStatusLabelInstalled)

	return versionCompatibleWithCluster(machineVersion, installed, clusterVersion)
}

// ParseTalosVersionLifecycleSupport parses a Talos version and reports whether it supports the LifecycleService install/upgrade APIs.
func ParseTalosVersionLifecycleSupport(version string) (semver.Version, bool) {
	parsed, err := semver.ParseTolerant(version)
	if err != nil {
		return semver.Version{}, false
	}

	return parsed, talosVersionSupportsLifecycleService(parsed)
}

// versionCompatibleWithCluster applies the version/installed compatibility rules to extracted inputs.
func versionCompatibleWithCluster(machineVersion semver.Version, installed bool, clusterVersion semver.Version) (ok bool, willAutoInstall bool, reason string) {
	// Refuse downgrade (e.g. installed at 1.14 trying to join a 1.13 cluster).
	if machineVersion.Major > clusterVersion.Major || (machineVersion.Major == clusterVersion.Major && machineVersion.Minor > clusterVersion.Minor) {
		return false, false, "the machine has a newer Talos version installed: downgrade is not allowed. Upgrade the machine or change the cluster Talos version"
	}

	if installed {
		return true, false, ""
	}

	// Same major.minor: Omni applies config and Talos installs through the normal config path.
	if machineVersion.Major == clusterVersion.Major && machineVersion.Minor == clusterVersion.Minor {
		return true, false, ""
	}

	// Cross-minor and not-installed: Omni can support the install only if both sides are >= 1.13.
	if talosVersionSupportsLifecycleService(machineVersion) && talosVersionSupportsLifecycleService(clusterVersion) {
		return true, true, ""
	}

	return false, false, "the machine running from ISO or PXE must have the same major and minor version as the cluster " +
		"it is going to be added to. Please use another ISO or change the cluster Talos version"
}

// talosVersionSupportsLifecycleService reports whether the Talos version exposes the LifecycleService install/upgrade APIs.
func talosVersionSupportsLifecycleService(v semver.Version) bool {
	minTalosVersion, _ := semver.ParseTolerant(LifecycleServiceMinTalosVersion) //nolint:errcheck
	if v.Major > minTalosVersion.Major {
		return true
	}

	if v.Major < minTalosVersion.Major {
		return false
	}

	return v.Minor >= minTalosVersion.Minor
}
