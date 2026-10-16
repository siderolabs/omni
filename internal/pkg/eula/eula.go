// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package eula manages the End User License Agreement acceptance.
package eula

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

// URL is the address of the End User License Agreement.
const URL = "https://siderolabs.com/eula/"

// EnsureAccepted creates or updates the EulaAcceptance resource with the given name and email, if both are set.
// It returns an error if the EULA was never accepted.
func EnsureAccepted(ctx context.Context, st state.State, logger *zap.Logger, name, email string) error {
	existing, err := safe.StateGetByID[*authres.EulaAcceptance](ctx, st, authres.EulaAcceptanceID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if name == "" || email == "" {
		if existing == nil {
			return fmt.Errorf("the End User License Agreement (%s) is not accepted, set the --eula-accept-name and --eula-accept-email flags (eulaAccept in the config) to accept it", URL)
		}

		return nil
	}

	if existing != nil && existing.TypedSpec().Value.AcceptedByName == name && existing.TypedSpec().Value.AcceptedByEmail == email {
		return nil
	}

	if err = safe.StateModify(ctx, st, authres.NewEulaAcceptance(), func(res *authres.EulaAcceptance) error {
		res.TypedSpec().Value.AcceptedByName = name
		res.TypedSpec().Value.AcceptedByEmail = email

		return nil
	}); err != nil {
		return err
	}

	logger.Info("End User License Agreement accepted", zap.String("url", URL), zap.String("name", name), zap.String("email", email))

	return nil
}
