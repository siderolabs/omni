// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl //nolint:testpackage

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	stateserver "github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

func TestEditUpdateConflict(t *testing.T) {
	if endpoint := os.Getenv("OMNI_TEST_EDIT_CONFLICT_ENDPOINT"); endpoint != "" {
		runConflictEditor(t, endpoint)
	}

	patchData := []byte("machine:\n  kubelet:\n    extraArgs:\n      max-pods: \"120\"\n")
	require.NoError(t, omni.ValidateConfigPatch(patchData))

	for _, mode := range []string{"single", "partial-unchanged", "partial-edited", "partial-recovered"} {
		t.Run(mode, func(t *testing.T) {
			st := state.WrapCore(inmem.NewState())
			patch := omni.NewConfigPatch("conflict")
			require.NoError(t, patch.TypedSpec().Value.SetUncompressedData(patchData))
			require.NoError(t, st.Create(t.Context(), patch))

			if mode != "single" {
				success := omni.NewConfigPatch("a-success")
				require.NoError(t, success.TypedSpec().Value.SetUncompressedData(patchData))
				require.NoError(t, st.Create(t.Context(), success))
			}

			rd, err := meta.NewResourceDefinition(omni.ConfigPatchExtension{}.ResourceDefinition())
			require.NoError(t, err)
			require.NoError(t, st.Create(t.Context(), rd))

			version := system.NewSysVersion(system.SysVersionID)
			version.TypedSpec().Value.BackendVersion = "1.13.3"
			require.NoError(t, st.Create(t.Context(), version))

			var listenConfig net.ListenConfig

			listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)

			server := grpc.NewServer()
			t.Cleanup(server.Stop)

			v1alpha1.RegisterStateServer(server, stateserver.NewState(st))
			go func() { assert.NoError(t, server.Serve(listener)) }()

			endpoint := "grpc://" + listener.Addr().String()
			recordPath := filepath.Join(t.TempDir(), "editor-paths")
			t.Setenv("OMNI_TEST_EDIT_CONFLICT_MODE", mode)
			t.Setenv("OMNI_TEST_EDIT_CONFLICT_ENDPOINT", endpoint)
			t.Setenv("OMNI_TEST_EDIT_CONFLICT_PATHS", recordPath)
			t.Setenv("OMNI_EDITOR", strconv.Quote(os.Args[0])+" -test.run=TestEditUpdateConflict --")

			configPath := filepath.Join(t.TempDir(), "omniconfig")
			require.NoError(t, os.WriteFile(configPath, []byte("context: test\ncontexts:\n  test:\n    url: "+endpoint+"\n"), 0o600))

			for _, name := range []string{"OMNI_ENDPOINT", "OMNI_SERVICE_ACCOUNT_KEY", "SIDERO_SERVICE_ACCOUNT_KEY"} {
				t.Setenv(name, "")
				require.NoError(t, os.Unsetenv(name))
			}

			previousOmniconfig := access.CmdFlags.Omniconfig

			t.Cleanup(func() {
				RootCmd.SetArgs(nil)

				access.CmdFlags.Omniconfig = previousOmniconfig
			})

			args := []string{"--omniconfig", configPath, "edit", "configpatch"}
			if mode == "single" {
				args = append(args, "conflict")
			}

			RootCmd.SetArgs(args)

			err = Execute()

			paths, readErr := os.ReadFile(recordPath)
			require.NoError(t, readErr)

			editorPaths := strings.Split(strings.TrimSpace(string(paths)), "\n")
			for _, path := range editorPaths {
				t.Cleanup(func() { os.Remove(path) }) //nolint:errcheck
			}

			stored, getErr := st.Get(t.Context(), patch.Metadata())
			require.NoError(t, getErr)

			value, exists := stored.Metadata().Annotations().Get("concurrent-edit")
			require.True(t, exists)
			require.Equal(t, "keep", value)

			_, exists = stored.Metadata().Annotations().Get("operator-edit")
			require.False(t, exists, "the conflicting update must not overwrite the concurrent edit")

			if mode != "single" {
				succeeded, successErr := st.Get(t.Context(), omni.NewConfigPatch("a-success").Metadata())
				require.NoError(t, successErr)

				expectedValue, expectedVersion := "first", "2"
				if mode == "partial-edited" || mode == "partial-recovered" {
					expectedValue, expectedVersion = "second", "3"
				}

				value, exists := succeeded.Metadata().Annotations().Get("operator-edit")
				require.True(t, exists)
				require.Equal(t, expectedValue, value)
				require.Equal(t, expectedVersion, succeeded.Metadata().Version().String(), "only changed documents should be updated on retry")
			}

			if mode == "partial-recovered" {
				require.NoError(t, err)
				require.Len(t, editorPaths, 2)

				_, statErr := os.Stat(editorPaths[1])
				require.ErrorIs(t, statErr, os.ErrNotExist)

				return
			}

			require.Error(t, err, "an API update failure must not be reported as success")
			require.Contains(t, err.Error(), "conflict")

			expectedRuns, expectedEdit := 2, "first"
			if mode != "single" {
				expectedRuns, expectedEdit = 3, "second"
			}

			require.Len(t, editorPaths, expectedRuns, "reopen the editor with the API error")
			recoveryPath := editorPaths[len(editorPaths)-1]
			require.Contains(t, err.Error(), strconv.Quote(recoveryPath))
			contents, readErr := os.ReadFile(recoveryPath)
			require.NoError(t, readErr)
			require.Contains(t, string(contents), "operator-edit: "+expectedEdit)
			require.Contains(t, string(contents), "# Edit Failed:")
		})
	}
}

