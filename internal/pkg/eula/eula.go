// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package eula provides helpers for managing EULA acceptance.
package eula

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

// StateGetter is the minimal interface for reading state, used by the interceptor.
type StateGetter interface {
	Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error)
}

// IsAccepted returns true if the EULA has been accepted.
func IsAccepted(ctx context.Context, st StateGetter) (bool, error) {
	_, err := st.Get(ctx, authres.NewEulaAcceptance().Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// AcceptParams holds the identity of the person accepting the EULA via CLI flags.
type AcceptParams struct {
	Name  string
	Email string
}

// Accept creates the EulaAcceptance resource if it does not already exist.
// This is used when Omni is started with --eula-accept-name/--eula-accept-email, where the accepting party
// is considered to have accepted the EULA externally (e.g., SaaS users).
func Accept(ctx context.Context, st state.State, params AcceptParams) error {
	existing, err := safe.StateGetByID[*authres.EulaAcceptance](ctx, st, authres.EulaAcceptanceID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if existing != nil {
		// Already accepted — nothing to do.
		return nil
	}

	res := authres.NewEulaAcceptance()
	res.TypedSpec().Value.AcceptedByName = params.Name
	res.TypedSpec().Value.AcceptedByEmail = params.Email

	return st.Create(ctx, res)
}
