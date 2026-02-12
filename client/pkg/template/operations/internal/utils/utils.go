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
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/diff"
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

	diffStr, err := diff.Compute(oldYaml, newYaml)
	if err != nil {
		return err
	}

	outputDiff(w, diffStr, oldPath, newPath)

	return nil
}

func outputDiff(w io.Writer, diffStr, fromPath, toPath string) {
	// Strip the library's generic header; we print our own with resource paths.
	diffStr, _ = strings.CutPrefix(diffStr, "--- a\n+++ b\n")

	if diffStr == "" {
		return
	}

	bold := color.New(color.Bold)
	bold.Fprintf(w, "--- %s\n", fromPath) //nolint:errcheck
	bold.Fprintf(w, "+++ %s\n", toPath)   //nolint:errcheck

	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	for line := range strings.SplitSeq(diffStr, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			cyan.Fprintln(w, line) //nolint:errcheck
		case strings.HasPrefix(line, "-"):
			red.Fprintln(w, line) //nolint:errcheck
		case strings.HasPrefix(line, "+"):
			green.Fprintln(w, line) //nolint:errcheck
		case line == "":
			// skip trailing empty line
		default:
			fmt.Fprintln(w, line) //nolint:errcheck
		}
	}
}

// Describe a resources in human readable format.
func Describe(r resource.Resource) string {
	return fmt.Sprintf("%s(%s)", r.Metadata().Type(), r.Metadata().ID())
}