func runConflictEditor(t *testing.T, endpoint string) {
	t.Helper()

	path := os.Args[len(os.Args)-1]
	recordPath := os.Getenv("OMNI_TEST_EDIT_CONFLICT_PATHS")
	paths, err := os.ReadFile(recordPath)
	first := os.IsNotExist(err)
	mode := os.Getenv("OMNI_TEST_EDIT_CONFLICT_MODE")

	retry := err == nil && mode != "single" && strings.Count(string(paths), "\n") == 1
	if first || retry {
		c, clientErr := client.New(endpoint)
		require.NoError(t, clientErr)

		st := c.Omni().State()
		if first {
			concurrent, getErr := st.Get(t.Context(), omni.NewConfigPatch("conflict").Metadata())
			require.NoError(t, getErr)
			concurrent.Metadata().Annotations().Set("concurrent-edit", "keep")
			require.NoError(t, st.Update(t.Context(), concurrent))
		}

		contents, readErr := os.ReadFile(path)
		require.NoError(t, readErr)

		resources, decodeErr := decodeEditedResources(contents)
		require.NoError(t, decodeErr)

		var edited []byte

		for _, res := range resources {
			switch {
			case first:
				res.Metadata().Annotations().Set("operator-edit", "first")
			case mode == "partial-recovered" && res.Metadata().ID() == "conflict":
				res.Metadata().Annotations().Delete("operator-edit")
			case res.Metadata().ID() == "conflict" || mode == "partial-edited" || mode == "partial-recovered":
				res.Metadata().Annotations().Set("operator-edit", "second")
			}

			data, marshalErr := marshalResourceCanonical(res)
			require.NoError(t, marshalErr)

			if len(edited) > 0 {
				edited = append(edited, []byte("---\n")...)
			}

			edited = append(edited, data...)
		}

		require.NoError(t, os.WriteFile(path, edited, 0o600))
		require.NoError(t, c.Close())
	} else {
		require.NoError(t, err)
	}

	require.NoError(t, os.WriteFile(recordPath, append(paths, []byte(path+"\n")...), 0o600))
	os.Exit(0)
}
