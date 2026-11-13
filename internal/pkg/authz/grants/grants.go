// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package grants finds out who the caller of a request is and what it was granted.
package grants

import (
	"context"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/access"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/authz"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// AllowResource returns nil when the caller in the context can do the request to the resource API.
//
// The grants of a caller come from two places: the preset of the user, and the access policy. The preset is in the context,
// the access policy has to be read from the state. So the request is checked with the grants of the preset first,
// and the access policy is read only when those are not enough. Most requests never read it.
func AllowResource(ctx context.Context, st state.State, target authz.Target) error {
	subject := Subject(ctx)

	authzErr := authz.AllowResource(subject, target)
	if authzErr == nil {
		return nil
	}

	// the access policy is only for the users: it cannot help a request without a signature, or an infra provider
	if !subject.Authenticated || subject.Provider {
		return authzErr
	}

	policyGrants, err := accessPolicyGrants(ctx, st)
	if err != nil {
		return err
	}

	// the access policy grants nothing to this user, the first answer stands
	if len(policyGrants) == 0 {
		return authzErr
	}

	subject.Grants = append(subject.Grants, policyGrants...)

	return authz.AllowResource(subject, target)
}

// Subject returns the caller in the context, with the grants its preset stands for.
func Subject(ctx context.Context) authz.Subject {
	if actor.ContextIsInternalActor(ctx) {
		return authz.Subject{Internal: true}
	}

	roleVal, authenticated := ctxstore.Value[auth.RoleContextKey](ctx)
	if !authenticated {
		return authz.Subject{}
	}

	_, suspended := ctxstore.Value[auth.SuspendedContextKey](ctx)

	subject := authz.Subject{
		Authenticated: true,
		Suspended:     suspended,
		Grants:        authz.PresetGrants(roleVal.Role),
	}

	// An infra provider is recognized by its service account, and not by a role.
	// It gets no grants: what it can access is decided by the infra provider state and by the resource type table.
	if identity, ok := ctxstore.Value[auth.IdentityContextKey](ctx); ok {
		if sa, isServiceAccount := access.ParseServiceAccountFromFullID(identity.Identity); isServiceAccount && sa.IsInfraProvider {
			subject.Provider = true
			subject.Grants = nil
		}
	}

	return subject
}

// ClusterOfMachine returns the ID of the cluster the machine is a member of, empty when it is in no cluster.
func ClusterOfMachine(ctx context.Context, st state.State, machineID string) (string, error) {
	clusterMachine, err := safe.StateGetByID[*omni.ClusterMachine](actor.MarkContextAsInternalActor(ctx), st, machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil
		}

		return "", err
	}

	cluster, _ := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)

	return cluster, nil
}

// accessPolicyGrants returns what the access policy grants to the caller in the context.
func accessPolicyGrants(ctx context.Context, st state.State) ([]authz.Grant, error) {
	identityVal, ok := ctxstore.Value[auth.IdentityContextKey](ctx)
	if !ok {
		return nil, nil
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	policy, err := safe.StateGetByID[*authres.AccessPolicy](ctx, st, authres.AccessPolicyID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	identity, err := safe.StateGetByID[*authres.Identity](ctx, st, identityVal.Identity)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	return FromAccessPolicy(policy.TypedSpec().Value, identity.Metadata())
}
