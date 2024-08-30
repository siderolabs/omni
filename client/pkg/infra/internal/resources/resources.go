// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resources contains COSI resource helpers for the infra provider.
package resources

import "github.com/siderolabs/omni/client/pkg/omni/resources"

// ResourceType generates the correct resource name for the resources managed by the infra providers.
func ResourceType(name, providerID string) string {
	return name + "." + providerID + ".infraprovider.sidero.dev"
}

// ResourceNamespace generates the correct namespace name for the infra provider state.
func ResourceNamespace(providerID string) string {
	return resources.InfraProviderSpecificNamespacePrefix + providerID
}
