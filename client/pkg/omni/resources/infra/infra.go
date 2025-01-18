// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package infra provides resources for managing infra resources.
package infra

import "github.com/siderolabs/omni/client/pkg/omni/resources/registry"

func init() {
	registry.MustRegisterResource(MachineRequestType, &MachineRequest{})
	registry.MustRegisterResource(MachineRequestStatusType, &MachineRequestStatus{})
	registry.MustRegisterResource(InfraMachineType, &Machine{})
	registry.MustRegisterResource(InfraMachineStatusType, &MachineStatus{})
	registry.MustRegisterResource(InfraProviderStatusType, &ProviderStatus{})
	registry.MustRegisterResource(ConfigPatchRequestType, &ConfigPatchRequest{})
	registry.MustRegisterResource(InfraProviderHealthStatusType, &ProviderHealthStatus{})
}
