// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validated_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func TestValidations(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	testMgmtAddress := "test-mgmt-address"
	testTalosVersion := "test-talos-version"

	st := validated.NewState(innerSt,
		validated.WithCreateValidations(
			// generic create validator
			func(_ context.Context, res resource.Resource, _ ...state.CreateOption) error {
				val, _ := res.Metadata().Labels().Get("foo")

				if val != "bar" {
					return errors.New("expected label foo to be bar")
				}

				return nil
			},

			// type-specific create validator 1
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.Cluster, _ ...state.CreateOption) error {
				if res.TypedSpec().Value.TalosVersion != testTalosVersion {
					return fmt.Errorf("expected talos version to be %s", testTalosVersion)
				}

				return nil
			}),

			// type-specific create validator 2
			validated.NewCreateValidationForType(func(_ context.Context, res *omni.Machine, _ ...state.CreateOption) error {
				if res.TypedSpec().Value.ManagementAddress != testMgmtAddress {
					return fmt.Errorf("expected management address to be %s", testMgmtAddress)
				}

				return nil
			}),
		),
		validated.WithUpdateValidations(
			// generic update validator
			func(_ context.Context, existingRes resource.Resource, newRes resource.Resource, _ ...state.UpdateOption) error {
				if newRes.Metadata().Labels().Len() != existingRes.Metadata().Labels().Len()+1 {
					return errors.New("every update should add a new label")
				}

				return nil
			},
			// type-specific update validator 1
			validated.NewUpdateValidationForType(func(_ context.Context, _ *omni.Cluster, newRes *omni.Cluster, option ...state.UpdateOption) error {
				var errs error

				if newRes.TypedSpec().Value.TalosVersion != "test-update" {
					errs = multierror.Append(errs, errors.New("expected talos version to be test-update"))
				}

				var opts state.UpdateOptions

				for _, opt := range option {
					opt(&opts)
				}

				if opts.ExpectedPhase == nil || *opts.ExpectedPhase != resource.PhaseRunning {
					errs = multierror.Append(errs, errors.New("expected ExpectedPhase to be PhaseRunning"))
				}

				return errs
			}),
		),
		validated.WithDestroyValidations(
			// generic destroy validator
			func(_ context.Context, _ resource.Pointer, existingRes resource.Resource, _ ...state.DestroyOption) error {
				if _, exists := existingRes.Metadata().Labels().Get("foo3"); !exists {
					return errors.New("cannot delete a resource without label foo3")
				}

				return nil
			},
			// type-specific destroy validator 1
			validated.NewDestroyValidationForType(func(_ context.Context, _ resource.Pointer, existingRes *omni.Machine, _ ...state.DestroyOption) error {
				if existingRes.TypedSpec().Value.ManagementAddress == testMgmtAddress {
					return fmt.Errorf("cannot delete a machine with management address %s", testMgmtAddress)
				}

				return nil
			}),
		),
	)

	// prepare resources
	cluster := omni.NewCluster(resources.DefaultNamespace, "something")
	machine := omni.NewMachine(resources.DefaultNamespace, "something")
	machine.Metadata().Labels().Set("foo", "bar")

	// try to create cluster
	err := st.Create(ctx, cluster)
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "expected label foo to be bar")
	assert.ErrorContains(t, err, "expected talos version to be test")
	assert.NotContains(t, err.Error(), "expected management address to be test")

	// meet one of the creation requirements but not all and try again
	cluster.Metadata().Labels().Set("foo", "bar")
	err = st.Create(ctx, cluster)
	assert.True(t, validated.IsValidationError(err))
	assert.NotContains(t, err.Error(), "expected label foo to be bar")

	// meet remaining creation requirements and try again
	cluster.TypedSpec().Value.TalosVersion = testTalosVersion
	assert.NoError(t, st.Create(ctx, cluster))

	// try to create machine
	err = st.Create(ctx, machine)
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "expected management address to be test")

	// meet all creation requirements and try again
	machine.TypedSpec().Value.ManagementAddress = testMgmtAddress
	err = st.Create(ctx, machine)
	assert.NoError(t, err)

	// try to update cluster
	err = st.Update(ctx, cluster)
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "every update should add a new label")
	assert.ErrorContains(t, err, "expected talos version to be test")
	assert.ErrorContains(t, err, "expected ExpectedPhase to be PhaseRunning")

	// meet all update requirements and try again
	cluster.Metadata().Labels().Set("foo1", "bar")

	cluster.TypedSpec().Value.TalosVersion = "test-update"

	assert.NoError(t, st.Update(ctx, cluster, state.WithExpectedPhase(resource.PhaseRunning)))

	// try to update machine one more time
	cluster.TypedSpec().Value.KubernetesVersion = "foobar"

	err = st.Update(ctx, cluster, state.WithExpectedPhase(resource.PhaseRunning))
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "every update should add a new label")

	// meet all update requirements and try again
	cluster.Metadata().Labels().Set("foo2", "bar")
	assert.NoError(t, st.Update(ctx, cluster, state.WithExpectedPhase(resource.PhaseRunning)))

	// try to destroy cluster
	err = st.Destroy(ctx, cluster.Metadata())
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "cannot delete a resource without label foo3")

	// meet the delete conditions
	cluster.Metadata().Labels().Set("foo3", "bar")
	assert.NoError(t, st.Update(ctx, cluster, state.WithExpectedPhase(resource.PhaseRunning)))

	// try to destroy cluster again
	assert.NoError(t, st.Destroy(ctx, cluster.Metadata()))

	// try to destroy machine
	err = st.Destroy(ctx, machine.Metadata())
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "cannot delete a machine with management address test")

	// meet the delete conditions
	machine.Metadata().Labels().Set("foo3", "bar")

	machine.TypedSpec().Value.ManagementAddress = "test2"

	assert.NoError(t, st.Update(ctx, machine, state.WithExpectedPhase(resource.PhaseRunning)))

	// try to destroy machine again
	assert.NoError(t, st.Destroy(ctx, machine.Metadata()))

	// ensure that both resources are destroyed
	_, err = innerSt.Get(ctx, cluster.Metadata())
	assert.True(t, state.IsNotFoundError(err))

	_, err = innerSt.Get(ctx, machine.Metadata())
	assert.True(t, state.IsNotFoundError(err))
}

func TestTeardownDestroyValidations(t *testing.T) {
	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := state.WrapCore(
		validated.NewState(innerSt,
			validated.WithUpdateValidations(func(context.Context, resource.Resource, resource.Resource, ...state.UpdateOption) error {
				return errors.New("update")
			}), validated.WithDestroyValidations(func(_ context.Context, _ resource.Pointer, _ resource.Resource, option ...state.DestroyOption) error {
				opts := state.DestroyOptions{}

				for _, opt := range option {
					opt(&opts)
				}

				return errors.New("destroy by " + opts.Owner)
			}),
		),
	)

	res := omni.NewCluster(resources.DefaultNamespace, "something")

	require.NoError(t, st.Create(t.Context(), res))

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	_, err := safe.StateUpdateWithConflicts(ctx, st, res.Metadata(), func(res *omni.Cluster) error {
		res.TypedSpec().Value.TalosVersion = "1234"

		return nil
	})
	require.EqualError(t, err, "failed to validate: 1 error occurred:\n\t* update\n\n")

	const teardownOwner = "foobar-controller"

	_, err = st.Teardown(ctx, res.Metadata(), state.WithTeardownOwner(teardownOwner))
	require.EqualError(t, err, fmt.Sprintf("failed to validate: 2 errors occurred:\n\t* update\n\t* destroy by %s\n\n", teardownOwner))
}
