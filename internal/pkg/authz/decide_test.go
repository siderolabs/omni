// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/access/role"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/authz"
)

func authenticated(grants ...authz.Grant) authz.Subject {
	return authz.Subject{Authenticated: true, Grants: grants}
}

func on(r *authz.Role, clusters ...string) authz.Grant {
	if len(clusters) == 0 {
		return authz.Grant{Role: r, Scope: authz.Scope{Global: true}}
	}

	return authz.Grant{Role: r, Scope: authz.Scope{Clusters: clusters}}
}

func TestCheckResource(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		subject authz.Subject
		target  authz.Target
		code    codes.Code
	}{
		{
			name:   "not authenticated",
			target: authz.Target{Type: omni.ClusterType, Verb: state.Get, Cluster: "a"},
			code:   codes.Unauthenticated,
		},
		{
			name:   "type no caller can access, not authenticated",
			target: authz.Target{Type: omni.ClusterSecretsType, Verb: state.Get, Cluster: "a"},
			code:   codes.Unauthenticated,
		},
		{
			name:   "public type without a signature",
			target: authz.Target{Type: authres.AuthConfigType, Verb: state.Get},
		},
		{
			name:    "type no caller can access",
			subject: authenticated(authz.PresetGrants(role.Admin)...),
			target:  authz.Target{Type: omni.ClusterSecretsType, Verb: state.Get, Cluster: "a"},
			code:    codes.PermissionDenied,
		},
		{
			name:    "omni itself",
			subject: authz.Subject{Internal: true},
			target:  authz.Target{Type: omni.ClusterSecretsType, Verb: state.Update, Cluster: "a"},
		},
		{
			name:    "authenticated without grants reads a type which needs none",
			subject: authenticated(),
			target:  authz.Target{Type: omni.TalosVersionType, Verb: state.List},
		},
		{
			name:    "authenticated without grants",
			subject: authenticated(),
			target:  authz.Target{Type: omni.ClusterType, Verb: state.Get, Cluster: "a"},
			code:    codes.PermissionDenied,
		},
		{
			name:    "verb the type never allows",
			subject: authenticated(authz.PresetGrants(role.Admin)...),
			target:  authz.Target{Type: omni.ClusterStatusType, Verb: state.Update, Cluster: "a"},
			code:    codes.PermissionDenied,
		},
		{
			name:    "viewer of the cluster reads it",
			subject: authenticated(on(authz.ClusterViewer, "a")),
			target:  authz.Target{Type: omni.ClusterType, Verb: state.Get, Cluster: "a"},
		},
		{
			name:    "viewer of the cluster does not change it",
			subject: authenticated(on(authz.ClusterViewer, "a")),
			target:  authz.Target{Type: omni.ClusterType, Verb: state.Update, Cluster: "a"},
			code:    codes.PermissionDenied,
		},
		{
			name:    "viewer of another cluster",
			subject: authenticated(on(authz.ClusterViewer, "b")),
			target:  authz.Target{Type: omni.ClusterType, Verb: state.Get, Cluster: "a"},
			code:    codes.PermissionDenied,
		},
		{
			name:    "cluster pattern",
			subject: authenticated(on(authz.ClusterManager, "prod-*")),
			target:  authz.Target{Type: omni.MachineSetType, Verb: state.Create, Cluster: "prod-1"},
		},
		{
			name:    "exact cluster is not a pattern",
			subject: authenticated(on(authz.ClusterViewer, authz.ExactCluster("prod-*"))),
			target:  authz.Target{Type: omni.ClusterType, Verb: state.Get, Cluster: "prod-1"},
			code:    codes.PermissionDenied,
		},
		{
			name:    "list of all clusters needs more than one cluster",
			subject: authenticated(on(authz.ClusterViewer, "a", "b*")),
			target:  authz.Target{Type: omni.ClusterType, Verb: state.List, AllClusters: true},
			code:    codes.PermissionDenied,
		},
		{
			name:    "list of all clusters with the all clusters pattern",
			subject: authenticated(on(authz.ClusterViewer, authz.AllClustersPattern)),
			target:  authz.Target{Type: omni.ClusterType, Verb: state.List, AllClusters: true},
		},
		{
			name:    "cluster grant does not reach a resource in no cluster",
			subject: authenticated(on(authz.ClusterManager, authz.AllClustersPattern)),
			target:  authz.Target{Type: omni.MachineSetType, Verb: state.Create},
			code:    codes.PermissionDenied,
		},
		{
			name:    "viewer of the cluster reads a machine in it",
			subject: authenticated(on(authz.ClusterViewer, "a")),
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.Get, Cluster: "a"},
		},
		{
			name:    "viewer of all clusters does not read a machine in no cluster",
			subject: authenticated(on(authz.ClusterViewer, authz.AllClustersPattern)),
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.Get},
			code:    codes.PermissionDenied,
		},
		{
			name:    "viewer of all clusters does not list the machines",
			subject: authenticated(on(authz.ClusterViewer, authz.AllClustersPattern)),
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.List},
			code:    codes.PermissionDenied,
		},
		{
			name:    "viewer everywhere reads a machine in no cluster",
			subject: authenticated(on(authz.ClusterViewer)),
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.Get},
		},
		{
			name:    "machine viewer reads a machine in no cluster",
			subject: authenticated(on(authz.MachineViewer)),
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.Get},
		},
		{
			name:    "manager everywhere changes a config patch of a cluster",
			subject: authenticated(on(authz.ClusterManager)),
			target:  authz.Target{Type: omni.ConfigPatchType, Verb: state.Update, Cluster: "a"},
		},
		{
			name:    "manager everywhere does not change a config patch of a machine",
			subject: authenticated(on(authz.ClusterManager)),
			target:  authz.Target{Type: omni.ConfigPatchType, Verb: state.Update},
			code:    codes.PermissionDenied,
		},
		{
			name:    "machine manager changes a config patch of a machine",
			subject: authenticated(on(authz.MachineManager)),
			target:  authz.Target{Type: omni.ConfigPatchType, Verb: state.Update},
		},
		{
			name:    "machine manager does not change a config patch of a cluster",
			subject: authenticated(on(authz.MachineManager)),
			target:  authz.Target{Type: omni.ConfigPatchType, Verb: state.Update, Cluster: "a"},
			code:    codes.PermissionDenied,
		},
		{
			name:    "suspended instance allows reads",
			subject: authz.Subject{Authenticated: true, Suspended: true, Grants: authz.PresetGrants(role.Admin)},
			target:  authz.Target{Type: omni.ClusterType, Verb: state.Get, Cluster: "a"},
		},
		{
			name:    "suspended instance refuses writes",
			subject: authz.Subject{Authenticated: true, Suspended: true, Grants: authz.PresetGrants(role.Admin)},
			target:  authz.Target{Type: omni.ClusterType, Verb: state.Update, Cluster: "a"},
			code:    codes.PermissionDenied,
		},
		{
			name:    "infra provider reads machines",
			subject: authz.Subject{Authenticated: true, Provider: true},
			target:  authz.Target{Type: omni.MachineType, Verb: state.List},
		},
		{
			name:    "infra provider reads nothing else",
			subject: authz.Subject{Authenticated: true, Provider: true},
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.List},
			code:    codes.PermissionDenied,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := authz.CheckResource(tt.subject, tt.target)

			assert.Equal(t, tt.code, status.Code(err), "%v", err)
		})
	}
}

