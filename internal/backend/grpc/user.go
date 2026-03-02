// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/api/omni/management"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/auth/user"
)

func (s *managementServer) CreateUser(ctx context.Context, req *management.CreateUserRequest) (*management.CreateUserResponse, error) {
	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin)); err != nil {
		return nil, err
	}

	userID, err := user.Create(ctx, s.omniState, req.Email, req.Role)
	if err != nil {
		return nil, wrapError(err)
	}

	return &management.CreateUserResponse{UserId: userID}, nil
}

func (s *managementServer) UpdateUser(ctx context.Context, req *management.UpdateUserRequest) (*emptypb.Empty, error) {
	checkResult, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(checkResult.Identity, req.Email) {
		return nil, status.Error(codes.InvalidArgument, "updating your own role is not allowed")
	}

	if err := user.Update(ctx, s.omniState, req.Email, req.Role); err != nil {
		return nil, wrapError(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *managementServer) ListUsers(ctx context.Context, _ *emptypb.Empty) (*management.ListUsersResponse, error) {
	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin)); err != nil {
		return nil, err
	}

	identities, err := safe.StateListAll[*authres.Identity](ctx, s.omniState, state.WithLabelQuery(
		resource.LabelExists(authres.LabelIdentityTypeServiceAccount, resource.NotMatches),
	))
	if err != nil {
		return nil, err
	}

	users, err := safe.StateListAll[*authres.User](ctx, s.omniState)
	if err != nil {
		return nil, err
	}

	identityStatuses, err := safe.StateListAll[*authres.IdentityStatus](ctx, s.omniState, state.WithLabelQuery(
		resource.LabelExists(authres.LabelIdentityTypeServiceAccount, resource.NotMatches),
	))
	if err != nil {
		return nil, err
	}

	userByID := make(map[string]*authres.User, users.Len())
	for usr := range users.All() {
		userByID[usr.Metadata().ID()] = usr
	}

	identityStatusByID := make(map[string]*authres.IdentityStatus, identityStatuses.Len())
	for is := range identityStatuses.All() {
		identityStatusByID[is.Metadata().ID()] = is
	}

	result := make([]*management.ListUsersResponse_User, 0, identities.Len())

	for identity := range identities.All() {
		u := &management.ListUsersResponse_User{
			Id:    identity.TypedSpec().Value.UserId,
			Email: identity.Metadata().ID(),
		}

		if foundUser, ok := userByID[identity.TypedSpec().Value.UserId]; ok {
			u.Role = foundUser.TypedSpec().Value.Role
		}

		if is, ok := identityStatusByID[identity.Metadata().ID()]; ok {
			u.LastActive = is.TypedSpec().Value.LastActive
		}

		samlLabels := map[string]string{}

		for key, value := range identity.Metadata().Labels().Raw() {
			if !strings.HasPrefix(key, authres.SAMLLabelPrefix) {
				continue
			}

			samlLabels[strings.TrimPrefix(key, authres.SAMLLabelPrefix)] = value
		}

		if len(samlLabels) > 0 {
			u.SamlLabels = samlLabels
		}

		result = append(result, u)
	}

	return &management.ListUsersResponse{Users: result}, nil
}

func (s *managementServer) DestroyUser(ctx context.Context, req *management.DestroyUserRequest) (*emptypb.Empty, error) {
	checkResult, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(checkResult.Identity, req.Email) {
		return nil, status.Error(codes.InvalidArgument, "destroying your own account is not allowed")
	}

	err = user.Destroy(ctx, s.omniState, req.Email)

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Errorf(codes.NotFound, "user %q not found", req.Email)
	case errors.Is(err, user.ErrIsServiceAccount):
		return nil, status.Errorf(codes.InvalidArgument, "%s", err)
	case err != nil:
		return nil, wrapError(err)
	}

	return &emptypb.Empty{}, nil
}
