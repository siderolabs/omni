// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"encoding/base64"
	"errors"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
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

// PGPAccessValidator validates a signature using PGP keys and roles/ACLs.
type PGPAccessValidator struct {
	logger       *zap.Logger
	state        state.State
	roleProvider RoleProvider
}

// NewPGPAccessValidator creates a new PGP signature validator.
func NewPGPAccessValidator(state state.State, roleProvider RoleProvider, logger *zap.Logger) (*PGPAccessValidator, error) {
	if state == nil {
		return nil, errors.New("state is nil")
	}

	if roleProvider == nil {
		return nil, errors.New("role provider is nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &PGPAccessValidator{
		logger:       logger,
		state:        state,
		roleProvider: roleProvider,
	}, nil
}

// ValidateAccess validates the access to an exposed service in the given cluster ID,
// using the PGP public keys in the Omni database.
func (p *PGPAccessValidator) ValidateAccess(ctx context.Context, publicKeyID, publicKeyIDSignatureBase64 string, clusterID resource.ID) error {
	singatureBytes, err := base64.StdEncoding.DecodeString(publicKeyIDSignatureBase64)
	if err != nil {
		return err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	publicKey, err := safe.StateGet[*authres.PublicKey](ctx, p.state, authres.NewPublicKey(resources.DefaultNamespace, publicKeyID).Metadata())
	if err != nil {
		return err
	}

	key, err := pgpcrypto.NewKeyFromArmored(string(publicKey.TypedSpec().Value.GetPublicKey()))
	if err != nil {
		return err
	}

	pgpKey, err := pgp.NewKey(key)
	if err != nil {
		return err
	}

	if err = pgpKey.Validate(); err != nil {
		return err
	}

	if err = pgpKey.Verify([]byte(publicKeyID), singatureBytes); err != nil {
		return err
	}

	publicKeyRoleStr := publicKey.TypedSpec().Value.GetRole()
	if publicKeyRoleStr != "" {
		publicKeyRole, parseErr := role.Parse(publicKeyRoleStr)
		if parseErr != nil {
			return parseErr
		}

		ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: publicKeyRole})
	}

	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: publicKey.TypedSpec().Value.GetIdentity().GetEmail()})

	accessRole, err := p.roleProvider.RoleForCluster(ctx, clusterID)
	if err != nil {
		return err
	}

	return accessRole.Check(role.Reader)
}
