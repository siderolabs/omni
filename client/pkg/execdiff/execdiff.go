// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package execdiff provides diff rendering for dry-run operations,
// supporting both built-in colorized output and external diff tools
// via the OMNI_EXTERNAL_DIFF environment variable.
package execdiff

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"

	"github.com/siderolabs/omni/client/pkg/diff"
)

// EnvExternalDiff is the environment variable that specifies an external diff program.
//
// When set, the diff tool is invoked with two directory paths containing the
// old and new resource YAML files. The value may include arguments, e.g.
// "colordiff -N -u".
//
// Exit codes: 0 = no differences, 1 = differences found, >1 = error.
const EnvExternalDiff = "OMNI_EXTERNAL_DIFF"

// ErrDifferencesFound is a sentinel error returned by callers of Differ.Flush()
// when they want to signal that differences were found. Commands should wrap
// their result with this sentinel so that the top-level CLI can translate it
// into the documented exit status (1 = differences found).
var ErrDifferencesFound = errors.New("differences found")

type entry struct {
	label    string
	filename string
	oldYAML  []byte
	newYAML  []byte
}

// Differ accumulates resource diffs and renders them either using a built-in
// colorized unified diff or by invoking an external diff program.
type Differ struct {
	w       io.Writer
	extCmd  string
	entries []entry
	hasDiff bool
}

// New creates a Differ that writes to w. If OMNI_EXTERNAL_DIFF is set,
// diffs are queued and rendered by the external tool on Flush.
func New(w io.Writer) *Differ {
	return &Differ{
		w:      w,
		extCmd: os.Getenv(EnvExternalDiff),
	}
}

// IsExternal returns true if an external diff tool is configured.
func (d *Differ) IsExternal() bool {
	return d.extCmd != ""
}

// SanitizeFilename returns a safe filename derived from the given parts.
//
// Path separators and traversal sequences are replaced with underscores so
// that joining the result with a temp directory base path cannot escape the
// directory. Empty / dot-only parts are replaced with an underscore.
func SanitizeFilename(parts ...string) string {
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		out = append(out, sanitizeFilenamePart(p))
	}

	return strings.Join(out, "-")
}

func sanitizeFilenamePart(part string) string {
	sanitized := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "__",
		":", "_",
		"\x00", "_",
	).Replace(part)

	switch sanitized {
	case "", ".", "..":
		return "_"
	default:
		return sanitized
	}
}

// AddDiff records a diff between two versions of a resource.
//
// For creates, oldYAML should be nil. For deletes, newYAML should be nil.
// label is a human-readable description (e.g. "MachineSet(cluster1)").
// filename is used as the YAML file name in temp directories for external mode;
// callers must provide a sanitized filename (see SanitizeFilename) - filenames
// that contain path separators or traversal sequences are rejected.
//
// In built-in mode, the diff is rendered immediately to the writer.
// In external mode, the entry is queued for Flush.
func (d *Differ) AddDiff(label, filename string, oldYAML, newYAML []byte) error {
	if d.IsExternal() {
		if err := validateFilename(filename); err != nil {
			return err
		}

		d.entries = append(d.entries, entry{
			label:    label,
			filename: filename,
			oldYAML:  oldYAML,
			newYAML:  newYAML,
		})

		return nil
	}

	return d.renderBuiltin(label, oldYAML, newYAML)
}

// validateFilename ensures the filename is safe to join with a temp directory
// base path: no path separators, no traversal components, not absolute, not empty.
func validateFilename(name string) error {
	if name == "" {
		return errors.New("empty filename")
	}

	if filepath.IsAbs(name) {
		return fmt.Errorf("absolute path not allowed: %q", name)
	}

	if name != filepath.Base(name) {
		return fmt.Errorf("filename must not contain path separators: %q", name)
	}

	// filepath.Base("..") returns "..", so this also blocks parent-traversal.
	if name == "." || name == ".." {
		return fmt.Errorf("invalid filename: %q", name)
	}

	return nil
}

// Flush invokes the external diff tool with temp directories containing
// the queued entries. Returns true if differences were found.
//
// In built-in mode this is a no-op and returns hasDiff accumulated from AddDiff calls.
func (d *Differ) Flush() (bool, error) {
	if !d.IsExternal() {
		return d.hasDiff, nil
	}

	if len(d.entries) == 0 {
		return false, nil
	}

	liveDir, err := os.MkdirTemp("", "omni-diff-LIVE-")
	if err != nil {
		return false, fmt.Errorf("failed to create temp dir: %w", err)
	}

	defer os.RemoveAll(liveDir) //nolint:errcheck

	mergedDir, err := os.MkdirTemp("", "omni-diff-MERGED-")
	if err != nil {
		return false, fmt.Errorf("failed to create temp dir: %w", err)
	}

	defer os.RemoveAll(mergedDir) //nolint:errcheck

	for _, e := range d.entries {
		// Filenames were validated in AddDiff, but defend in depth: re-validate
		// here to avoid any path escape if the slice was mutated.
		if err := validateFilename(e.filename); err != nil {
			return false, err
		}

		if e.oldYAML != nil {
			if err := os.WriteFile(filepath.Join(liveDir, e.filename), e.oldYAML, 0o600); err != nil {
				return false, fmt.Errorf("failed to write old YAML for %s: %w", e.label, err)
			}
		}

		if e.newYAML != nil {
			if err := os.WriteFile(filepath.Join(mergedDir, e.filename), e.newYAML, 0o600); err != nil {
				return false, fmt.Errorf("failed to write new YAML for %s: %w", e.label, err)
			}
		}
	}

	return d.runExternal(liveDir, mergedDir)
}

func (d *Differ) renderBuiltin(label string, oldYAML, newYAML []byte) error {
	diffStr, err := diff.Compute(oldYAML, newYAML)
	if err != nil {
		return err
	}

	// Strip the library's generic header; we print our own with the label.
	diffStr, _ = strings.CutPrefix(diffStr, "--- a\n+++ b\n")

	if diffStr == "" {
		return nil
	}

	d.hasDiff = true

	bold := color.New(color.Bold)
	bold.Fprintf(d.w, "--- %s\n", label) //nolint:errcheck
	bold.Fprintf(d.w, "+++ %s\n", label) //nolint:errcheck

	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	for line := range strings.SplitSeq(diffStr, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			cyan.Fprintln(d.w, line) //nolint:errcheck
		case strings.HasPrefix(line, "-"):
			red.Fprintln(d.w, line) //nolint:errcheck
		case strings.HasPrefix(line, "+"):
			green.Fprintln(d.w, line) //nolint:errcheck
		case line == "":
			// skip trailing empty line
		default:
			fmt.Fprintln(d.w, line) //nolint:errcheck
		}
	}

	return nil
}

func (d *Differ) runExternal(liveDir, mergedDir string) (bool, error) {
	parts := strings.Fields(d.extCmd)
	if len(parts) == 0 {
		return false, errors.New("OMNI_EXTERNAL_DIFF is set but empty after parsing")
	}

	name := parts[0]
	args := append(parts[1:], liveDir, mergedDir)

	cmd := exec.Command(name, args...)
	cmd.Stdout = d.w
	cmd.Stderr = d.w

	err := cmd.Run()
	if err == nil {
		return false, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 1 {
			return true, nil
		}

		return false, fmt.Errorf("external diff exited with code %d", exitErr.ExitCode())
	}

	return false, fmt.Errorf("failed to run external diff %q: %w", name, err)
}
