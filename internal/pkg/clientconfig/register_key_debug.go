// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build sidero.debug

package clientconfig

import (
	"context"
	"time"

	"github.com/siderolabs/go-api-signature/pkg/client/auth"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"google.golang.org/grpc/metadata"

	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
)

func registerKey(ctx context.Context, cli *auth.Client, key *pgp.Key, email string, opts ...auth.RegisterPGPPublicKeyOption) error {
	armoredPublicKey, err := key.ArmorPublic()
	if err != nil {
		return err
	}

	_, err = cli.RegisterPGPPublicKey(ctx, email, []byte(armoredPublicKey), opts...)
	if err != nil {
		return err
	}

	debugCtx := metadata.AppendToOutgoingContext(ctx, grpcomni.DebugVerifiedEmailHeaderKey, email)

	err = cli.ConfirmPublicKey(debugCtx, key.Fingerprint())
	if err != nil {
		return err
	}

	timeoutCtx, timeoutCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	defer timeoutCtxCancel()

	return cli.AwaitPublicKeyConfirmation(timeoutCtx, key.Fingerprint())
}
