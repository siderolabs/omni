// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz

// Operation is an action a role can allow, which is not a verb on a resource group.
type Operation string

// The operations.
const (
	// AuditLog is reading the audit log.
	AuditLog Operation = "audit-log"
	// BreakGlass is getting the configs which connect to the machines and the Kubernetes API of a cluster directly, bypassing Omni.
	BreakGlass Operation = "break-glass"
	// ClusterDiagnostics is collecting the support bundle of a cluster, and checking whether its Kubernetes can be upgraded.
	// Both are read-only, but they run with Omni's own access, so read access to the cluster is not enough for them.
	ClusterDiagnostics Operation = "cluster-diagnostics"
	// MachinePower is powering a machine on and off.
	MachinePower Operation = "machine-power"
	// MachineMaintenance is using the Talos API of a machine which is in no cluster, e.g., to install or upgrade Talos.
	// The Talos API has no roles in maintenance mode, so this operation also covers reading.
	MachineMaintenance Operation = "machine-maintenance"
)

// readOnlyOperations are the operations which change nothing, so they stay allowed on a suspended instance.
var readOnlyOperations = []Operation{AuditLog, ClusterDiagnostics}
