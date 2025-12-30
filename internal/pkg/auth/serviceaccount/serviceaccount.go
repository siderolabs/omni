// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package serviceaccount defines common code for creating a service account.
package serviceaccount

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
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

	ptr := authres.NewIdentity(resources.DefaultNamespace, id).Metadata()

	_, err := st.Get(ctx, ptr)
	if err == nil {
		return "", &eConflict{
			error: fmt.Errorf("service account %q already exists", id),
			res:   ptr,
		}
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

	publicKeyResource := authres.NewPublicKey(key.ID)
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

// Destroy service account.
func Destroy(ctx context.Context, st state.State, name string) error {
	sa := access.ParseServiceAccountFromName(name)
	id := sa.FullID()

	identity, err := safe.StateGet[*authres.Identity](ctx, st, authres.NewIdentity(resources.DefaultNamespace, id).Metadata())
	if err != nil {
		return err
	}

	_, isServiceAccount := identity.Metadata().Labels().Get(authres.LabelIdentityTypeServiceAccount)
	if !isServiceAccount {
		return &eNotFound{}
	}

	pubKeys, err := st.List(
		ctx,
		authres.NewPublicKey("").Metadata(),
		state.WithLabelQuery(resource.LabelEqual(authres.LabelIdentityUserID, identity.TypedSpec().Value.UserId)),
	)
	if err != nil {
		return err
	}

	var destroyErr error

	for _, pubKey := range pubKeys.Items {
		err = st.TeardownAndDestroy(ctx, pubKey.Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			destroyErr = multierror.Append(destroyErr, err)
		}
	}

	err = st.TeardownAndDestroy(ctx, identity.Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		destroyErr = multierror.Append(destroyErr, err)
	}

	err = st.TeardownAndDestroy(ctx, authres.NewUser(resources.DefaultNamespace, identity.TypedSpec().Value.UserId).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		destroyErr = multierror.Append(destroyErr, err)
	}

	return destroyErr
}
