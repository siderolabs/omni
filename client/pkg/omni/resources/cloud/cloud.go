// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cloud provides resources for managing cloud resources.
package cloud

import "github.com/siderolabs/omni/client/pkg/omni/resources/registry"

func init() {
	registry.MustRegisterResource(MachineRequestType, &MachineRequest{})
	registry.MustRegisterResource(MachineRequestStatusType, &MachineRequestStatus{})
}
