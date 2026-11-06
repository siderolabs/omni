// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"

	"github.com/siderolabs/omni/client/pkg/constants"
)

var talosVersionFloor = semver.MustParse(constants.TalosVersionFloor)

// ParseTalosVersion parses the Talos version reported by a machine, without its pre-release part.
func ParseTalosVersion(talosVersion string) (semver.Version, error) {
	version, err := semver.ParseTolerant(strings.TrimLeft(talosVersion, "v"))
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to parse Talos version %q: %w", talosVersion, err)
	}

	version.Pre = nil

	return version, nil
}

// IsBelowTalosVersionFloor reports whether the Talos version is below constants.TalosVersionFloor.
func IsBelowTalosVersionFloor(talosVersion string) (bool, error) {
	version, err := ParseTalosVersion(talosVersion)
	if err != nil {
		return false, err
	}

	return version.LT(talosVersionFloor), nil
}

// IsBelowTalosVersionFloorOrMissing reports whether the Talos version is below constants.TalosVersionFloor or empty.
//
// Talos sends its version on SideroLink provision requests only since 1.6, so a missing version there means an older Talos.
func IsBelowTalosVersionFloorOrMissing(talosVersion string) (bool, error) {
	if talosVersion == "" {
		return true, nil
	}

	return IsBelowTalosVersionFloor(talosVersion)
}
