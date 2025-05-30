// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/util/jsonpath"
)

// Table outputs resources in Table view.
type Table struct {
	dynamicColumns []dynamicColumn
	displayType    string
	w              tabwriter.Writer
	withEvents     bool
}

type dynamicColumn func(value any) (string, error)

// NewTable initializes table resource output.
func NewTable() *Table {
	output := &Table{}
	output.w.Init(os.Stdout, 0, 0, 3, ' ', 0)

	return output
}

// WriteHeader implements output.Writer interface.
func (table *Table) WriteHeader(definition *meta.ResourceDefinition, withEvents bool) error {
	table.withEvents = withEvents
	fields := []string{"NAMESPACE", "TYPE", "ID", "VERSION"}

	if withEvents {
		fields = append([]string{"*"}, fields...)
	}

	table.displayType = definition.TypedSpec().DisplayType

	for _, col := range definition.TypedSpec().PrintColumns {
		fields = append(fields, strings.ToUpper(col.Name))

		expr := jsonpath.New(col.Name)
		if err := expr.Parse(col.JSONPath); err != nil {
			return fmt.Errorf("error parsing column %q jsonpath: %w", col.Name, err)
		}

		expr = expr.AllowMissingKeys(true)

		table.dynamicColumns = append(table.dynamicColumns, func(val any) (string, error) {
			var buf bytes.Buffer

			if e := expr.Execute(&buf, val); e != nil {
				return "", e
			}

			return buf.String(), nil
		})
	}

	_, err := fmt.Fprintln(&table.w, strings.Join(fields, "\t"))

	return err
}

// WriteResource implements output.Writer interface.
func (table *Table) WriteResource(r resource.Resource, event state.EventType) error {
	values := []string{r.Metadata().Namespace(), table.displayType, r.Metadata().ID(), r.Metadata().Version().String()}

	if table.withEvents {
		var label string

		switch event {
		case state.Created:
			label = "+"
		case state.Destroyed:
			label = "-"
		case state.Updated:
			label = " "
		case state.Errored, state.Bootstrapped, state.Noop: // ignored
		}

		values = append([]string{label}, values...)
	}

	yamlR, err := resource.MarshalYAML(r)
	if err != nil {
		return err
	}

	marshaled, err := yaml.Marshal(yamlR)
	if err != nil {
		return err
	}

	var data struct {
		Spec any `yaml:"spec"`
	}

	if err = yaml.Unmarshal(marshaled, &data); err != nil {
		return err
	}

	for _, dynamicColumn := range table.dynamicColumns {
		var value string

		value, err = dynamicColumn(data.Spec)
		if err != nil {
			return err
		}

		values = append(values, value)
	}

	_, err = fmt.Fprintln(&table.w, strings.Join(values, "\t"))

	return err
}

// Flush implements output.Writer interface.
func (table *Table) Flush() error {
	return table.w.Flush()
}
