// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package accesspolicy

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// RoleForCluster returns the role of the current user for the given cluster, and whether the role matches all clusters.
func RoleForCluster(ctx context.Context, id resource.ID, st state.State) (role.Role, bool, error) {
	userRole := role.None

	if val, ok := ctxstore.Value[auth.RoleContextKey](ctx); ok {
		userRole = val.Role
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	accessPolicy, err := safe.StateGet[*authres.AccessPolicy](ctx, st, authres.NewAccessPolicy().Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return userRole, false, nil
		}

		return role.None, false, err
	}

	identityVal, identityExists := ctxstore.Value[auth.IdentityContextKey](ctx)
	if !identityExists {
		return userRole, false, nil
	}

	identity, err := safe.StateGet[*authres.Identity](ctx, st, authres.NewIdentity(resources.DefaultNamespace, identityVal.Identity).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return userRole, false, nil
		}

		return role.None, false, err
	}

	clusterMD := omni.NewCluster(resources.DefaultNamespace, id).Metadata()

	checkResult, err := Check(accessPolicy, clusterMD, identity.Metadata())
	if err != nil {
		return role.None, false, err
	}

	maxRole, err := role.Max(userRole, checkResult.Role)
	if err != nil {
		return role.None, false, err
	}

	return maxRole, checkResult.MatchesAllClusters, nil
}
