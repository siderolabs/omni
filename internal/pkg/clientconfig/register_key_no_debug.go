// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build !sidero.debug

package clientconfig

import (
	"context"
	"errors"

	"github.com/siderolabs/go-api-signature/pkg/client/auth"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
)

func registerKey(context.Context, *auth.Client, *pgp.Key, string, ...auth.RegisterPGPPublicKeyOption) error {
	return errors.New("registerKey is not implemented in non-debug builds")
}
