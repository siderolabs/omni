// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import "github.com/cosi-project/runtime/pkg/resource"

// UpdateChange is a pair of old/new resources.
type UpdateChange struct {
	Old resource.Resource
	New resource.Resource
}

// SyncResult describes the actions to perform to sync the template resources.
type SyncResult struct {
	// Resources to create.
	Create []resource.Resource
	// Resources to update.
	Update []UpdateChange
	// Resources to delete split by phases.
	Destroy [][]resource.Resource
}
