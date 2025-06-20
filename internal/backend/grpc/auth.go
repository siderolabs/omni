// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	authpb "github.com/siderolabs/go-api-signature/api/auth"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
)

const (
	loginPath = "/omni/authenticate"

	// tsgen:authPublicKeyIDQueryParam
	publicKeyIDQueryParam = "public-key-id"

	awaitPublicKeyConfirmationTimeout = 5 * time.Minute
)

type authServer struct {
	authpb.UnimplementedAuthServiceServer

	state  state.State
	logger *zap.Logger
}

func (s *authServer) register(server grpc.ServiceRegistrar) {
	authpb.RegisterAuthServiceServer(server, s)
}

func (s *authServer) gateway(ctx context.Context, mux *gateway.ServeMux, address string, opts []grpc.DialOption) error {
	return authpb.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, address, opts)
}

// RegisterPublicKey registers a public key for the given identity.
// The registered key will be unconfirmed, and a login page URL will be returned.
func (s *authServer) RegisterPublicKey(ctx context.Context, request *authpb.RegisterPublicKeyRequest) (*authpb.RegisterPublicKeyResponse, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	email := strings.ToLower(request.GetIdentity().GetEmail())

	pubKey, err := validatePublicKey(request.GetPublicKey())
	if err != nil {
		return nil, err
	}

	loginURL, err := s.buildLoginURL(pubKey.id)
	if err != nil {
		return nil, err
	}

	result := &authpb.RegisterPublicKeyResponse{
		PublicKeyId: pubKey.id,
		LoginUrl:    loginURL,
	}

	identity, err := safe.StateGet[*authres.Identity](ctx, s.state, authres.NewIdentity(resources.DefaultNamespace, email).Metadata())
	if state.IsNotFoundError(err) {
		s.logger.Error("public key not registered, identity not found",
			zap.String("email", email),
			zap.String("fingerprint", pubKey.id),
		)

		// we do not fail explicitly to prevent user enumeration
		return result, nil
	}

	if err != nil {
		return nil, err
	}

	userID := identity.TypedSpec().Value.GetUserId()

	roleStr := request.GetRole()

	// if skipUserRole is false, we use the role of the user
	if !request.GetSkipUserRole() {
		var user *authres.User

		user, err = safe.StateGet[*authres.User](ctx, s.state, authres.NewUser(resources.DefaultNamespace, userID).Metadata())
		if state.IsNotFoundError(err) {
			s.logger.Error("public key not registered, user not found",
				zap.String("email", email),
				zap.String("user_id", userID),
				zap.String("fingerprint", pubKey.id),
			)

			// we do not fail explicitly to prevent user enumeration
			return result, nil
		}

		if err != nil {
			return nil, err
		}

		roleStr = user.TypedSpec().Value.Role
	}

	pubKeyRole, err := role.Parse(roleStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse role for public key: %w", err)
	}

	setPubKeyAttributes := func(k *authres.PublicKey) {
		k.Metadata().Labels().Set(authres.LabelPublicKeyUserID, userID)

		k.TypedSpec().Value.Confirmed = false
		k.TypedSpec().Value.PublicKey = pubKey.data
		k.TypedSpec().Value.Expiration = timestamppb.New(pubKey.expiration)
		k.TypedSpec().Value.Role = string(pubKeyRole)
		k.TypedSpec().Value.Identity = &specs.Identity{
			Email: email,
		}
	}

	newPubKey := authres.NewPublicKey(resources.DefaultNamespace, pubKey.id)

	_, err = safe.StateGet[*authres.PublicKey](ctx, s.state, newPubKey.Metadata())
	if state.IsNotFoundError(err) {
		setPubKeyAttributes(newPubKey)

		err = s.state.Create(ctx, newPubKey, state.WithCreateOwner(pointer.To(omni.KeyPrunerController{}).Name()))
		if err != nil {
			return nil, err
		}

		s.logger.Info("new public key registered",
			zap.String("email", email),
			zap.String("fingerprint", pubKey.id),
			zap.Time("expiration", pubKey.expiration),
			zap.String("role", newPubKey.TypedSpec().Value.GetRole()),
		)

		return result, nil
	}

	if err != nil {
		return nil, err
	}

	// it already exists, do nothing

	return result, nil
}

