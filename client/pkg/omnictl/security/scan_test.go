// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package security

import (
	"bytes"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/require"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
)

// TestWriteScanRow guards against passing a tab-joined string as a single safeout.Fprintf
// argument: safeout.Cell escapes tabs in any string value it's handed (correctly, since that
// value might be untrusted API data), so joining the severity counts into one string before
// calling safeout.Fprintf turns the columns into literal "\x09" text instead of aligning them.
func TestWriteScanRow(t *testing.T) {
	var buf bytes.Buffer

	tw := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)

	writeScanRow(tw, target{
		schematicID:  "abc123",
		arch:         imagefactorypb.Arch_AMD64,
		role:         "worker",
		machineCount: 3,
	}, "1.13.7", &severityCounts{Critical: 4, High: 50, Medium: 44, Low: 2, Other: 2})
	require.NoError(t, tw.Flush())

	require.NotContains(t, buf.String(), `\x09`)
	require.Equal(t, "abc123 amd64 1.13.7 worker 3 4 50 44 2 2\n", buf.String())
}

// TestWriteScanRowMissingReport covers the "-" placeholder path: a nil counts is a row the
// factory has no report for, not a scan that came back clean.
func TestWriteScanRowMissingReport(t *testing.T) {
	var buf bytes.Buffer

	tw := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)

	writeScanRow(tw, target{schematicID: "abc123", arch: imagefactorypb.Arch_ARM64}, "1.13.7", nil)
	require.NoError(t, tw.Flush())

	require.Equal(t, "abc123 arm64 1.13.7 - - - - - - -\n", buf.String())
}
