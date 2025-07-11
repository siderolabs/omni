// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package siderolink contains SideroLink controller resources.
package siderolink

import (
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
)

// Namespace is the default namespace for the SideroLink resources.
const Namespace = resources.DefaultNamespace

// CounterNamespace is the default namespace for the SideroLink counter resources.
//
// tsgen:SiderolinkCounterNamespace
const CounterNamespace = resources.MetricsNamespace

func init() {
	registry.MustRegisterResource(ConnectionParamsType, &ConnectionParams{})
	registry.MustRegisterResource(ConfigType, &Config{})
	registry.MustRegisterResource(LinkType, &Link{})
	registry.MustRegisterResource(PendingMachineType, &PendingMachine{})
	registry.MustRegisterResource(PendingMachineStatusType, &PendingMachineStatus{})
	registry.MustRegisterResource(LinkStatusType, &LinkStatus{})
	registry.MustRegisterResource(ProviderJoinConfigType, &ProviderJoinConfig{})
	registry.MustRegisterResource(MachineJoinConfigType, &MachineJoinConfig{})
	registry.MustRegisterResource(APIConfigType, &APIConfig{})
	registry.MustRegisterResource(JoinTokenType, &JoinToken{})
	registry.MustRegisterResource(JoinTokenStatusType, &JoinTokenStatus{})
	registry.MustRegisterResource(JoinTokenUsageType, &JoinTokenUsage{})
	registry.MustRegisterResource(DefaultJoinTokenType, &DefaultJoinToken{})

	// NOTE: this resource is not used anymore, but still used in the migration code.
	registry.MustRegisterResource(DeprecatedLinkCounterType, &DeprecatedLinkCounter{})
}
