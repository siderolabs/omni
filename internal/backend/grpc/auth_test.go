// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-api-signature/api/auth"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access/role"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/grpc"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
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

// TestPublicKeyRegisterConfirm checks that a registered key is stored only once it is confirmed, and that a wait for an
// unknown key takes as long as a wait for a registered one. Overlapping confirmations of one key all succeed.
func TestPublicKeyRegisterConfirm(t *testing.T) {
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

	user := authres.NewUser(identity.TypedSpec().Value.UserId)
	require.NoError(t, st.Create(ctx, user))

	otherIdentity := authres.NewIdentity("other@a.com")
	otherIdentity.TypedSpec().Value.UserId = "8c2e0f11-5a33-4f8d-bb97-1d4e6a0c7b32"
	require.NoError(t, st.Create(ctx, otherIdentity))

	otherCtx := ctxstore.WithValue(t.Context(), omniauth.VerifiedEmailContextKey{Email: "other@a.com"})

	registerWithLifetime := func(email, keyPem string, lifetime time.Duration) string {
		resp, registerErr := authServer.RegisterPublicKey(ctx, &auth.RegisterPublicKeyRequest{
			Identity: &auth.Identity{Email: email},
			PublicKey: &auth.PublicKey{
				PlainKey: &auth.PublicKey_Plain{
					KeyPem:    keyPem,
					NotBefore: timestamppb.Now(),
					NotAfter:  timestamppb.New(time.Now().Add(lifetime)),
				},
			},
		})
		require.NoError(t, registerErr)

		return resp.PublicKeyId
	}

	register := func(email, keyPem string) string {
		return registerWithLifetime(email, keyPem, time.Hour)
	}

	awaitWithin := func(keyID string, timeout time.Duration) error {
		awaitCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		_, awaitErr := authServer.AwaitPublicKeyConfirmation(awaitCtx, &auth.AwaitPublicKeyConfirmationRequest{PublicKeyId: keyID})

		return awaitErr
	}

	keyID := register("caller@a.com", `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8N0YkTeVTfD8xgJsjSMgvAmZquzv
LwfQb9Oa7fBNdyIiS2GPVzSFQtcIYbxBYBzvEY8RZjteEf7e/c/WWznGTQ==
-----END PUBLIC KEY-----`)

	_, err = safe.StateGetByID[*authres.PublicKey](ctx, st, keyID)
	require.True(t, state.IsNotFoundError(err), "the key must not be stored before it is confirmed")

	// a registered key and a key of an unknown email wait the same way
	registeredErr := awaitWithin(keyID, 100*time.Millisecond)
	unknownErr := awaitWithin(register("nobody@a.com", `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEsJ+S5HzNM92M894Nv1oa+wWBVBDf
hsVo7TvVWjCmeFq64NncUawbmwbeWaYoSahFCc7jRVqrL+txdG9p+9+YHg==
-----END PUBLIC KEY-----`), 100*time.Millisecond)

	require.Error(t, registeredErr)
	require.Equal(t, registeredErr.Error(), unknownErr.Error())

	// another user cannot confirm the key, and it is not stored
	_, err = authServer.ConfirmPublicKey(otherCtx, &auth.ConfirmPublicKeyRequest{PublicKeyId: keyID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = safe.StateGetByID[*authres.PublicKey](ctx, st, keyID)
	require.True(t, state.IsNotFoundError(err))

	var eg errgroup.Group

	eg.Go(func() error { return awaitWithin(keyID, 10*time.Second) })

	for range 4 {
		eg.Go(func() error {
			_, confirmErr := authServer.ConfirmPublicKey(ctx, &auth.ConfirmPublicKeyRequest{PublicKeyId: keyID})

			return confirmErr
		})
	}

	require.NoError(t, eg.Wait())

	stored, err := safe.StateGetByID[*authres.PublicKey](ctx, st, keyID)
	require.NoError(t, err)
	require.True(t, stored.TypedSpec().Value.Confirmed) //nolint:staticcheck
	require.Equal(t, identity.TypedSpec().Value.UserId, stored.Metadata().Labels().Raw()[authres.LabelPublicKeyUserID])

	// a wait after the confirmation returns without waiting, and a second confirmation succeeds
	require.NoError(t, awaitWithin(keyID, time.Second))

	_, err = authServer.ConfirmPublicKey(ctx, &auth.ConfirmPublicKeyRequest{PublicKeyId: keyID})
	require.NoError(t, err)

	// a registered key whose ID is taken by a key of another user in the meantime is not confirmed
	takenID := register("caller@a.com", `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEtIVVNP0J82GHkzOMh5DWZpmaw8No
rjVb6m+7ndeoP+9CB3SOfmjkCJDJpr/Mgc4u6sh8leeEMXjUFGRg1nsY8A==
-----END PUBLIC KEY-----`)

	taken := authres.NewPublicKey(takenID)
	taken.Metadata().Labels().Set(authres.LabelPublicKeyUserID, otherIdentity.TypedSpec().Value.UserId)
	require.NoError(t, st.Create(ctx, taken))

	_, err = authServer.ConfirmPublicKey(ctx, &auth.ConfirmPublicKeyRequest{PublicKeyId: takenID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// a registered key that expired before the confirmation is not confirmed
	expiredID := registerWithLifetime("caller@a.com", `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE3WDXzo5+zBpxy8gVRLkIGSFmUEb6
e3jZVPlrk+078y8/+99sKfmGskU+ynBsKNglG/uy2LO1yl8WGXhJtXJF+w==
-----END PUBLIC KEY-----`, 200*time.Millisecond)

	time.Sleep(300 * time.Millisecond)

	_, err = authServer.ConfirmPublicKey(ctx, &auth.ConfirmPublicKeyRequest{PublicKeyId: expiredID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// a key stored unconfirmed by an earlier version is confirmed in place
	stored = authres.NewPublicKey("6b0f92c4ad314e7f88a3c5d1e0724b19")
	stored.Metadata().Labels().Set(authres.LabelPublicKeyUserID, identity.TypedSpec().Value.UserId)
	stored.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(time.Hour))
	require.NoError(t, st.Create(ctx, stored, state.WithCreateOwner(new(omnictrl.KeyPrunerController{}).Name())))

	_, err = authServer.ConfirmPublicKey(ctx, &auth.ConfirmPublicKeyRequest{PublicKeyId: stored.Metadata().ID()})
	require.NoError(t, err)
	require.NoError(t, awaitWithin(stored.Metadata().ID(), time.Second))
}

// TestRegisterPublicKeyLimited checks that registrations from one address are limited, and other addresses are not affected.
func TestRegisterPublicKeyLimited(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	authServer, err := grpc.NewAuthServer(st, config.Services{
		Api: config.Service{
			AdvertisedURL: new("http://localhost:8099"),
		},
	}, zaptest.NewLogger(t))
	require.NoError(t, err)

	register := func(ip string) error {
		ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-forwarded-for", ip))

		_, registerErr := authServer.RegisterPublicKey(ctx, &auth.RegisterPublicKeyRequest{
			Identity: &auth.Identity{Email: "nobody@a.com"},
			PublicKey: &auth.PublicKey{
				PlainKey: &auth.PublicKey_Plain{
					KeyPem: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8N0YkTeVTfD8xgJsjSMgvAmZquzv
LwfQb9Oa7fBNdyIiS2GPVzSFQtcIYbxBYBzvEY8RZjteEf7e/c/WWznGTQ==
-----END PUBLIC KEY-----`,
					NotBefore: timestamppb.Now(),
					NotAfter:  timestamppb.New(time.Now().Add(time.Hour)),
				},
			},
		})

		return registerErr
	}

	for range grpc.RegistrationBurst {
		require.NoError(t, register("203.0.113.9"))
	}

	require.Equal(t, codes.ResourceExhausted, status.Code(register("203.0.113.9")))
	require.NoError(t, register("203.0.113.10"))
}