func TestPresetGrants(t *testing.T) {
	t.Parallel()

	assert.Empty(t, authz.PresetGrants(role.None))
	assert.Empty(t, authz.PresetGrants(role.InfraProvider))

	for _, preset := range []role.Role{role.Reader, role.Auditor, role.Operator, role.Admin} {
		for _, grant := range authz.PresetGrants(preset) {
			assert.True(t, grant.Scope.Global, "%s as %s", grant.Role.Name, preset)
		}
	}
}

func TestErrNoGrant(t *testing.T) {
	t.Parallel()

	target := authz.Target{Type: omni.ClusterType, Verb: state.Update, Cluster: "a"}

	// no grant covers the request: more grants can change the answer
	err := authz.CheckResource(authenticated(on(authz.ClusterViewer, "a")), target)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.ErrorIs(t, err, authz.ErrNoGrant)

	// a suspended instance is read-only, whatever the grants are
	err = authz.CheckResource(authz.Subject{Authenticated: true, Suspended: true, Grants: []authz.Grant{on(authz.ClusterManager, "a")}}, target)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.NotErrorIs(t, err, authz.ErrNoGrant)

	// a request without a signature
	assert.NotErrorIs(t, authz.CheckResource(authz.Subject{}, target), authz.ErrNoGrant)

	// an infra provider has no grants, and more grants are not what it lacks
	assert.NotErrorIs(t, authz.CheckResource(authz.Subject{Authenticated: true, Provider: true}, target), authz.ErrNoGrant)

	// the checks of a resource group and of an operation mark a missing grant the same way
	assert.ErrorIs(t, authz.CheckResourceGroup(authenticated(on(authz.ClusterViewer, "a")), authz.Clusters, authz.Update, target), authz.ErrNoGrant)
	assert.ErrorIs(t, authz.CheckOperation(authenticated(on(authz.ClusterViewer, "a")), authz.ClusterDiagnostics, target), authz.ErrNoGrant)
}

