// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

var (
	// ErrUnauthenticated is returned when the context does not contain the required authentication information.
	ErrUnauthenticated = errors.New("unauthenticated")

	// ErrUnauthorized is returned when the context does not contain the required authorization information.
	ErrUnauthorized = errors.New("unauthorized")
)

// CheckOptions are the options for the checks.
type CheckOptions struct {
	Role           role.Role
	VerifiedEmail  bool
	ValidSignature bool
}

// DefaultCheckOptions returns the default check options.
func DefaultCheckOptions() CheckOptions {
	return CheckOptions{
		Role: role.None,
	}
}

// CheckResult is the result of a successful check.
type CheckResult struct {
	VerifiedEmail     string
	Identity          string
	UserID            string
	Labels            map[string]string
	Role              role.Role
	HasValidSignature bool
	AuthEnabled       bool
}

// CheckOption is a functional option for Check.
type CheckOption func(*CheckOptions)

// WithRole checks the context to have the given role.
//
// If the required role is other than role.None, WithValidSignature is ignored and the signature is always checked.
func WithRole(role role.Role) CheckOption {
	return func(opts *CheckOptions) {
		opts.Role = role
	}
}

// WithValidSignature checks if the context has a valid signature.
//
// If the required role set via WithRole is other than role.None, this setting is ignored and the signature is always checked.
func WithValidSignature(validSignature bool) CheckOption {
	return func(opts *CheckOptions) {
		opts.ValidSignature = validSignature
	}
}

// WithVerifiedEmail checks if there is a verified email in the context.
func WithVerifiedEmail() CheckOption {
	return func(opts *CheckOptions) {
		opts.VerifiedEmail = true
	}
}

// Check checks the given context for the given authentication and authorization conditions.
//
// The returned error can be checked against ErrUnauthenticated and ErrUnauthorized.
func Check(ctx context.Context, opt ...CheckOption) (CheckResult, error) {
	authVal, ok := ctxstore.Value[EnabledAuthContextKey](ctx)
	if !ok {
		return CheckResult{}, fmt.Errorf("%w: auth configuration not found in context", ErrUnauthenticated)
	}

	if !authVal.Enabled {
		return CheckResult{
			AuthEnabled: false,
		}, nil
	}

	result := CheckResult{
		AuthEnabled: authVal.Enabled,
	}

	opts := DefaultCheckOptions()

	for _, o := range opt {
		o(&opts)
	}

	// If the required role is other than role.None, we always check the signature.
	if opts.Role != role.None {
		opts.ValidSignature = true
	}

	if opts.VerifiedEmail {
		emailVal, ok := ctxstore.Value[VerifiedEmailContextKey](ctx)
		if !ok {
			return CheckResult{}, fmt.Errorf("%w: missing verified email", ErrUnauthenticated)
		}

		result.VerifiedEmail = emailVal.Email
	}

	ctxRole := role.None
	ctxRoleExists := false

	if val, ok := ctxstore.Value[RoleContextKey](ctx); ok {
		ctxRole = val.Role
		ctxRoleExists = true
	}

	result.Role = ctxRole

	// RoleContextKey is set on the context only when there is a valid signature, so we can rely on this.
	result.HasValidSignature = ctxRoleExists

	if opts.ValidSignature && !result.HasValidSignature {
		return CheckResult{}, fmt.Errorf("%w: missing valid signature", ErrUnauthenticated)
	}

	if opts.Role != role.None {
		err := ctxRole.Check(opts.Role)
		if err != nil {
			return CheckResult{}, fmt.Errorf("%w: %v", ErrUnauthorized, err) //nolint:errorlint
		}
	}

	if val, ok := ctxstore.Value[IdentityContextKey](ctx); ok {
		result.Identity = val.Identity
	}

	if val, ok := ctxstore.Value[UserIDContextKey](ctx); ok {
		result.UserID = val.UserID
	}

	return result, nil
}

// CheckGRPC wraps Check function returning gRPC error codes.
func CheckGRPC(ctx context.Context, opt ...CheckOption) (CheckResult, error) {
	result, err := Check(ctx, opt...)
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			return CheckResult{}, status.Errorf(codes.Unauthenticated, "%s", err)
		}

		if errors.Is(err, ErrUnauthorized) {
			return CheckResult{}, status.Errorf(codes.PermissionDenied, "%s", err)
		}

		return CheckResult{}, err
	}

	return result, nil
}
