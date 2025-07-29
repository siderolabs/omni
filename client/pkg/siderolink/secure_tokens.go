// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"strings"

	"github.com/blang/semver"
)

var minSupportedSecureTokensVersion = semver.MustParse("1.6.0")

// SupportsSecureJoinTokens checks if the Talos version supports secure join tokens.
func SupportsSecureJoinTokens(talosVersion string) bool {
	v, err := semver.ParseTolerant(strings.TrimLeft(talosVersion, "v"))
	if err != nil {
		return false
	}

	return v.GTE(minSupportedSecureTokensVersion)
}
