// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package rmock

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
)

// Teardown is the same as rtestutils Teardown but uses the correct owner.
func Teardown[R generic.ResourceWithRD](ctx context.Context, t *testing.T, st state.State, ids []string) {
	var r R

	rtestutils.Teardown[R](ctx, t, st, ids, state.WithTeardownOwner(owners[r.ResourceDefinition().Type]))
}

// Destroy is the same as rtestutils Destroy but removes the resources using the correct owner.
func Destroy[R generic.ResourceWithRD](ctx context.Context, t *testing.T, st state.State, ids []string) {
	var r R

	rtestutils.Destroy[R](ctx, t, st, ids, state.WithDestroyOwner(owners[r.ResourceDefinition().Type]))
}
