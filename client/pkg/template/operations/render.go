// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations

import (
	"fmt"
	"io"

	"github.com/cosi-project/runtime/pkg/resource"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/pkg/template"
)

// RenderTemplate outputs the rendered template to the given output.
func RenderTemplate(templateReader io.Reader, output io.Writer) error {
	tmpl, err := template.Load(templateReader)
	if err != nil {
		return fmt.Errorf("error loading template: %w", err)
	}

	if err = tmpl.Validate(); err != nil {
		return err
	}

	resources, err := tmpl.Translate()
	if err != nil {
		return fmt.Errorf("error rendering template: %w", err)
	}

	enc := yaml.NewEncoder(output)
	enc.SetIndent(2)

	for _, r := range resources {
		m, err := resource.MarshalYAML(r)
		if err != nil {
			return fmt.Errorf("error marshaling resource: %w", err)
		}

		if err = enc.Encode(m); err != nil {
			return fmt.Errorf("error encoding resource: %w", err)
		}
	}

	return nil
}
