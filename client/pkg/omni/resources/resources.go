// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resources defines various internal Omni resources.
package resources

import "github.com/cosi-project/runtime/pkg/resource"

// DefaultNamespace is the default namespace for all resources.
//
// DefaultNamespace has persistence enabled.
//
// tsgen:DefaultNamespace
const DefaultNamespace resource.Namespace = "default"

// EphemeralNamespace is the namespace for resources which are not persisted.
//
// EphemeralNamespace has no persistence.
//
// tsgen:EphemeralNamespace
const EphemeralNamespace resource.Namespace = "ephemeral"

// MetricsNamespace is the namespace for resources that store metrics, such as counters.
// It is backed by the secondary storage which is optimized for frequently updated data and has relaxed consistency guarantees.
//
// tsgen:MetricsNamespace
const MetricsNamespace resource.Namespace = "metrics"

// VirtualNamespace is the namespace where resources are virtual (synthetic),
// i.e. they behave like resources but not actual resources. For example, a resource whose contents change
// based on the requester user's identity.
//
// VirtualNamespace has no persistence.
//
// tsgen:VirtualNamespace
const VirtualNamespace resource.Namespace = "virtual"

// ExternalNamespace is the namespace where resources are external
//
// ExternalNamespace has no persistence.
//
// tsgen:ExternalNamespace
const ExternalNamespace resource.Namespace = "external"

// CloudProviderNamespace is the namespace for cloud provider specific resources, e.g., `MachineRequest` and `MachineRequestStatus`.
const CloudProviderNamespace resource.Namespace = "cloud-provider"

// CloudProviderSpecificNamespacePrefix is the prefix for cloud provider specific namespaces.
//
// A cloud-provider specific namespace is a namespace in which cloud provider has full access.
//
// For example, a cloud provider named `qemu-1` would have full access on namespace `cloud-provider:qemu-1`.
const CloudProviderSpecificNamespacePrefix resource.Namespace = CloudProviderNamespace + ":"
