// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor_test

import (
	"context"
	"testing"

	cosiv1alpha1 "github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	authpb "github.com/siderolabs/go-api-signature/api/auth"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	resapi "github.com/siderolabs/omni/client/api/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/interceptor"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestEULACheckUnary(t *testing.T) {
	signedIn := ctxstore.WithValue(context.Background(), auth.IdentityContextKey{Identity: "alice@example.com"})

	getEulaAcceptance := &resapi.GetRequest{Type: authres.EulaAcceptanceType}
	createEulaAcceptance := &resapi.CreateRequest{
		Resource: &resapi.Resource{Metadata: &cosiv1alpha1.Metadata{Type: authres.EulaAcceptanceType}},
	}

	for _, test := range []struct {
		ctx      context.Context //nolint:containedctx
		req      any
		name     string
		method   string
		accepted bool
		allowed  bool
	}{
		{
			name:    "signing in carries no identity yet",
			ctx:     context.Background(),
			method:  resapi.ResourceService_Get_FullMethodName,
			req:     &resapi.GetRequest{Type: "Clusters.omni.sidero.dev"},
			allowed: true,
		},
		{
			name:    "a signed in user is held back",
			ctx:     signedIn,
			method:  resapi.ResourceService_Get_FullMethodName,
			req:     &resapi.GetRequest{Type: "Clusters.omni.sidero.dev"},
			allowed: false,
		},
		{
			name:     "and let through once it is accepted",
			ctx:      signedIn,
			method:   resapi.ResourceService_Get_FullMethodName,
			req:      &resapi.GetRequest{Type: "Clusters.omni.sidero.dev"},
			accepted: true,
			allowed:  true,
		},
		{
			name:    "reading the acceptance",
			ctx:     signedIn,
			method:  resapi.ResourceService_Get_FullMethodName,
			req:     getEulaAcceptance,
			allowed: true,
		},
		{
			name:    "accepting",
			ctx:     signedIn,
			method:  resapi.ResourceService_Create_FullMethodName,
			req:     createEulaAcceptance,
			allowed: true,
		},
		{
			name:    "signing out",
			ctx:     signedIn,
			method:  authpb.AuthService_RevokePublicKey_FullMethodName,
			req:     &authpb.RevokePublicKeyRequest{},
			allowed: true,
		},
		{
			name:    "listing the acceptance is not accepting it",
			ctx:     signedIn,
			method:  resapi.ResourceService_List_FullMethodName,
			req:     &resapi.ListRequest{Type: authres.EulaAcceptanceType},
			allowed: false,
		},
		{
			name:    "an internal actor",
			ctx:     actor.MarkContextAsInternalActor(signedIn),
			method:  resapi.ResourceService_Get_FullMethodName,
			req:     &resapi.GetRequest{Type: "Clusters.omni.sidero.dev"},
			allowed: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			if test.accepted {
				require.NoError(t, st.Create(context.Background(), authres.NewEulaAcceptance()))
			}

			called := false
			handler := func(context.Context, any) (any, error) {
				called = true

				return nil, nil //nolint:nilnil
			}

			check := interceptor.NewEULACheck(st, zaptest.NewLogger(t), "https://omni.example.com/")

			_, err := check.Unary()(test.ctx, test.req, &grpc.UnaryServerInfo{FullMethod: test.method}, handler)

			require.Equal(t, test.allowed, called)

			if test.allowed {
				require.NoError(t, err)

				return
			}

			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			require.Contains(t, err.Error(), "https://omni.example.com/eula")
		})
	}
}
