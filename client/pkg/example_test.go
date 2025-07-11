// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg_test

import (
	"os"

	"github.com/siderolabs/omni/client/pkg/omnictl"
	"github.com/siderolabs/omni/client/pkg/version"
)

//nolint:wsl,testableexamples
func Example() {
	// This is an example of building omnictl executable.
	version.Name = "omni"
	version.SHA = "build SHA" // Optional.
	version.Tag = "v0.29.0"   // Optional.
	version.API = 1           // Required: omnictl validates that the client has the same API version as the server.

	// You can disable this validation and warnings by setting:
	// version.SuppressVersionWarning = true

	// Initialize Root cmd version.
	omnictl.RootCmd.Version = version.String()

	// Run Root command.
	if err := omnictl.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
