// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
)

func ComputeDiff(previousData, newData []byte) (string, error) {
	if bytes.Equal(previousData, newData) {
		return "", nil
	}

	oldConfigString := string(previousData)
	newConfigString := string(newData)

	edits := myers.ComputeEdits("", oldConfigString, newConfigString)
	diff := gotextdiff.ToUnified("", "", oldConfigString, edits)

	return formatDiff(diff)
}

//nolint:gocognit
func formatDiff(u gotextdiff.Unified) (string, error) {
	if len(u.Hunks) == 0 {
		return "", nil
	}

	var sb strings.Builder

	for _, hunk := range u.Hunks {
		fromCount, toCount := 0, 0

		for _, l := range hunk.Lines {
			//nolint:exhaustive
			switch l.Kind {
			case gotextdiff.Delete:
				fromCount++
			case gotextdiff.Insert:
				toCount++
			default:
				fromCount++
				toCount++
			}
		}

		_, err := sb.WriteString("@@")
		if err != nil {
			return "", err
		}

		if fromCount > 1 {
			_, err = fmt.Fprintf(&sb, " -%d,%d", hunk.FromLine, fromCount)
		} else {
			_, err = fmt.Fprintf(&sb, " -%d", hunk.FromLine)
		}

		if err != nil {
			return "", err
		}

		if toCount > 1 {
			_, err = fmt.Fprintf(&sb, " +%d,%d", hunk.ToLine, toCount)
		} else {
			_, err = fmt.Fprintf(&sb, " +%d", hunk.ToLine)
		}

		if err != nil {
			return "", err
		}

		_, err = sb.WriteString(" @@\n")
		if err != nil {
			return "", err
		}

		for _, l := range hunk.Lines {
			//nolint:exhaustive
			switch l.Kind {
			case gotextdiff.Delete:
				_, err = fmt.Fprintf(&sb, "-%s", l.Content)
			case gotextdiff.Insert:
				_, err = fmt.Fprintf(&sb, "+%s", l.Content)
			default:
				_, err = fmt.Fprintf(&sb, " %s", l.Content)
			}

			if err != nil {
				return "", err
			}

			if !strings.HasSuffix(l.Content, "\n") {
				_, err := sb.WriteString("\n\\ No newline at end of file\n")
				if err != nil {
					return "", err
				}
			}
		}
	}

	return sb.String(), nil
}
