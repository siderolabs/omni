// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

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
	"github.com/stretchr/testify/require"

	pkgaccess "github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/client"
	clientconstants "github.com/siderolabs/omni/client/pkg/constants"
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

			switch {
			case spec.Profile == "aws",
				spec.Extension == "iso" && spec.Overlay == "",
				spec.Overlay == "rpi_generic":
				images = append(images, val)
			}
		}

		require.Greater(t, len(images), 2)

		name := "test-" + uuid.NewString()

		key := createServiceAccount(testCtx, t, client, name, role.Admin)

		for _, image := range images {
			t.Run(image.Metadata().ID(), func(t *testing.T) {
				t.Parallel()

				output := filepath.Join(t.TempDir(), image.Metadata().ID())

				stdout, stderr, err := runCmd(
					testCtx,
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

func runCmd(ctx context.Context, path, endpoint, key string, args ...string) (bytes.Buffer, bytes.Buffer, error) {
	var stdout, stderr bytes.Buffer

	args = append([]string{"--insecure-skip-tls-verify"}, args...)

	cmd := exec.CommandContext(
		ctx,
		path,
		args...,
	)

	cmd.Stdin = nil
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	tempHomeDir, err := os.MkdirTemp("", "cli-test")
	if err != nil {
		return stdout, stderr, fmt.Errorf("failed to create temp home dir: %w", err)
	}
	defer os.RemoveAll(tempHomeDir) //nolint:errcheck

	cmd.Env = []string{
		fmt.Sprintf("HOME=%s", tempHomeDir),
		fmt.Sprintf("OMNI_ENDPOINT=%s", endpoint),
		fmt.Sprintf("OMNI_SERVICE_ACCOUNT_KEY=%s", key),
	}

	err = cmd.Start()
	if err != nil {
		return stdout, stderr, err
	}

	err = cmd.Wait()

	return stdout, stderr, err
}

func createServiceAccount(ctx context.Context, t *testing.T, client *client.Client, name string, role role.Role) string {
	// generate a new PGP key with long lifetime
	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	serviceAccountEmail := name + pkgaccess.ServiceAccountNameSuffix

	key, err := pgp.GenerateKey(name, comment, serviceAccountEmail, auth.ServiceAccountMaxAllowedLifetime)
	require.NoError(t, err)

	armoredPublicKey, err := key.ArmorPublic()

	require.NoError(t, err)

	// create service account with the generated key
	_, err = client.Management().CreateServiceAccount(ctx, name, armoredPublicKey, string(role), false)
	require.NoError(t, err)

	encodedKey, err := serviceaccount.Encode(name, key)
	require.NoError(t, err)

	return encodedKey
}

// AssertInstallationMediaPresetCLI verifies the `omnictl media` CLI: preset create/list/delete and download from a preset.
func AssertInstallationMediaPresetCLI(testCtx context.Context, client *client.Client, omnictlPath, httpEndpoint string) TestFunc {
	return func(t *testing.T) {
		t.Parallel()

		if omnictlPath == "" {
			t.Skip()
		}

		presetName := "test-preset-" + uuid.NewString()
		name := "test-" + uuid.NewString()

		key := createServiceAccount(testCtx, t, client, name, role.Admin)

		// Create a preset
		stdout, stderr, err := runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "create", presetName,
			"--arch", "amd64",
			"--talos-version", clientconstants.DefaultTalosVersion,
		)
		require.NoErrorf(t, err, "failed to create preset. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		// List presets and verify ours exists
		stdout, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "list",
		)
		require.NoErrorf(t, err, "failed to list presets. stdout: %q | stderr: %q", stdout.String(), stderr.String())
		require.Contains(t, stdout.String(), presetName)

		// Wide list output should include extra columns
		stdout, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "list", "-o", "wide",
		)
		require.NoErrorf(t, err, "failed to list presets wide. stdout: %q | stderr: %q", stdout.String(), stderr.String())
		require.Contains(t, stdout.String(), "PLATFORM/OVERLAY")
		require.Contains(t, stdout.String(), "KERNEL ARGS")

		// Download from preset
		output := filepath.Join(t.TempDir(), "preset-download.iso")

		stdout, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "download", presetName,
			"--format", "iso",
			"--output", output,
		)
		require.NoErrorf(t, err, "failed to download from preset. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		res, err := os.Stat(output)
		require.NoError(t, err)
		require.Greater(t, res.Size(), int64(1024*1024))

		// Delete the preset
		stdout, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "delete", presetName,
		)
		require.NoErrorf(t, err, "failed to delete preset. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		// Verify deletion
		stdout, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "list",
		)
		require.NoErrorf(t, err, "failed to list presets after deletion. stdout: %q | stderr: %q", stdout.String(), stderr.String())
		require.NotContains(t, stdout.String(), presetName)

		// --platform and --overlay are mutually exclusive
		_, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "create", "test-mutex-"+uuid.NewString(),
			"--arch", "amd64",
			"--platform", "aws",
			"--overlay", "rpi_generic",
		)
		require.Error(t, err, "create with --platform and --overlay should fail")
		require.Contains(t, stderr.String(), "cannot be used together")

		// Deleting a non-existent preset returns an error
		_, _, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "delete", "does-not-exist-"+uuid.NewString(),
		)
		require.Error(t, err, "deleting a non-existent preset should fail")

		// Cloud preset on AWS rejects secure boot (AWS does not support it)
		_, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "create", "test-aws-sb-"+uuid.NewString(),
			"--arch", "amd64",
			"--platform", "aws",
			"--secureboot",
		)
		require.Error(t, err, "AWS preset with --secureboot should be rejected")
		require.Contains(t, stderr.String(), "does not support secure boot")

		// Unknown Talos version is rejected up front
		_, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "create", "test-bad-version-"+uuid.NewString(),
			"--arch", "amd64",
			"--talos-version", "9.99.99",
		)
		require.Error(t, err, "create with unknown Talos version should be rejected")
		require.Contains(t, stderr.String(), `unknown Talos version "9.99.99"`)

		// Preset created without --talos-version uses the CLI default
		defaultsPreset := "test-defaults-" + uuid.NewString()

		stdout, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "preset", "create", defaultsPreset, "--arch", "amd64",
		)
		require.NoErrorf(t, err, "failed to create defaults preset. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		t.Cleanup(func() {
			//nolint:errcheck
			runCmd(testCtx, omnictlPath, httpEndpoint, key, "media", "preset", "delete", defaultsPreset)
		})

		// Download with --arch and --extensions overrides
		archOverrideOutput := filepath.Join(t.TempDir(), "arch-override.iso")

		stdout, stderr, err = runCmd(
			testCtx,
			omnictlPath,
			httpEndpoint,
			key,
			"media", "download", defaultsPreset,
			"--format", "iso",
			"--arch", "arm64",
			"--extensions", "qemu-guest-agent",
			"--extensions", "intel-ucode",
			"--output", archOverrideOutput,
		)
		require.NoErrorf(t, err, "download with overrides failed. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		res, err = os.Stat(archOverrideOutput)
		require.NoError(t, err)
		require.Greater(t, res.Size(), int64(1024*1024))
	}
}

