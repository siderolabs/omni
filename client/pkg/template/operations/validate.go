// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations

import (
	"fmt"
	"io"

	"github.com/siderolabs/omni/client/pkg/template"
)

// ValidateTemplate performs template validation.
func ValidateTemplate(templateReader io.Reader) error {
	tmpl, err := template.Load(templateReader)
	if err != nil {
		return fmt.Errorf("error loading template: %w", err)
	}

	return tmpl.Validate()
}
