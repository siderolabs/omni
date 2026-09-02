// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	yaml "go.yaml.in/yaml/v4"
)

// YAML outputs resources in YAML format.
type YAML struct {
	w          io.Writer
	needDashes bool
	withEvents bool
}

// NewYAML initializes YAML resource output.
func NewYAML(w io.Writer) *YAML {
	return &YAML{w: w}
}

// WriteHeader implements output.Writer interface.
func (y *YAML) WriteHeader(_ *meta.ResourceDefinition, withEvents bool) error {
	y.withEvents = withEvents

	return nil
}

// WriteResource implements output.Writer interface.
func (y *YAML) WriteResource(r resource.Resource, event state.EventType) error {
	out, err := resource.MarshalYAML(r)
	if err != nil {
		return err
	}

	if y.needDashes {
		fmt.Fprintln(y.w, "---") //nolint:errcheck
	}

	y.needDashes = true

	if y.withEvents {
		fmt.Fprintf(y.w, "event: %s\n", strings.ToLower(event.String())) //nolint:errcheck
	}

	return yaml.NewEncoder(y.w).Encode(out)
}

// Flush implements output.Writer interface.
func (y *YAML) Flush() error {
	return nil
}
