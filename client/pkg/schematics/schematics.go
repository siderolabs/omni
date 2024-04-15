// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package schematics contains helpers for schematic generation.
package schematics

import (
	"strings"

	"github.com/blang/semver"
)

var supportedVersion = semver.MustParse("1.7.0")

// SupportsOverlays checks if Talos version supports overlays.
func SupportsOverlays(version string) (bool, error) {
	talosVersion, err := semver.Parse(strings.TrimPrefix(version, "v"))
	if err != nil {
		return false, err
	}

	talosVersion.Pre = nil

	return talosVersion.GTE(supportedVersion), nil
}