// AssertUserCLI verifies user management cli commands.
func AssertUserCLI(testCtx context.Context, client *client.Client, omnictlPath, httpEndpoint string) TestFunc {
	return func(t *testing.T) {
		name := "test-" + uuid.NewString()

		key := createServiceAccount(testCtx, t, client, name, role.Admin)

		stdout, stderr, err := runCmd(testCtx, omnictlPath, httpEndpoint, key, "user", "create", "a@a.com", "--role", "Admin")
		require.NoErrorf(t, err, "failed to create user. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		stdout, stderr, err = runCmd(testCtx, omnictlPath, httpEndpoint, key, "user", "list")
		require.NoErrorf(t, err, "failed to list users. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		require.Contains(t, stdout.String(), "a@a.com")

		stdout, stderr, err = runCmd(testCtx, omnictlPath, httpEndpoint, key, "user", "set-role", "--role", "Reader", "a@a.com")
		require.NoErrorf(t, err, "failed to set role. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		stdout, stderr, err = runCmd(testCtx, omnictlPath, httpEndpoint, key, "user", "delete", "a@a.com")
		require.NoErrorf(t, err, "failed to delete user. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		stdout, stderr, err = runCmd(testCtx, omnictlPath, httpEndpoint, key, "user", "list")
		require.NoErrorf(t, err, "failed to list users. stdout: %q | stderr: %q", stdout.String(), stderr.String())

		require.NotContains(t, stdout.String(), "a@a.com")
	}
}
