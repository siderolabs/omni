// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

// Exported for testing: the two conversions between a provision step's spec and the management API, so
// they can be checked without standing up a management service to answer the RPC.
var (
	InstallationMediaRequest      = installationMediaRequest
	InstallationMediaFromResponse = installationMediaFromResponse
)