func TestCheckOperation(t *testing.T) {
	t.Parallel()

	suspended := func(grants ...authz.Grant) authz.Subject {
		return authz.Subject{Authenticated: true, Suspended: true, Grants: grants}
	}

	for _, tt := range []struct {
		name      string
		operation authz.Operation
		subject   authz.Subject
		target    authz.Target
		code      codes.Code
	}{
		{name: "not signed in", operation: authz.AuditLog, code: codes.Unauthenticated},
		{name: "omni itself", operation: authz.BreakGlass, subject: authz.Subject{Internal: true}},
		{name: "infra provider", operation: authz.MachinePower, subject: authz.Subject{Authenticated: true, Provider: true}, code: codes.PermissionDenied},
		{name: "no grants", operation: authz.AuditLog, subject: authenticated(), code: codes.PermissionDenied},
		{name: "role without the operation", operation: authz.AuditLog, subject: authenticated(on(authz.ClusterManager)), code: codes.PermissionDenied},
		{name: "role with the operation", operation: authz.AuditLog, subject: authenticated(on(authz.AuditLogViewer))},
		{
			name: "cluster manager gets the diagnostics of its cluster", operation: authz.ClusterDiagnostics,
			subject: authenticated(on(authz.ClusterManager, "a")), target: authz.Target{Cluster: "a"},
		},
		{
			name: "cluster manager gets the diagnostics of another cluster", operation: authz.ClusterDiagnostics,
			subject: authenticated(on(authz.ClusterManager, "a")), target: authz.Target{Cluster: "b"}, code: codes.PermissionDenied,
		},
		{
			name: "cluster viewer gets no diagnostics", operation: authz.ClusterDiagnostics,
			subject: authenticated(on(authz.ClusterViewer, "a")), target: authz.Target{Cluster: "a"}, code: codes.PermissionDenied,
		},
		{
			name: "cluster manager powers a machine of its cluster", operation: authz.MachinePower,
			subject: authenticated(on(authz.ClusterManager, "a")), target: authz.Target{Cluster: "a"},
		},
		{
			name: "cluster manager powers a machine of another cluster", operation: authz.MachinePower,
			subject: authenticated(on(authz.ClusterManager, "a")), target: authz.Target{Cluster: "b"}, code: codes.PermissionDenied,
		},
		{
			name: "cluster manager of all clusters does not power a machine in no cluster", operation: authz.MachinePower,
			subject: authenticated(on(authz.ClusterManager, authz.AllClustersPattern)), code: codes.PermissionDenied,
		},
		{
			name: "machine manager powers a machine in no cluster", operation: authz.MachinePower,
			subject: authenticated(on(authz.MachineManager)),
		},
		{
			name: "machine manager powers a machine of a cluster", operation: authz.MachinePower,
			subject: authenticated(on(authz.MachineManager)), target: authz.Target{Cluster: "a"},
		},
		{
			name: "cluster manager has no maintenance", operation: authz.MachineMaintenance,
			subject: authenticated(on(authz.ClusterManager)), code: codes.PermissionDenied,
		},
		{
			name: "suspended, operation which only reads", operation: authz.ClusterDiagnostics,
			subject: suspended(on(authz.ClusterManager, "a")), target: authz.Target{Cluster: "a"},
		},
		{
			name: "suspended, any other operation", operation: authz.MachinePower,
			subject: suspended(on(authz.MachineManager)), code: codes.PermissionDenied,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.code, status.Code(authz.CheckOperation(tt.subject, tt.operation, tt.target)))
		})
	}
}

