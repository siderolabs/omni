// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz

import (
	"context"
	"slices"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/go-api-signature/pkg/client/interceptor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/client"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

var (
	allVerbs  = []state.Verb{state.Get, state.List, state.Create, state.Update, state.Destroy}
	readVerbs = []state.Verb{state.Get, state.List}

	verbNames = map[state.Verb]string{state.Get: "get", state.List: "list", state.Create: "create", state.Update: "update", state.Destroy: "destroy"}
)

// resourceAccess is what the roles may do with a resource type.
type resourceAccess struct {
	resource resource.Resource
	role     role.Role // the role every verb needs, Reader for reads and Operator for writes when empty
	verbs    []state.Verb
}

func (a resourceAccess) allows(r role.Role, verb state.Verb) bool {
	if !slices.Contains(a.verbs, verb) {
		return false
	}

	required := a.role

	if required == "" {
		required = role.Reader

		if !verb.Readonly() {
			required = role.Operator
		}
	}

	return r.Check(required) == nil
}

// AssertResourceAccess checks that the access checks of the resource API work in a running Omni.
//
// The checks themselves are tested for every resource type, role and verb by TestAuthorizationGolden in
// internal/backend/runtime/omni, so one resource type for each kind of access is enough here.
func AssertResourceAccess(ctx context.Context, rootCli *client.Client, clients *ClientFactory) func(t *testing.T) {
	// the denials are expected, keep them out of the server logs
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(grpcutil.LogLevelOverrideMetadataKey, zapcore.PanicLevel.String()))

	return func(t *testing.T) {
		roles := []role.Role{role.None, role.Reader, role.Operator, role.Admin}
		states := make(map[role.Role]state.State, len(roles))

		for _, r := range roles {
			cli, err := clients.Get(ctx, r)
			require.NoError(t, err)

			currentUser, err := safe.StateGetByID[*virtual.CurrentUser](ctx, cli.Omni().State(), virtual.CurrentUserID)
			require.NoError(t, err)
			require.Equal(t, string(r), currentUser.TypedSpec().Value.GetRole())

			states[r] = cli.Omni().State()
		}

		unsignedCtx := context.WithValue(ctx, interceptor.SkipInterceptorContextKey{}, struct{}{})

		for _, access := range []resourceAccess{
			{resource: omni.NewMachineClass(uuid.NewString()), verbs: allVerbs},                   // managed by users
			{resource: omni.NewClusterStatus(uuid.NewString()), verbs: readVerbs},                 // written by controllers
			{resource: omni.NewTalosVersion(uuid.NewString()), verbs: readVerbs, role: role.None}, // readable by any signed-in caller
			{resource: authres.NewIdentity(uuid.NewString()), verbs: readVerbs, role: role.Admin},
			{resource: omni.NewClusterSecrets(uuid.NewString())}, // no access
		} {
			typ := access.resource.Metadata().Type()

			for _, verb := range allVerbs {
				err := do(unsignedCtx, rootCli.Omni().State(), access.resource, verb)
				assert.Equalf(t, codes.Unauthenticated, status.Code(err), "unsigned %s on %s: %v", verbNames[verb], typ, err)

				for _, r := range roles {
					err = do(ctx, states[r], access.resource, verb)
					assert.Equalf(t, access.allows(r, verb), !isDenied(err), "%s on %s as %s: %v", verbNames[verb], typ, r, err)
				}
			}
		}
	}
}

func do(ctx context.Context, st state.State, res resource.Resource, verb state.Verb) error {
	var err error

	switch verb { //nolint:exhaustive
	case state.Get:
		_, err = st.Get(ctx, res.Metadata())
	case state.List:
		_, err = st.List(ctx, res.Metadata())
	case state.Create:
		err = st.Create(ctx, res.DeepCopy())
	case state.Update:
		err = st.Update(ctx, res.DeepCopy())
	case state.Destroy:
		err = st.Destroy(ctx, res.Metadata())
	}

	return err
}

// isDenied reports whether the access checks refused the request.
// On writes, the COSI client turns PermissionDenied into an owner conflict, which the resources here cannot have otherwise.
func isDenied(err error) bool {
	return status.Code(err) == codes.PermissionDenied || state.IsOwnerConflictError(err)
}
