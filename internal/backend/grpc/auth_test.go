// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-api-signature/api/auth"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access/role"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/grpc"
	omniauth "github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestRegisterPublicKey(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	authServer, err := grpc.NewAuthServer(st, config.Services{
		Api: config.Service{
			AdvertisedURL: new("http://localhost:8099"),
		},
	}, zaptest.NewLogger(t))

	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
	defer cancel()

	email := "a@a.com"

	key := `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8N0YkTeVTfD8xgJsjSMgvAmZquzv
LwfQb9Oa7fBNdyIiS2GPVzSFQtcIYbxBYBzvEY8RZjteEf7e/c/WWznGTQ==
-----END PUBLIC KEY-----`

	for _, tt := range []struct {
		request    *auth.RegisterPublicKeyRequest
		checkError func(t *testing.T, e error)
		name       string
	}{
		{
			name: "no public key data",
			request: &auth.RegisterPublicKeyRequest{
				Identity: &auth.Identity{
					Email: email,
				},
			},
			checkError: func(t *testing.T, e error) {
				require.Equal(t, codes.InvalidArgument, status.Code(e))
			},
		},
		{
			name: "plain key",
			request: &auth.RegisterPublicKeyRequest{
				Identity: &auth.Identity{
					Email: email,
				},
				PublicKey: &auth.PublicKey{
					PlainKey: &auth.PublicKey_Plain{
						KeyPem:    key,
						NotBefore: timestamppb.Now(),
						NotAfter:  timestamppb.New(time.Now().Add(time.Hour * 7)),
					},
				},
			},
		},
		{
			name: "plain key expiration rejected",
			request: &auth.RegisterPublicKeyRequest{
				Identity: &auth.Identity{
					Email: email,
				},
				PublicKey: &auth.PublicKey{
					PlainKey: &auth.PublicKey_Plain{
						KeyPem:    key,
						NotBefore: timestamppb.Now(),
						NotAfter:  timestamppb.New(time.Now().Add(time.Hour * 9)),
					},
				},
			},
			checkError: func(t *testing.T, e error) {
				require.Equal(t, codes.InvalidArgument, status.Code(e))
			},
		},
		{
			name: "plain key wrong validity range",
			request: &auth.RegisterPublicKeyRequest{
				Identity: &auth.Identity{
					Email: email,
				},
				PublicKey: &auth.PublicKey{
					PlainKey: &auth.PublicKey_Plain{
						KeyPem:    key,
						NotBefore: timestamppb.New(time.Now().Add(time.Hour * 5)),
						NotAfter:  timestamppb.Now(),
					},
				},
			},
			checkError: func(t *testing.T, e error) {
				require.Equal(t, codes.InvalidArgument, status.Code(e))
			},
		},
		{
			name: "plain key wrong not range",
			request: &auth.RegisterPublicKeyRequest{
				Identity: &auth.Identity{
					Email: email,
				},
				PublicKey: &auth.PublicKey{
					PlainKey: &auth.PublicKey_Plain{
						KeyPem:    key,
						NotBefore: timestamppb.New(time.Now().Add(-time.Hour)),
						NotAfter:  timestamppb.New(time.Now().Add(-time.Minute * 10)),
					},
				},
			},
			checkError: func(t *testing.T, e error) {
				require.Equal(t, codes.InvalidArgument, status.Code(e))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err = authServer.RegisterPublicKey(ctx, tt.request)

			if tt.checkError != nil {
				tt.checkError(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

// TestRevokePublicKeyDenials checks that a missing key and a key owned by someone else are answered the same way.
func TestRevokePublicKeyDenials(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	authServer, err := grpc.NewAuthServer(st, config.Services{
		Api: config.Service{
			AdvertisedURL: new("http://localhost:8099"),
		},
	}, zaptest.NewLogger(t))
	require.NoError(t, err)

	ctx := ctxstore.WithValue(t.Context(), omniauth.RoleContextKey{Role: role.Admin})
	ctx = ctxstore.WithValue(ctx, omniauth.IdentityContextKey{Identity: "caller@a.com"})

	someoneElses := authres.NewPublicKey("aa9e26dbdc5b4e5d9fa5ed21d0f3a2e1")
	someoneElses.TypedSpec().Value.Identity = &specs.Identity{Email: "someone-else@a.com"}

	require.NoError(t, st.Create(ctx, someoneElses))

	_, othersErr := authServer.RevokePublicKey(ctx, &auth.RevokePublicKeyRequest{PublicKeyId: someoneElses.Metadata().ID()})
	_, missingErr := authServer.RevokePublicKey(ctx, &auth.RevokePublicKeyRequest{PublicKeyId: "4a5e1cf0a7b24bd2bb0a6f1d3e7c9088"})

	require.Error(t, othersErr)
	require.Error(t, missingErr)
	require.Equal(t, othersErr.Error(), missingErr.Error())
	require.Equal(t, codes.PermissionDenied, status.Code(othersErr))
}

// TestConfirmPublicKeyDenials checks that a missing key and a key owned by someone else are answered the same way.
func TestConfirmPublicKeyDenials(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	authServer, err := grpc.NewAuthServer(st, config.Services{
		Api: config.Service{
			AdvertisedURL: new("http://localhost:8099"),
		},
	}, zaptest.NewLogger(t))
	require.NoError(t, err)

	ctx := ctxstore.WithValue(t.Context(), omniauth.VerifiedEmailContextKey{Email: "caller@a.com"})

	identity := authres.NewIdentity("caller@a.com")
	identity.TypedSpec().Value.UserId = "3f1b7f26-97f6-4d0a-9b4e-2a6b1c0f9d55"

	require.NoError(t, st.Create(ctx, identity))

	someoneElses := authres.NewPublicKey("c1de4a7b90f24e6ab3d5182f6c0e9a44")
	someoneElses.Metadata().Labels().Set(authres.LabelPublicKeyUserID, "8c2e0f11-5a33-4f8d-bb97-1d4e6a0c7b32")

	require.NoError(t, st.Create(ctx, someoneElses))

	_, othersErr := authServer.ConfirmPublicKey(ctx, &auth.ConfirmPublicKeyRequest{PublicKeyId: someoneElses.Metadata().ID()})
	_, missingErr := authServer.ConfirmPublicKey(ctx, &auth.ConfirmPublicKeyRequest{PublicKeyId: "6b0f92c4ad314e7f88a3c5d1e0724b19"})

	require.Error(t, othersErr)
	require.Error(t, missingErr)
	require.Equal(t, othersErr.Error(), missingErr.Error())
	require.Equal(t, codes.PermissionDenied, status.Code(othersErr))
}
