// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package system provides resources related to the DB state itself.
package system

import "github.com/siderolabs/omni/client/pkg/omni/resources/registry"

func init() {
	registry.MustRegisterResource(CertRefreshTickType, &CertRefreshTick{})
	registry.MustRegisterResource(DBVersionType, &DBVersion{})
	registry.MustRegisterResource(SysVersionType, &SysVersion{})
}
