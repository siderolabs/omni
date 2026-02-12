// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package diff provides a memory-safe unified diff computation.
package diff

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/neticdk/go-stdlib/diff/myers"
)

// MaxLines is the maximum total number of lines (old + new) for which a full
// structural diff is computed. Beyond this the diff is summarized because no
// human can meaningfully review it and the algorithmic cost becomes prohibitive.
const MaxLines = 75_000

// Compute returns a unified diff (without the --- / +++ header) between two
// byte slices. For inputs whose combined line count exceeds MaxLines it returns
// a short summary instead.
func Compute(previousData, newData []byte) (string, error) {
	if bytes.Equal(previousData, newData) {
		return "", nil
	}

	prevLines := bytes.Count(previousData, []byte("\n"))
	newLines := bytes.Count(newData, []byte("\n"))

	if prevLines+newLines > MaxLines {
		return fmt.Sprintf("@@ -%d,%d +%d,%d @@ diff too large to display\n", 1, prevLines, 1, newLines), nil
	}

	result, err := myers.Diff(string(previousData), string(newData),
		myers.WithUnifiedFormatter(),
		myers.WithLinearSpace(true),
		// Disable the library's standard-Myers and LCS fallback paths:
		// - Standard Myers (< smallInputThreshold) is O((N+M)²) when inputs are asymmetric.
		// - LCS (> largeInputThreshold) is O(N*M) for the DP table.
		// By setting these to 0 and MaxLines respectively, only Hirschberg's
		// O(N+M) linear-space algorithm runs. Our MaxLines guard above ensures
		// inputs never exceed largeInputThreshold.
		myers.WithSmallInputThreshold(0),
		myers.WithLargeInputThreshold(MaxLines),
	)
	if err != nil {
		return "", err
	}

	// Strip the "--- a\n+++ b\n" header that the library always prepends.
	if after, ok := strings.CutPrefix(result, "--- a\n+++ b\n"); ok {
		result = after
	}

	return result, nil
}
