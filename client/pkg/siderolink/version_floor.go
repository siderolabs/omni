// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"strings"

	"github.com/blang/semver/v4"

	"github.com/siderolabs/omni/client/pkg/constants"
)

var talosVersionFloor = semver.MustParse(constants.TalosVersionFloor)

// IsBelowTalosVersionFloor reports whether the Talos version is below constants.TalosVersionFloor.
//
// An empty or unparsable version is not below the floor.
func IsBelowTalosVersionFloor(talosVersion string) bool {
	parsed, err := semver.ParseTolerant(strings.TrimLeft(talosVersion, "v"))
	if err != nil {
		return false
	}

	parsed.Pre = nil

	return parsed.LT(talosVersionFloor)
}

// IsBelowTalosVersionFloorOrMissing reports whether the Talos version is below constants.TalosVersionFloor or empty.
//
// Talos sends its version on SideroLink provision requests only since 1.6, so a missing version there means an older Talos.
func IsBelowTalosVersionFloorOrMissing(talosVersion string) bool {
	return talosVersion == "" || IsBelowTalosVersionFloor(talosVersion)
}
