// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package output provides writers in different formats.
package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/jsonpath"
)

// Writer interface.
type Writer interface {
	WriteHeader(definition *meta.ResourceDefinition, withEvents bool) error
	WriteResource(r resource.Resource, event state.EventType) error
	Flush() error
}

// NewWriter builds writer from type.
func NewWriter(format string) (Writer, error) { //nolint:ireturn
	switch {
	case format == "table":
		return NewTable(), nil
	case format == "yaml":
		return NewYAML(), nil
	case format == "json":
		return NewJSON(os.Stdout), nil
	case strings.HasPrefix(format, "jsonpath="):
		path := format[len("jsonpath="):]

		jp := jsonpath.New("talos")

		if err := jp.Parse(path); err != nil {
			return nil, fmt.Errorf("error parsing jsonpath: %w", err)
		}

		return NewJSONPath(os.Stdout, jp), nil
	default:
		return nil, fmt.Errorf("output format %q is not supported", format)
	}
}

// CompleteOutputArg represents tab completion for `--output` argument.
func CompleteOutputArg(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return []string{"json", "table", "yaml"}, cobra.ShellCompDirectiveNoFileComp
}
