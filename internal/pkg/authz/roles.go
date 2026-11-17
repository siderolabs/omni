// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz

import (
	"slices"

	"github.com/siderolabs/omni/client/pkg/access/role"
)

// Rule grants verbs on resource groups.
type Rule struct {
	Groups []ResourceGroup
	Verbs  Verbs
}

// Role is a named list of rules.
type Role struct {
	Name        string
	Description string
	Rules       []Rule
	// ClusterScoped is set for the roles which are about clusters, and can be granted on some clusters only.
	// Such a role reaches the resources of a machine only while the machine is in a cluster.
	ClusterScoped bool
}

// The built-in roles.
//
// Each one covers a single area, and they are combined: a caller can have several of them, each in its own scope.
// A preset, which is the role stored for a user (Reader, Auditor, Operator, Admin), stands for a fixed set of them, see PresetRoles.
var (
	ClusterViewer = &Role{
		Name:          "cluster-viewer",
		Description:   "Read clusters and the machines in them.",
		ClusterScoped: true,
		Rules: []Rule{
			{Groups: []ResourceGroup{Clusters, Machines}, Verbs: Read},
		},
	}

	ClusterManager = &Role{
		Name:          "cluster-manager",
		Description:   "Create, change and delete clusters, and operate the machines in them.",
		ClusterScoped: true,
		Rules: []Rule{
			{Groups: []ResourceGroup{Clusters}, Verbs: AllVerbs},
			{Groups: []ResourceGroup{Machines}, Verbs: Read},
		},
	}

	KubernetesUser = &Role{
		Name:          "kubernetes-user",
		Description:   "Access the Kubernetes API of clusters.",
		ClusterScoped: true,
	}

	WorkloadProxyUser = &Role{
		Name:          "workload-proxy-user",
		Description:   "Access the services of clusters which are exposed through the workload proxy.",
		ClusterScoped: true,
		Rules: []Rule{
			{Groups: []ResourceGroup{ExposedServices}, Verbs: Read},
		},
	}

	MachineViewer = &Role{
		Name:        "machine-viewer",
		Description: "Read all machines, their settings, join tokens and the infra providers.",
		Rules: []Rule{
			{Groups: []ResourceGroup{Machines, MachineManagement, Providers}, Verbs: Read},
		},
	}

	MachineManager = &Role{
		Name:        "machine-manager",
		Description: "Change the settings of machines, remove machines, and manage machine classes, machine requests and join tokens.",
		Rules: []Rule{
			{Groups: []ResourceGroup{Machines, MachineManagement}, Verbs: AllVerbs},
			{Groups: []ResourceGroup{Providers}, Verbs: Read},
		},
	}

	AuditLogViewer = &Role{
		Name:        "audit-log-viewer",
		Description: "Read the audit log.",
	}

	AccessManager = &Role{
		Name:        "access-manager",
		Description: "Manage users, service accounts and their access, the infra providers and the credentials Omni stores.",
		Rules: []Rule{
			{Groups: []ResourceGroup{AccessControl, Providers}, Verbs: AllVerbs},
		},
	}
)

// Roles returns the built-in roles.
func Roles() []*Role {
	return []*Role{ClusterViewer, ClusterManager, KubernetesUser, WorkloadProxyUser, MachineViewer, MachineManager, AuditLogViewer, AccessManager}
}

// allows reports whether a rule of the role grants the verb on the resource group.
func (r *Role) allows(group ResourceGroup, verb Verbs) bool {
	return slices.ContainsFunc(r.Rules, func(rule Rule) bool {
		return rule.Verbs&verb != 0 && slices.Contains(rule.Groups, group)
	})
}

// Preset is what Omni stores as the role of a user: Reader, Auditor, Operator or Admin.
//
// It is another name for role.Role, so that it is not confused with Role in this package.
// The users and the API still call it a role.
type Preset = role.Role

// PresetRoles returns the built-in roles a preset stands for.
func PresetRoles(preset Preset) []*Role {
	switch preset {
	case role.Reader:
		return []*Role{ClusterViewer, WorkloadProxyUser, MachineViewer}
	case role.Auditor:
		return []*Role{ClusterViewer, WorkloadProxyUser, MachineViewer, AuditLogViewer}
	case role.Operator:
		return []*Role{ClusterViewer, ClusterManager, WorkloadProxyUser, KubernetesUser, MachineViewer, MachineManager}
	case role.Admin:
		return []*Role{ClusterViewer, ClusterManager, WorkloadProxyUser, KubernetesUser, MachineViewer, MachineManager, AuditLogViewer, AccessManager}
	case role.None, role.InfraProvider:
		return nil
	default:
		return nil
	}
}
