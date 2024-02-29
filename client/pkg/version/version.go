// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package version provides version information.
package version

import (
	"fmt"
)

var (
	// Upstream omnictl build copies these values from the parent project.

	// Name is set at build time.
	Name string
	// Tag is set at build time.
	Tag string
	// SHA should be set to the build hash.
	SHA string

	// API is the supported backend API version.
	API uint32

	// SuppressVersionWarning disable logs that warn library users that the pkg is built without version set.
	SuppressVersionWarning bool
)

// String returns the textual representation of the version.
func String() string {
	return fmt.Sprintf("%s (API Version: %d)", Tag, API)
}
