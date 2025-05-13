// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

type runtime struct {
	state.State
}

func (rt *runtime) Create(ctx context.Context, r resource.Resource) error {
	return rt.State.Create(ctx, r)
}

func (rt *runtime) Destroy(ctx context.Context, r resource.Pointer, _ ...controller.DeleteOption) error {
	return rt.State.Destroy(ctx, r)
}

func (rt *runtime) Modify(ctx context.Context, r resource.Resource, f func(resource.Resource) error, _ ...controller.ModifyOption) error {
	_, err := rt.ModifyWithResult(ctx, r, f)

	return err
}

func (rt *runtime) Update(ctx context.Context, r resource.Resource) error {
	return rt.State.Update(ctx, r)
}

func (rt *runtime) ModifyWithResult(ctx context.Context, r resource.Resource, f func(resource.Resource) error, _ ...controller.ModifyOption) (resource.Resource, error) {
	res, err := rt.UpdateWithConflicts(ctx, r.Metadata(), f)
	if state.IsNotFoundError(err) {
		res = r.DeepCopy()

		if err = f(res); err != nil {
			return nil, err
		}

		return res, rt.State.Create(ctx, res)
	}

	return res, err
}

func (rt *runtime) Teardown(ctx context.Context, r resource.Pointer, _ ...controller.DeleteOption) (bool, error) {
	return rt.State.Teardown(ctx, r)
}

func TestTeardownAndDestroy(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)

	defer cancel()

	state := state.WrapCore(namespaced.NewState(inmem.Build))

	rt := &runtime{
		State: state,
	}

	resources := []resource.Resource{
		omni.NewConfigPatch(resources.DefaultNamespace, "100"),
		omni.NewConfigPatch(resources.DefaultNamespace, "101"),
		omni.NewConfigPatch(resources.DefaultNamespace, "102"),
		omni.NewConfigPatch(resources.DefaultNamespace, "103"),
		omni.NewConfigPatch(resources.DefaultNamespace, "104"),
		omni.NewConfigPatch(resources.DefaultNamespace, "105"),
	}

	withFinalizer := 4
	skip := 5
	finalizer := "F"

	for i, res := range resources {
		if i == skip {
			continue
		}

		require.NoError(t, state.Create(ctx, res))

		if i == withFinalizer {
			require.NoError(t, state.AddFinalizer(ctx, res.Metadata(), finalizer))
		}
	}

	destroyed, err := helpers.TeardownAndDestroyAll(ctx, rt, slices.Values(xslices.Map(resources, func(r resource.Resource) resource.Pointer {
		return r.Metadata()
	})))
	require.NoError(t, err)
	require.False(t, destroyed)

	for i, res := range resources {
		if i != withFinalizer {
			rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, t, state, res.Metadata().ID())

			continue
		}

		rtestutils.AssertResource(ctx, t, state, res.Metadata().ID(), func(res *omni.ConfigPatch, assert *assert.Assertions) {
			assert.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
		})
	}

	require.NoError(t, state.RemoveFinalizer(ctx, resources[withFinalizer].Metadata(), finalizer))

	destroyed, err = helpers.TeardownAndDestroyAll(ctx, rt, slices.Values(xslices.Map(resources, func(r resource.Resource) resource.Pointer {
		return r.Metadata()
	})))
	require.NoError(t, err)
	require.True(t, destroyed)

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, t, state, resources[withFinalizer].Metadata().ID())
}
