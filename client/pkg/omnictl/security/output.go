// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	yaml "go.yaml.in/yaml/v4"
)

// outputFlag is the flag set shared by all security commands: how to render the fetched
// artifact(s), mirroring `omnictl get`'s -o flag. json/yaml render the structured result value;
// a command that also supports a human-readable default (scan's severity table) checks
// f.format itself rather than going through write, since that shape isn't a plain encoding of
// the same value.
type outputFlag struct {
	format string
}

func registerOutputFlag(cmd *cobra.Command, flag *outputFlag, def, help string) {
	cmd.Flags().StringVarP(&flag.format, "output", "o", def, help)
}

// validate checks the output flag for validity without making any network calls.
func (f outputFlag) validate(allowed ...string) error {
	if slices.Contains(allowed, f.format) {
		return nil
	}

	return fmt.Errorf("invalid --output %q: must be one of %s", f.format, strings.Join(allowed, ", "))
}

// write renders v to stdout in the requested format.
func (f outputFlag) write(v any) error {
	stdout := os.Stdout //nolint:forbidigo

	if f.format != "yaml" {
		return writeJSON(stdout, v)
	}

	var buf bytes.Buffer

	if err := writeJSON(&buf, v); err != nil {
		return err
	}

	var generic any

	if err := json.Unmarshal(buf.Bytes(), &generic); err != nil {
		return err
	}

	return yaml.NewEncoder(stdout).Encode(generic)
}

// writeJSON encodes v as indented JSON to w, streaming rather than buffering the whole document.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	return enc.Encode(v)
}

// rawJSON returns data as a json.RawMessage.
func rawJSON(data []byte) (json.RawMessage, error) {
	if !json.Valid(data) {
		return nil, errors.New("factory returned invalid JSON")
	}

	return json.RawMessage(data), nil
}
