// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// validateTalosVersion ensures the newVersion resolves to a known TalosVersion resource and is
// not deprecated, unless the current version is also deprecated (which allows patch upgrades
// within the deprecated line).
func validateTalosVersion(ctx context.Context, st state.State, current, newVersion string) error {
	var currentVersionIsDeprecated bool

	talosVersion, err := safe.StateGet[*omni.TalosVersion](ctx, st, omni.NewTalosVersion(newVersion).Metadata())
	if err != nil {
		return fmt.Errorf("invalid talos version %q: %w", newVersion, err)
	}

	if current != "" {
		var ver *omni.TalosVersion

		ver, err := safe.StateGet[*omni.TalosVersion](ctx, st, omni.NewTalosVersion(current).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if ver != nil {
			currentVersionIsDeprecated = ver.TypedSpec().Value.Deprecated
		}
	}

	// disallow updating to the deprecated Talos version from the non-deprecated one
	// 1.3.0 -> 1.3.7 should still work for example
	if talosVersion.TypedSpec().Value.Deprecated && !currentVersionIsDeprecated {
		return fmt.Errorf("talos version %q is no longer supported", newVersion)
	}

	return nil
}

// validateKubernetesVersion ensures the upgrade path from current to newVersion is supported,
// permitting in-flight upgrade aborts.
func validateKubernetesVersion(current, newVersion string, upgradeStatus *omni.KubernetesUpgradeStatus) error {
	if current == "" {
		return nil
	}

	// Allow aborting ongoing upgrade
	if upgradeStatus != nil && upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion != "" && upgradeStatus.TypedSpec().Value.LastUpgradeVersion != "" {
		previousVersion, err := semver.ParseTolerant(current)
		if err != nil {
			return err
		}

		futureVersion, err := semver.ParseTolerant(newVersion)
		if err != nil {
			return err
		}

		currentUpgradeVersion, err := semver.ParseTolerant(upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion)
		if err != nil {
			return err
		}

		lastUpgradeVersion, err := semver.ParseTolerant(upgradeStatus.TypedSpec().Value.LastUpgradeVersion)
		if err != nil {
			return err
		}

		if previousVersion.LT(currentUpgradeVersion) && futureVersion.LT(lastUpgradeVersion) {
			return nil
		}

		if futureVersion.EQ(lastUpgradeVersion) && previousVersion.EQ(currentUpgradeVersion) {
			return nil
		}
	}

	upgradePath, err := upgrade.NewPath(current, newVersion)
	if err != nil {
		return err
	}

	if !upgradePath.IsSupported() {
		return fmt.Errorf("kubernetes version is not supported for upgrade to %q from %q", newVersion, current)
	}

	return nil
}
