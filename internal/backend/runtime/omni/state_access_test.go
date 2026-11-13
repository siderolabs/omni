// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// TestCheckForKindAccessClusterTerms covers the cluster a label query is authorized against.
//
// The user below has no Omni-wide role and reaches cluster1 only through an access policy, so a query
// that does not select cluster1 and nothing else must be rejected.
func TestCheckForKindAccessClusterTerms(t *testing.T) {
	t.Parallel()

	const identity = "user-1@example.com"

	st := state.WrapCore(inmem.NewState())

	accessPolicy := authres.NewAccessPolicy()
	accessPolicy.TypedSpec().Value.Rules = []*specs.AccessPolicyRule{
		{
			Users:    []string{identity},
			Clusters: []string{"cluster1"},
			Role:     string(role.Reader),
		},
	}

	require.NoError(t, st.Create(t.Context(), accessPolicy))
	require.NoError(t, st.Create(t.Context(), authres.NewIdentity(identity)))

	ctx := ctxstore.WithValue(t.Context(), auth.RoleContextKey{Role: role.None})
	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: identity})

	clusterTerm := func(clusterID string, opts ...resource.TermOption) resource.LabelTerm {
		var query resource.LabelQuery

		resource.LabelEqual(omnires.LabelCluster, clusterID, opts...)(&query)

		return query.Terms[0]
	}

	for name, tt := range map[string]struct {
		terms   []resource.LabelTerm
		allowed bool
	}{
		"the cluster the policy covers": {
			terms:   []resource.LabelTerm{clusterTerm("cluster1")},
			allowed: true,
		},
		"the same cluster twice": {
			terms:   []resource.LabelTerm{clusterTerm("cluster1"), clusterTerm("cluster1")},
			allowed: true,
		},
		"another cluster": {
			terms: []resource.LabelTerm{clusterTerm("cluster2")},
		},
		"every cluster but the covered one": {
			terms: []resource.LabelTerm{clusterTerm("cluster1", resource.NotMatches)},
		},
		"the covered cluster, with another one excluded": {
			terms:   []resource.LabelTerm{clusterTerm("cluster1"), clusterTerm("cluster2", resource.NotMatches)},
			allowed: true,
		},
		"two different clusters, the covered one first": {
			terms: []resource.LabelTerm{clusterTerm("cluster1"), clusterTerm("cluster2")},
		},
		"two different clusters, the covered one last": {
			terms: []resource.LabelTerm{clusterTerm("cluster2"), clusterTerm("cluster1")},
		},
		"no cluster at all": {
			terms: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := omni.CheckForKindAccess(ctx, st, state.List, omnires.NewClusterMachineStatus("").Metadata(), tt.terms)

			if tt.allowed {
				assert.NoError(t, err)

				return
			}

			assert.Equal(t, codes.PermissionDenied, status.Code(err), "unexpected error: %v", err)
		})
	}
}

// TestKernelArgsIsNotClusterScoped verifies that a cluster label on a KernelArgs resource does not
// authorize the request.
func TestKernelArgsIsNotClusterScoped(t *testing.T) {
	t.Parallel()

	const identity = "user-2@example.com"

	innerSt := state.WrapCore(inmem.NewState())

	accessPolicy := authres.NewAccessPolicy()
	accessPolicy.TypedSpec().Value.Rules = []*specs.AccessPolicyRule{
		{
			Users:    []string{identity},
			Clusters: []string{"alpha"},
			Role:     string(role.Operator),
		},
	}

	require.NoError(t, innerSt.Create(t.Context(), accessPolicy))
	require.NoError(t, innerSt.Create(t.Context(), authres.NewIdentity(identity)))

	st := state.WrapCore(validated.NewState(innerSt, omni.AuthorizationValidationOptions(innerSt)...))

	ctx := ctxstore.WithValue(t.Context(), auth.RoleContextKey{Role: role.None})
	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: identity})

	// a cluster resource of the covered cluster is still writable through the policy
	configPatch := omnires.NewConfigPatch("patch-1")
	configPatch.Metadata().Labels().Set(omnires.LabelCluster, "alpha")

	assert.NoError(t, st.Create(ctx, configPatch))

	// kernel args name a machine, so a cluster label does not scope them
	kernelArgs := omnires.NewKernelArgs("m1")
	kernelArgs.Metadata().Labels().Set(omnires.LabelCluster, "alpha")

	err := st.Create(ctx, kernelArgs)

	assert.Equal(t, codes.PermissionDenied, status.Code(err), "unexpected error: %v", err)
}

// TestInfraProviderTypesOutsideTheirNamespaces covers infra provider types written into a namespace that is not served by the infra provider state.
//
// The infra provider state checks the access to these types, so a namespace it does not serve must not skip the regular checks.
func TestInfraProviderTypesOutsideTheirNamespaces(t *testing.T) {
	t.Parallel()

	innerSt := state.WrapCore(inmem.NewState())
	st := state.WrapCore(validated.NewState(innerSt, omni.AuthorizationValidationOptions(innerSt)...))

	for _, r := range []role.Role{role.None, role.Reader, role.Admin} {
		ctx := ctxstore.WithValue(t.Context(), auth.RoleContextKey{Role: r})

		for _, res := range []resource.Resource{
			typed.NewResource[infra.ProviderSpec, infra.ProviderExtension](
				resource.NewMetadata(resources.DefaultNamespace, infra.ProviderType, "rogue", resource.VersionUndefined),
				protobuf.NewResourceSpec(&specs.InfraProviderSpec{}),
			),
			typed.NewResource[infra.MachineRequestSpec, infra.MachineRequestExtension](
				resource.NewMetadata(resources.DefaultNamespace, infra.MachineRequestType, "rogue", resource.VersionUndefined),
				protobuf.NewResourceSpec(&specs.MachineRequestSpec{}),
			),
		} {
			err := st.Create(ctx, res)

			assert.Equal(t, codes.PermissionDenied, status.Code(err), "role %s, type %s", r, res.Metadata().Type())
		}
	}
}

// TestInfraProviderSpecificNamespace checks that the resources in the namespace of an infra provider, which have types of its own,
// are left to the infra provider state.
func TestInfraProviderSpecificNamespace(t *testing.T) {
	t.Parallel()

	innerSt := state.WrapCore(inmem.NewState())
	st := state.WrapCore(validated.NewState(innerSt, omni.AuthorizationValidationOptions(innerSt)...))

	ctx := ctxstore.WithValue(t.Context(), auth.RoleContextKey{Role: role.InfraProvider})

	res := typed.NewResource[infra.ProviderSpec, infra.ProviderExtension](
		resource.NewMetadata(resources.InfraProviderSpecificNamespacePrefix+"qemu-1", "TestRess.qemu-1.infraprovider.sidero.dev", "test", resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraProviderSpec{}),
	)

	require.NoError(t, st.Create(ctx, res))
}
