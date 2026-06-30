// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgaccess "github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/internal/pkg/auth"
)

const (
	// These limits must match the values in hack/test/templates/omni-config.yaml.
	limitMaxServiceAccounts = 5
	limitMaxUsers           = 5

	limitSAPrefix   = "e2e-limit-sa-"
	limitUserPrefix = "e2e-limit-user-"
)

// AssertServiceAccountLimits verifies that service account creation is blocked when the configured limit is reached.
func AssertServiceAccountLimits(testCtx context.Context, cli *client.Client) TestFunc {
	return func(t *testing.T) {
		// Clean up any leftover service accounts from previous runs.
		cleanupLimitServiceAccounts(testCtx, t, cli)

		var created []string

		defer func() {
			for _, name := range created {
				if err := cli.Management().DestroyServiceAccount(testCtx, name); err != nil {
					t.Logf("cleanup: failed to destroy service account %q: %v", name, err)
				}
			}
		}()

		for i := range limitMaxServiceAccounts {
			name := fmt.Sprintf("%s%d", limitSAPrefix, i)

			comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
			email := name + pkgaccess.ServiceAccountNameSuffix

			key, err := pgp.GenerateKey(name, comment, email, auth.ServiceAccountMaxAllowedLifetime)
			require.NoError(t, err)

			armoredPublicKey, err := key.ArmorPublic()
			require.NoError(t, err)

			_, err = cli.Management().CreateServiceAccount(testCtx, name, armoredPublicKey, string(role.Reader), false)
			if err != nil {
				assert.ErrorContains(t, err, "maximum number of service accounts")

				return
			}

			created = append(created, name)
		}

		// If we successfully created all limitMaxServiceAccounts, the next one must fail.
		overflowName := fmt.Sprintf("%s%s", limitSAPrefix, "overflow")
		comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
		email := overflowName + pkgaccess.ServiceAccountNameSuffix

		key, err := pgp.GenerateKey(overflowName, comment, email, auth.ServiceAccountMaxAllowedLifetime)
		require.NoError(t, err)

		armoredPublicKey, err := key.ArmorPublic()
		require.NoError(t, err)

		_, err = cli.Management().CreateServiceAccount(testCtx, overflowName, armoredPublicKey, string(role.Reader), false)
		require.Error(t, err, "expected service account creation to fail at the limit")
		assert.ErrorContains(t, err, "maximum number of service accounts")
	}
}

// AssertUserLimits verifies that user creation is blocked when the configured limit is reached.
func AssertUserLimits(testCtx context.Context, cli *client.Client) TestFunc {
	return func(t *testing.T) {
		// Clean up any leftover test users from previous runs.
		cleanupLimitUsers(testCtx, t, cli)

		var created []string

		defer func() {
			for _, userEmail := range created {
				if err := cli.Management().DestroyUser(testCtx, userEmail); err != nil {
					t.Logf("cleanup: failed to delete user %q: %v", userEmail, err)
				}
			}
		}()

		for i := range limitMaxUsers {
			userEmail := fmt.Sprintf("%s%d@test.com", limitUserPrefix, i)

			_, err := cli.Management().CreateUser(testCtx, userEmail, string(role.Reader))
			if err != nil {
				assert.ErrorContains(t, err, "maximum number of users")

				return
			}

			created = append(created, userEmail)
		}

		// If we successfully created all limitMaxUsers, the next one must fail.
		overflowEmail := fmt.Sprintf("%soverflow@test.com", limitUserPrefix)

		_, err := cli.Management().CreateUser(testCtx, overflowEmail, string(role.Reader))
		require.Error(t, err, "expected user creation to fail at the limit")
		assert.ErrorContains(t, err, "maximum number of users")
	}
}

func cleanupLimitServiceAccounts(ctx context.Context, t *testing.T, cli *client.Client) {
	t.Helper()

	saList, err := cli.Management().ListServiceAccounts(ctx)
	require.NoError(t, err)

	for _, sa := range saList {
		if strings.HasPrefix(sa.Name, limitSAPrefix) {
			if err := cli.Management().DestroyServiceAccount(ctx, sa.Name); err != nil {
				t.Logf("cleanup: failed to destroy leftover SA %q: %v", sa.Name, err)
			}
		}
	}
}

func cleanupLimitUsers(ctx context.Context, t *testing.T, cli *client.Client) {
	t.Helper()

	users, err := cli.Management().ListUsers(ctx)
	if err != nil {
		t.Logf("cleanup: failed to list users: %v", err)

		return
	}

	for _, u := range users {
		if strings.HasPrefix(u.Email, limitUserPrefix) {
			if err := cli.Management().DestroyUser(ctx, u.Email); err != nil {
				t.Logf("cleanup: failed to delete leftover user %q: %v", u.Email, err)
			}
		}
	}
}
