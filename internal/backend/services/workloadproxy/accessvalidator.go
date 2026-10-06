// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// RoleProvider provides the current actor's role for a cluster.
type RoleProvider interface {
	RoleForCluster(ctx context.Context, id resource.ID) (role.Role, error)
}

// AccessPolicyRoleProvider provides the role for a cluster by looking into the context and evaluating access policies.
type AccessPolicyRoleProvider struct {
	state state.State
}

// NewAccessPolicyRoleProvider creates a new RoleProvider that uses the access policy in the state.
func NewAccessPolicyRoleProvider(state state.State) (*AccessPolicyRoleProvider, error) {
	if state == nil {
		return nil, errors.New("state is nil")
	}

	return &AccessPolicyRoleProvider{state: state}, nil
}

// RoleForCluster returns the role of the current user for the given cluster.
func (a *AccessPolicyRoleProvider) RoleForCluster(ctx context.Context, clusterID resource.ID) (role.Role, error) {
	accessRole, _, err := accesspolicy.RoleForCluster(ctx, clusterID, a.state)

	return accessRole, err
}

// SignatureAccessValidator validates a signature using PGP keys and roles/ACLs.
type SignatureAccessValidator struct {
	logger       *zap.Logger
	authenticate auth.AuthenticatorFunc
	roleProvider RoleProvider
}

// NewSignatureAccessValidator creates a new PGP signature validator.
func NewSignatureAccessValidator(authenticate auth.AuthenticatorFunc, roleProvider RoleProvider, logger *zap.Logger) (*SignatureAccessValidator, error) {
	if authenticate == nil {
		return nil, errors.New("authenticator is nil")
	}

	if roleProvider == nil {
		return nil, errors.New("role provider is nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &SignatureAccessValidator{
		logger:       logger,
		authenticate: authenticate,
		roleProvider: roleProvider,
	}, nil
}

// ValidateAccess validates the access to an exposed service in the given cluster ID,
// using the PGP public keys in the Omni database.
func (p *SignatureAccessValidator) ValidateAccess(ctx context.Context, publicKeyID, publicKeyIDSignatureBase64 string, clusterID resource.ID) error {
	singatureBytes, err := base64.StdEncoding.DecodeString(publicKeyIDSignatureBase64)
	if err != nil {
		return err
	}

	authenticator, err := p.authenticate(ctx, publicKeyID)
	if err != nil {
		return err
	}

	if err = authenticator.Verifier.Verify([]byte(publicKeyID), singatureBytes); err != nil {
		return err
	}

	ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: authenticator.Role})
	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: authenticator.Identity})

	accessRole, err := p.roleProvider.RoleForCluster(ctx, clusterID)
	if err != nil {
		return err
	}

	return accessRole.Check(role.Reader)
}