// AwaitPublicKeyConfirmation waits until the public key with the given information is confirmed.
func (s *authServer) AwaitPublicKeyConfirmation(ctx context.Context, request *authpb.AwaitPublicKeyConfirmationRequest) (*emptypb.Empty, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	ctx, cancel := context.WithTimeout(ctx, awaitPublicKeyConfirmationTimeout)
	defer cancel()

	pubKey := authres.NewPublicKey(resources.DefaultNamespace, request.GetPublicKeyId())

	_, err := s.state.WatchFor(ctx, pubKey.Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			pubKeyResource, ok := r.(*authres.PublicKey)
			if !ok {
				return false, errors.New("resource is not a PublicKey")
			}

			return pubKeyResource.TypedSpec().Value.GetConfirmed(), nil
		}))
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ConfirmPublicKey confirms the public key with the given ID.
// It uses the ID token in the request metadata to validate the user identity.
func (s *authServer) ConfirmPublicKey(ctx context.Context, request *authpb.ConfirmPublicKeyRequest) (*emptypb.Empty, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	email, err := verifiedEmail(ctx)
	if err != nil {
		return nil, err
	}

	if request.GetPublicKeyId() == "" {
		return nil, status.Error(codes.InvalidArgument, "public key id is required")
	}

	identity, err := safe.StateGet[*authres.Identity](ctx, s.state, authres.NewIdentity(resources.DefaultNamespace, email).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.PermissionDenied, "The identity %q is not authorized for this instance", email)
		}

		return nil, err
	}

	pubKey, err := safe.StateGet[*authres.PublicKey](ctx, s.state, authres.NewPublicKey(resources.DefaultNamespace, request.GetPublicKeyId()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Error(codes.PermissionDenied, "permission denied")
		}

		return nil, err
	}

	userID := identity.TypedSpec().Value.UserId

	existingUserID, ok := pubKey.Metadata().Labels().Get(authres.LabelPublicKeyUserID)
	if !ok || existingUserID != userID {
		return nil, errors.New("public key <> id mismatch")
	}

	_, err = safe.StateUpdateWithConflicts(ctx, s.state, pubKey.Metadata(), func(pk *authres.PublicKey) error {
		pk.TypedSpec().Value.Confirmed = true

		return nil
	}, state.WithUpdateOwner(pointer.To(omni.KeyPrunerController{}).Name()))
	if err != nil {
		return nil, err
	}

	s.logger.Info("public key confirmed",
		zap.String("email", email),
		zap.String("fingerprint", pubKey.Metadata().ID()),
		zap.Time("expiration", pubKey.TypedSpec().Value.GetExpiration().AsTime()),
		zap.String("role", pubKey.TypedSpec().Value.GetRole()),
	)

	return &emptypb.Empty{}, nil
}

func verifiedEmail(ctx context.Context) (string, error) {
	if email := debugEmail(ctx); email != "" {
		return email, nil
	}

	authCheckResult, err := auth.Check(ctx, auth.WithVerifiedEmail())
	if err != nil {
		return "", err
	}

	return authCheckResult.VerifiedEmail, nil
}

func (s *authServer) buildLoginURL(pgpKeyID string) (string, error) {
	loginURL, err := url.Parse(config.Config.Services.API.URL())
	if err != nil {
		return "", err
	}

	loginURL.Path = loginPath

	query := loginURL.Query()
	query.Set(publicKeyIDQueryParam, pgpKeyID)
	query.Set(auth.FlowQueryParam, auth.CLIAuthFlow)

	loginURL.RawQuery = query.Encode()

	return loginURL.String(), nil
}
