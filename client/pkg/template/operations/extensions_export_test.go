// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

// TestExportTemplateEmptyExtensions verifies that an extensions configuration with an empty list is
// exported as an explicitly empty list, so a re-applied template keeps the extensions cleared instead
// of falling back to the machine's initially discovered set.
func TestExportTemplateEmptyExtensions(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	st := buildState(ctx, t)

	// clear the extensions of the cluster-level extensions configuration
	_, err := safe.StateUpdateWithConflicts(
		ctx, st, omni.NewExtensionsConfiguration("schematic-export-test").Metadata(),
		func(res *omni.ExtensionsConfiguration) error {
			res.TypedSpec().Value.Extensions = []string{}

			return nil
		},
	)
	require.NoError(t, err)

	var sb strings.Builder

	_, err = operations.ExportTemplate(ctx, st, "export-test", false, &sb)
	require.NoError(t, err)

	require.Contains(t, sb.String(), "systemExtensions: []")
}
