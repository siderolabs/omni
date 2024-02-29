// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package utils contains various utility functions for template operations.
package utils

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/fatih/color"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"gopkg.in/yaml.v3"
)

// MarshalResource to YAML format (bytes).
func MarshalResource(r resource.Resource) ([]byte, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	m, err := resource.MarshalYAML(r)
	if err != nil {
		return nil, err
	}

	if err := enc.Encode(m); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// RenderDiff outputs colorized diff between two resources.
//
// One of the resources might be nil.
func RenderDiff(w io.Writer, oldR, newR resource.Resource) error {
	var (
		oldYaml, newYaml []byte
		oldPath, newPath string
		err              error
	)

	if oldR != nil {
		oldYaml, err = MarshalResource(oldR)
		if err != nil {
			return err
		}

		oldPath = resource.String(oldR)
	} else {
		oldPath = "/dev/null"
	}

	if newR != nil {
		newYaml, err = MarshalResource(newR)
		if err != nil {
			return err
		}

		newPath = resource.String(newR)
	} else {
		newPath = "/dev/null"
	}

	edits := myers.ComputeEdits(span.URIFromPath(oldPath), string(oldYaml), string(newYaml))
	diff := gotextdiff.ToUnified(oldPath, newPath, string(oldYaml), edits)

	outputDiff(w, diff)

	return nil
}

func outputDiff(w io.Writer, u gotextdiff.Unified) {
	if len(u.Hunks) == 0 {
		return
	}

	bold := color.New(color.Bold)
	bold.Fprintf(w, "--- %s\n", u.From) //nolint:errcheck
	bold.Fprintf(w, "+++ %s\n", u.To)   //nolint:errcheck

	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	for _, hunk := range u.Hunks {
		fromCount, toCount := 0, 0

		for _, l := range hunk.Lines {
			switch l.Kind { //nolint:exhaustive
			case gotextdiff.Delete:
				fromCount++
			case gotextdiff.Insert:
				toCount++
			default:
				fromCount++
				toCount++
			}
		}

		cyan.Fprintf(w, "@@") //nolint:errcheck

		if fromCount > 1 {
			cyan.Fprintf(w, " -%d,%d", hunk.FromLine, fromCount) //nolint:errcheck
		} else {
			cyan.Fprintf(w, " -%d", hunk.FromLine) //nolint:errcheck
		}

		if toCount > 1 {
			cyan.Fprintf(w, " +%d,%d", hunk.ToLine, toCount) //nolint:errcheck
		} else {
			cyan.Fprintf(w, " +%d", hunk.ToLine) //nolint:errcheck
		}

		cyan.Printf(" @@\n") //nolint:errcheck

		for _, l := range hunk.Lines {
			switch l.Kind { //nolint:exhaustive
			case gotextdiff.Delete:
				red.Fprintf(w, "-%s", l.Content) //nolint:errcheck
			case gotextdiff.Insert:
				green.Fprintf(w, "+%s", l.Content) //nolint:errcheck
			default:
				fmt.Fprintf(w, " %s", l.Content)
			}

			if !strings.HasSuffix(l.Content, "\n") {
				red.Fprintf(w, "\n\\ No newline at end of file\n") //nolint:errcheck
			}
		}
	}
}

// Describe a resources in human readable format.
func Describe(r resource.Resource) string {
	return fmt.Sprintf("%s(%s)", r.Metadata().Type(), r.Metadata().ID())
}
