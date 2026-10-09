// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl //nolint:testpackage

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func TestEditPreservesRecoveryFile(t *testing.T) {
	// The editor writes invalid YAML once, then exits without changes.
	if recordPath := os.Getenv("OMNI_TEST_EDIT_RECOVERY"); recordPath != "" {
		path := os.Args[len(os.Args)-1]

		paths, err := os.ReadFile(recordPath)
		if os.IsNotExist(err) {
			if os.Getenv("OMNI_TEST_EDIT_NO_CHANGES") != "1" {
				require.NoError(t, os.WriteFile(path, []byte("[\n"), 0o600))
			}
		} else {
			require.NoError(t, err)
		}

		require.NoError(t, os.WriteFile(recordPath, append(paths, []byte(path+"\n")...), 0o600))

		os.Exit(0)
	}

	recordPath := filepath.Join(t.TempDir(), "editor-paths")
	t.Setenv("OMNI_TEST_EDIT_RECOVERY", recordPath)
	t.Setenv("OMNI_EDITOR", strconv.Quote(os.Args[0])+" -test.run=TestEditPreservesRecoveryFile --")

	// Parsing fails before any API call.
	err := editFn(nil)(t.Context(), []resource.Resource{omni.NewConfigPatch("test-patch")}, nil)
	require.Error(t, err)

	paths, readErr := os.ReadFile(recordPath)
	require.NoError(t, readErr)

	editorPaths := strings.Split(strings.TrimSpace(string(paths)), "\n")
	for _, path := range editorPaths {
		t.Cleanup(func() {
			os.Remove(path) //nolint:errcheck // the editor may have already removed the file
		})
	}

	require.Len(t, editorPaths, 2)

	recoveryPath := editorPaths[1]
	require.Contains(t, err.Error(), "A copy of your changes has been stored to "+strconv.Quote(recoveryPath))

	contents, readErr := os.ReadFile(recoveryPath)
	require.NoError(t, readErr, "the advertised recovery file must survive edit cancellation")
	require.Contains(t, string(contents), "# Edit Failed:")
	require.Contains(t, string(contents), "[\n")
}

func TestEditRemovesUnchangedFile(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), "editor-paths")
	t.Setenv("OMNI_TEST_EDIT_RECOVERY", recordPath)
	t.Setenv("OMNI_TEST_EDIT_NO_CHANGES", "1")
	t.Setenv("OMNI_EDITOR", strconv.Quote(os.Args[0])+" -test.run=TestEditPreservesRecoveryFile --")

	require.NoError(t, editFn(nil)(t.Context(), []resource.Resource{omni.NewConfigPatch("test-patch")}, nil))

	paths, err := os.ReadFile(recordPath)
	require.NoError(t, err)

	editorPaths := strings.Split(strings.TrimSpace(string(paths)), "\n")
	for _, path := range editorPaths {
		t.Cleanup(func() {
			os.Remove(path) //nolint:errcheck // the editor may have already removed the file
		})
	}

	require.Len(t, editorPaths, 1)

	_, err = os.Stat(editorPaths[0])
	require.ErrorIs(t, err, os.ErrNotExist)
}
