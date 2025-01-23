// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package serviceaccount defines common code for creating a service account.
package serviceaccount

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// Create a service account.
func Create(ctx context.Context, st state.State, name, userRole string, useUserRole bool, armoredPGPPublicKey []byte) (string, error) {
	sa := access.ParseServiceAccountFromName(name)
	saRole := role.Admin

	if useUserRole && sa.IsInfraProvider {
		return "", fmt.Errorf("infra provider service accounts must have the role %q, but use-user-role was requested", role.InfraProvider)
	}

	if !useUserRole {
		var err error

		if saRole, err = role.Parse(userRole); err != nil {
			return "", err
		}

		if sa.IsInfraProvider && saRole != role.InfraProvider {
			return "", fmt.Errorf("infra-provider service accounts must have the role %q", role.InfraProvider)
		}

		if saRole == role.InfraProvider && !sa.IsInfraProvider {
			return "", fmt.Errorf("service accounts with role %q must be prefixed with %q", role.InfraProvider, access.InfraProviderServiceAccountPrefix)
		}
	}

	id := sa.FullID()

	ctx = actor.MarkContextAsInternalActor(ctx)

	_, err := st.Get(ctx, authres.NewIdentity(resources.DefaultNamespace, id).Metadata())
	if err == nil {
		return "", fmt.Errorf("service account %q already exists", id)
	}

	if !state.IsNotFoundError(err) { // the identity must not exist
		return "", err
	}

	key, err := access.ValidatePGPPublicKey(
		armoredPGPPublicKey,
		pgp.WithMaxAllowedLifetime(auth.ServiceAccountMaxAllowedLifetime),
	)
	if err != nil {
		return "", err
	}

	newUserID := uuid.NewString()

	publicKeyResource := authres.NewPublicKey(resources.DefaultNamespace, key.ID)
	publicKeyResource.Metadata().Labels().Set(authres.LabelPublicKeyUserID, newUserID)

	if sa.IsInfraProvider {
		publicKeyResource.Metadata().Labels().Set(authres.LabelInfraProvider, "")
	}

	publicKeyResource.TypedSpec().Value.PublicKey = key.Data
	publicKeyResource.TypedSpec().Value.Expiration = timestamppb.New(key.Expiration)
	publicKeyResource.TypedSpec().Value.Role = string(saRole)

	// register the public key of the service account as "confirmed" because we are already authenticated
	publicKeyResource.TypedSpec().Value.Confirmed = true

	publicKeyResource.TypedSpec().Value.Identity = &specs.Identity{
		Email: id,
	}

	err = st.Create(ctx, publicKeyResource)
	if err != nil {
		return "", err
	}

	// create the user resource representing the service account with the same scopes as the public key
	user := authres.NewUser(resources.DefaultNamespace, newUserID)
	user.TypedSpec().Value.Role = publicKeyResource.TypedSpec().Value.GetRole()

	if sa.IsInfraProvider {
		user.Metadata().Labels().Set(authres.LabelInfraProvider, "")
	}

	err = st.Create(ctx, user)
	if err != nil {
		return "", err
	}

	// create the identity resource representing the service account
	identity := authres.NewIdentity(resources.DefaultNamespace, id)
	identity.TypedSpec().Value.UserId = user.Metadata().ID()
	identity.Metadata().Labels().Set(authres.LabelIdentityUserID, newUserID)
	identity.Metadata().Labels().Set(authres.LabelIdentityTypeServiceAccount, "")

	if sa.IsInfraProvider {
		identity.Metadata().Labels().Set(authres.LabelInfraProvider, "")
	}

	err = st.Create(ctx, identity)
	if err != nil {
		return "", err
	}

	return key.ID, nil
}
