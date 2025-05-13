// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"fmt"
	"iter"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// TeardownAndDestroy calls Teardown for a resource, then calls Destroy if the resource doesn't have finalizers.
// It returns true if the resource were destroyed.
func TeardownAndDestroy(
	ctx context.Context,
	r controller.Writer,
	ptr resource.Pointer,
	options ...controller.DeleteOption,
) (bool, error) {
	ready, err := r.Teardown(ctx, ptr, options...)
	if err != nil {
		if state.IsNotFoundError(err) {
			return true, nil
		}

		return false, fmt.Errorf("failed to teardown resource %w", err)
	}

	if !ready {
		return false, nil
	}

	if err = r.Destroy(ctx, ptr, options...); err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to destroy resource %w", err)
	}

	return true, nil
}

// TeardownAndDestroyAll calls Teardown for all resources, then calls Destroy for all resources which
// have no finalizers.
// It returns true if all resources were destroyed.
func TeardownAndDestroyAll(
	ctx context.Context,
	r controller.Writer,
	resources iter.Seq[resource.Pointer],
	options ...controller.DeleteOption,
) (bool, error) {
	allDestroyed := true

	for res := range resources {
		destroyed, err := TeardownAndDestroy(ctx, r, res, options...)
		if err != nil {
			return false, err
		}

		if !destroyed {
			allDestroyed = false
		}
	}

	return allDestroyed, nil
}
