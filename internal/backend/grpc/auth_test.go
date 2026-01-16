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
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestRegisterPublicKey(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	authServer, err := grpc.NewAuthServer(st, config.Services{
		Api: config.Service{
			AdvertisedURL: pointer.To("http://localhost:8099"),
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
