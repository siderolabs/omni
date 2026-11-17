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

func TestAllowResource(t *testing.T) {
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
			subject: authenticated(on(authz.ClusterViewer)),
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.Get},
			code:    codes.PermissionDenied,
		},
		{
			name:    "viewer of all clusters does not list the machines",
			subject: authenticated(on(authz.ClusterViewer)),
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.List},
			code:    codes.PermissionDenied,
		},
		{
			name:    "machine viewer reads a machine in no cluster",
			subject: authenticated(on(authz.MachineViewer)),
			target:  authz.Target{Type: omni.MachineStatusType, Verb: state.Get},
		},
		{
			name:    "manager of all clusters changes a config patch of a cluster",
			subject: authenticated(on(authz.ClusterManager)),
			target:  authz.Target{Type: omni.ConfigPatchType, Verb: state.Update, Cluster: "a"},
		},
		{
			name:    "manager of all clusters does not change a config patch of a machine",
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

			err := authz.AllowResource(tt.subject, tt.target)

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
