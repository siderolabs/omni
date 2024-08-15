// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"crypto/rand"
	"io"

	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/zitadel/oidc/v3/pkg/op"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/oidc"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/pkg/auth"
)

// OIDCProvider provides a link to the OIDC implementation to authenticate the actual request.
type OIDCProvider interface {
	op.OpenIDProvider
	AuthenticateRequest(requestID, identity string) error
}

type oidcServer struct {
	oidc.UnimplementedOIDCServiceServer

	provider OIDCProvider
}

func (s *oidcServer) register(server grpc.ServiceRegistrar) {
	oidc.RegisterOIDCServiceServer(server, s)
}

func (s *oidcServer) gateway(ctx context.Context, mux *gateway.ServeMux, address string, opts []grpc.DialOption) error {
	return oidc.RegisterOIDCServiceHandlerFromEndpoint(ctx, mux, address, opts)
}

func (s *oidcServer) Authenticate(ctx context.Context, req *oidc.AuthenticateRequest) (*oidc.AuthenticateResponse, error) {
	authResult, err := auth.CheckGRPC(ctx, auth.WithValidSignature(true))
	if err != nil {
		return nil, err
	}

	identity := authResult.Identity
	if !authResult.AuthEnabled {
		identity = "anonymous@omni"
	}

	if err = s.provider.AuthenticateRequest(req.AuthRequestId, identity); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "failed to authenticate request: %s", err)
	}

	request, err := s.provider.Storage().AuthRequestByID(ctx, req.AuthRequestId)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "failed to authenticate request: %s", err)
	}

	if request.GetRedirectURI() == external.KeyCodeRedirectURL {
		var code string

		code, err = encodeToString(6)
		if err != nil {
			return nil, err
		}

		if err = s.provider.Storage().SaveAuthCode(ctx, req.AuthRequestId, code); err != nil {
			return nil, status.Errorf(codes.PermissionDenied, "failed to authenticate request: %s", err)
		}

		return &oidc.AuthenticateResponse{
			AuthCode: code,
		}, nil
	}

	issuer := s.provider.IssuerFromRequest(nil) // this is safe because we use op.StaticIssuer

	return &oidc.AuthenticateResponse{
		RedirectUrl: op.AuthCallbackURL(s.provider)(op.ContextWithIssuer(ctx, issuer), req.AuthRequestId),
	}, nil
}

func encodeToString(minBytes int) (string, error) {
	b := make([]byte, minBytes)

	n, err := io.ReadAtLeast(rand.Reader, b, minBytes)
	if n != minBytes {
		return "", err
	}

	for i := 0; i < len(b); i++ {
		b[i] = table[int(b[i])%len(table)]
	}

	return string(b), nil
}

var table = [...]byte{'1', '2', '3', '4', '5', '6', '7', '8', '9', '0', 'a', 'b', 'c', 'd', 'e', 'f'}
