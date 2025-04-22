// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/require"

	pkgaccess "github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// AssertDownloadUsingCLI verifies generated image download using omnictl.
func AssertDownloadUsingCLI(testCtx context.Context, client *client.Client, omnictlPath, httpEndpoint string) TestFunc {
	st := client.Omni().State()

	return func(t *testing.T) {
		t.Parallel()

		if omnictlPath == "" {
			t.Skip()
		}

		media, err := safe.StateListAll[*omni.InstallationMedia](testCtx, st)
		require.NoError(t, err)

		var images []*omni.InstallationMedia

		for val := range media.All() {
			spec := val.TypedSpec().Value

			switch spec.Profile {
			case constants.BoardRPiGeneric:
				fallthrough
			case "aws":
				fallthrough
			case "iso":
				images = append(images, val)
			}
		}

		require.Greater(t, len(images), 2)

		name := "test-" + uuid.NewString()

		key := createServiceAccount(testCtx, t, client, name)

		for _, image := range images {
			t.Run(image.Metadata().ID(), func(t *testing.T) {
				t.Parallel()

				output := filepath.Join(t.TempDir(), image.Metadata().ID())

				stdout, stderr, err := runCmd(
					omnictlPath,
					httpEndpoint,
					key, "download",
					"--insecure-skip-tls-verify",
					image.TypedSpec().Value.Name,
					"--initial-labels",
					"key=value",
					"--output",
					output)
				require.NoError(t, err, "stdout:\n %s\nstderr:\n%s", stdout.String(), stderr.String())

				res, err := os.Stat(output)

				require.NoError(t, err)

				require.Greater(t, res.Size(), int64(1024*1024))
			})
		}
	}
}

func runCmd(path, endpoint, key string, args ...string) (bytes.Buffer, bytes.Buffer, error) {
	var stdout, stderr bytes.Buffer

	args = append([]string{"--insecure-skip-tls-verify"}, args...)

	cmd := exec.Command(
		path,
		args...,
	)

	cmd.Stdin = nil
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = []string{
		fmt.Sprintf("OMNI_ENDPOINT=%s", endpoint),
		fmt.Sprintf("OMNI_SERVICE_ACCOUNT_KEY=%s", key),
	}

	if err := cmd.Start(); err != nil {
		return stdout, stderr, err
	}

	err := cmd.Wait()

	return stdout, stderr, err
}

func createServiceAccount(ctx context.Context, t *testing.T, client *client.Client, name string) string {
	// generate a new PGP key with long lifetime
	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	serviceAccountEmail := name + pkgaccess.ServiceAccountNameSuffix

	key, err := pgp.GenerateKey(name, comment, serviceAccountEmail, auth.ServiceAccountMaxAllowedLifetime)
	require.NoError(t, err)

	armoredPublicKey, err := key.ArmorPublic()

	require.NoError(t, err)

	// create service account with the generated key
	_, err = client.Management().CreateServiceAccount(ctx, name, armoredPublicKey, string(role.Admin), true)
	require.NoError(t, err)

	encodedKey, err := serviceaccount.Encode(name, key)
	require.NoError(t, err)

	return encodedKey
}

// AssertUserCLI verifies user management cli commands.
func AssertUserCLI(testCtx context.Context, client *client.Client, omnictlPath, httpEndpoint string) TestFunc {
	return func(t *testing.T) {
		name := "test-" + uuid.NewString()

		key := createServiceAccount(testCtx, t, client, name)

		stdout, stderr, err := runCmd(omnictlPath, httpEndpoint, key, "user", "create", "a@a.com", "--role", "Admin")
		require.NoErrorf(t, err, "failed to create user. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		stdout, stderr, err = runCmd(omnictlPath, httpEndpoint, key, "user", "list")
		require.NoErrorf(t, err, "failed to list users. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		require.Contains(t, stdout.String(), "a@a.com")

		stdout, stderr, err = runCmd(omnictlPath, httpEndpoint, key, "user", "set-role", "--role", "Reader", "a@a.com")
		require.NoErrorf(t, err, "failed to set role. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		stdout, stderr, err = runCmd(omnictlPath, httpEndpoint, key, "user", "delete", "a@a.com")
		require.NoErrorf(t, err, "failed to delete user. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		stdout, stderr, err = runCmd(omnictlPath, httpEndpoint, key, "user", "list")
		require.NoErrorf(t, err, "failed to list users. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		require.NotContains(t, stdout.String(), "a@a.com")
	}
}