func TestCheckResourceGroup(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		group   authz.ResourceGroup
		subject authz.Subject
		target  authz.Target
		verb    authz.Verbs
		code    codes.Code
	}{
		{name: "not signed in", group: authz.Clusters, verb: authz.Read, code: codes.Unauthenticated},
		{name: "omni itself", group: authz.AccessControl, verb: authz.Destroy, subject: authz.Subject{Internal: true}},
		{
			name: "infra provider", group: authz.Providers, verb: authz.Read,
			subject: authz.Subject{Authenticated: true, Provider: true}, code: codes.PermissionDenied,
		},
		{
			name: "verb of the role on a cluster of the grant", group: authz.Clusters, verb: authz.Update,
			subject: authenticated(on(authz.ClusterManager, "a")), target: authz.Target{Cluster: "a"},
		},
		{
			name: "verb of the role on another cluster", group: authz.Clusters, verb: authz.Update,
			subject: authenticated(on(authz.ClusterManager, "a")), target: authz.Target{Cluster: "b"}, code: codes.PermissionDenied,
		},
		{
			name: "verb the role does not have", group: authz.Clusters, verb: authz.Update,
			subject: authenticated(on(authz.ClusterViewer)), target: authz.Target{Cluster: "a"}, code: codes.PermissionDenied,
		},
		{
			name: "grant on clusters does not reach the instance", group: authz.MachineManagement, verb: authz.Create,
			subject: authenticated(on(authz.ClusterManager, "a")), code: codes.PermissionDenied,
		},
		{
			name: "cluster viewer reads the machines of a cluster", group: authz.Machines, verb: authz.Read,
			subject: authenticated(on(authz.ClusterViewer)), target: authz.Target{Cluster: "a"},
		},
		{
			name: "viewer of all clusters does not read the machines in no cluster", group: authz.Machines, verb: authz.Read,
			subject: authenticated(on(authz.ClusterViewer, authz.AllClustersPattern)), code: codes.PermissionDenied,
		},
		{
			name: "suspended, read", group: authz.Clusters, verb: authz.Read,
			subject: authz.Subject{Authenticated: true, Suspended: true, Grants: authz.PresetGrants(role.Admin)},
		},
		{
			name: "suspended, write", group: authz.Clusters, verb: authz.Update,
			subject: authz.Subject{Authenticated: true, Suspended: true, Grants: authz.PresetGrants(role.Admin)}, code: codes.PermissionDenied,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.code, status.Code(authz.CheckResourceGroup(tt.subject, tt.group, tt.verb, tt.target)))
		})
	}
}

func TestCheckInstallDiskConfig(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		cluster string
		subject authz.Subject
		verb    state.Verb
		code    codes.Code
	}{
		{name: "no grants", verb: state.Get, subject: authenticated(), code: codes.PermissionDenied},
		{name: "machine in no cluster, manager of some cluster", verb: state.Create, subject: authenticated(on(authz.ClusterManager, "a"))},
		{name: "machine in no cluster, manager of every cluster", verb: state.Create, subject: authenticated(authz.Grant{Role: authz.ClusterManager, Scope: authz.Scope{Global: true}})},
		{name: "machine in no cluster, viewer of some cluster", verb: state.Get, subject: authenticated(on(authz.ClusterViewer, "a")), code: codes.PermissionDenied},
		{
			name: "machine in no cluster, manager of no cluster", verb: state.Create,
			subject: authenticated(authz.Grant{Role: authz.ClusterManager}), code: codes.PermissionDenied,
		},
		{name: "machine in a cluster, manager of that cluster", cluster: "a", verb: state.Destroy, subject: authenticated(on(authz.ClusterManager, "a"))},
		{name: "machine in a cluster, manager of another cluster", cluster: "b", verb: state.Update, subject: authenticated(on(authz.ClusterManager, "a")), code: codes.PermissionDenied},
		{name: "machine in a cluster, viewer of that cluster reads", cluster: "a", verb: state.Get, subject: authenticated(on(authz.ClusterViewer, "a"))},
		{name: "machine in a cluster, viewer of that cluster writes", cluster: "a", verb: state.Update, subject: authenticated(on(authz.ClusterViewer, "a")), code: codes.PermissionDenied},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := authz.CheckInstallDiskConfig(tt.subject, tt.verb, tt.cluster)

			assert.Equal(t, tt.code, status.Code(err))

			// a refusal is always for a missing grant, so the access policy is tried after it
			if err != nil {
				assert.ErrorIs(t, err, authz.ErrNoGrant)
			}
		})
	}
}
