// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omnictl"
)

func TestConfigContextOutput(t *testing.T) {
	t.Parallel()

	if os.Getenv("OMNI_TEST_CONFIG_CONTEXT_OUTPUT") == "1" {
		omnictl.RootCmd.SetArgs(os.Args[slices.Index(os.Args, "--")+1:])

		if err := omnictl.Execute(); err != nil {
			os.Exit(1)
		}

		os.Exit(0)
	}

	const contents = `context: default
contexts:
  default:
    url: https://dev.example.com
    auth:
      siderov1:
        identity: dev@example.com
  production:
    url: https://prod.example.com
    auth:
      siderov1:
        identity: prod@example.com
`

	for _, tt := range []struct {
		name    string
		command string
		context string
		want    string
		wantErr bool
	}{
		{
			name:    "saved context",
			command: "info",
			want:    "Current context: default\nURL:             https://dev.example.com\nIdentity:        dev@example.com\n",
		},
		{
			name:    "explicit saved context",
			command: "info",
			context: "default",
			want:    "Current context: default\nURL:             https://dev.example.com\nIdentity:        dev@example.com\n",
		},
		{
			name:    "context override",
			command: "info",
			context: "production",
			want:    "Current context: production\nURL:             https://prod.example.com\nIdentity:        prod@example.com\n",
		},
		{
			name:    "unknown context",
			command: "info",
			context: "missing",
			want:    "Error: context not found: missing\n",
			wantErr: true,
		},
		{
			name:    "contexts with saved context",
			command: "contexts",
			want:    "CURRENT   NAME         URL\n*         default      https://dev.example.com\n          production   https://prod.example.com\n",
		},
		{
			name:    "contexts with override",
			command: "contexts",
			context: "production",
			want:    "CURRENT   NAME         URL\n          default      https://dev.example.com\n*         production   https://prod.example.com\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "omniconfig")
			require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestConfigContextOutput$", "--",
				"--omniconfig", path, "--context", tt.context, "config", tt.command)

			cmd.Env = append(os.Environ(), "OMNI_TEST_CONFIG_CONTEXT_OUTPUT=1")

			output, err := cmd.CombinedOutput()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err, string(output))
			}

			require.Equal(t, tt.want, string(output))

			after, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, contents, string(after))
		})
	}
}

func TestConfigContextSelection(t *testing.T) {
	t.Parallel()

	if os.Getenv("OMNI_TEST_CONFIG_CONTEXT_SELECTION") == "1" {
		omnictl.RootCmd.SetArgs(os.Args[slices.Index(os.Args, "--")+1:])

		if err := omnictl.Execute(); err != nil {
			os.Exit(1)
		}

		os.Exit(0)
	}

	const contents = "context: default\ncontexts:\n  default:\n    url: https://dev.example.com\n  production:\n    url: https://prod.example.com\n"

	path := filepath.Join(t.TempDir(), "omniconfig")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestConfigContextSelection$", "--",
		"--omniconfig", path, "config", "context", "missing")

	cmd.Env = append(os.Environ(), "OMNI_TEST_CONFIG_CONTEXT_SELECTION=1")

	output, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Equal(t, "Error: context not found: missing\n", string(output))

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, contents, string(after))

	cmd = exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestConfigContextSelection$", "--",
		"--omniconfig", path, "config", "context", "production")

	cmd.Env = append(os.Environ(), "OMNI_TEST_CONFIG_CONTEXT_SELECTION=1")

	output, err = cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	after, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(after), "context: production\n")
}

func TestConfigNewPath(t *testing.T) {
	t.Parallel()

	if os.Getenv("OMNI_TEST_CONFIG_NEW_PATH") == "1" {
		omnictl.RootCmd.SetArgs(os.Args[slices.Index(os.Args, "--")+1:])

		if err := omnictl.Execute(); err != nil {
			os.Exit(1)
		}

		os.Exit(0)
	}

	const existing = "context: default\ncontexts:\n  default:\n    url: https://existing.example.org\n"

	files := []string{"environment.yaml", "flag.yaml", "positional.yaml"}

	for _, tt := range []struct {
		name string
		want string
		args []string
	}{
		{name: "omniconfig flag", args: []string{"--omniconfig", "flag.yaml"}, want: "flag.yaml"},
		{name: "positional path overrides flag", args: []string{"--omniconfig", "flag.yaml", "positional.yaml"}, want: "positional.yaml"},
		{name: "positional path overrides environment", args: []string{"positional.yaml"}, want: "positional.yaml"},
		{name: "environment fallback", want: "environment.yaml"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			for _, file := range files {
				if file != tt.want {
					require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(existing), 0o600))
				}
			}

			args := append([]string{"-test.run=^TestConfigNewPath$", "--", "config", "new"}, tt.args...)
			args = append(args, "--url", "https://new.example.org", "--identity", "dev@example.org")
			cmd := exec.CommandContext(t.Context(), os.Args[0], args...)

			cmd.Dir = dir

			cmd.Env = append(os.Environ(), "OMNI_TEST_CONFIG_NEW_PATH=1", "OMNICONFIG=environment.yaml")

			output, err := cmd.CombinedOutput()
			require.NoError(t, err, string(output))

			for _, file := range files {
				contents, err := os.ReadFile(filepath.Join(dir, file))
				require.NoError(t, err)

				if file == tt.want {
					require.Contains(t, string(contents), "url: https://new.example.org\n")
					require.Contains(t, string(contents), "identity: dev@example.org\n")
				} else {
					require.Equal(t, existing, string(contents))
				}
			}
		})
	}
}
