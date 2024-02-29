// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations

import (
	"context"
	"fmt"
	"io"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/template"
	"github.com/siderolabs/omni/client/pkg/template/operations/internal/utils"
)

// DiffTemplate outputs the diff between template resources and existing resources.
func DiffTemplate(ctx context.Context, templateReader io.Reader, output io.Writer, st state.State) error {
	tmpl, err := template.Load(templateReader)
	if err != nil {
		return fmt.Errorf("error loading template: %w", err)
	}

	if err = tmpl.Validate(); err != nil {
		return err
	}

	syncResult, err := tmpl.Sync(ctx, st)
	if err != nil {
		return fmt.Errorf("error syncing template: %w", err)
	}

	for _, p := range syncResult.Update {
		if err = utils.RenderDiff(output, p.Old, p.New); err != nil {
			return err
		}
	}

	for _, r := range syncResult.Create {
		if err = utils.RenderDiff(output, nil, r); err != nil {
			return err
		}
	}

	for _, phase := range syncResult.Destroy {
		for _, r := range phase {
			if err = utils.RenderDiff(output, r, nil); err != nil {
				return err
			}
		}
	}

	return nil
}
