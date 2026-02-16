// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

func TestClusterStatusReconcile(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct { //nolint:govet
		name             string
		cpMachineSet     *specs.MachineSetStatusSpec
		workerMachineSet *specs.MachineSetStatusSpec
		cpStatus         *specs.ControlPlaneStatusSpec
		lbStatus         *specs.LoadBalancerStatusSpec
		expected         *specs.ClusterStatusSpec
	}{
		{
			name: "no-statuses",
			expected: &specs.ClusterStatusSpec{
				Available:          false,
				Phase:              specs.ClusterStatusSpec_UNKNOWN,
				Ready:              false,
				ControlplaneReady:  false,
				KubernetesAPIReady: false,
				Machines: &specs.Machines{
					Total:   0,
					Healthy: 0,
				},
			},
		},
		{
			name: "all-healthy",
			cpMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:   3,
					Healthy: 3,
				},
			},
			workerMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:   2,
					Healthy: 2,
				},
			},
			cpStatus: &specs.ControlPlaneStatusSpec{
				Conditions: []*specs.ControlPlaneStatusSpec_Condition{
					{
						Type:   specs.ConditionType_Etcd,
						Reason: "",
						Status: specs.ControlPlaneStatusSpec_Condition_Ready,
					},
				},
			},
			lbStatus: &specs.LoadBalancerStatusSpec{
				Healthy: true,
			},
			expected: &specs.ClusterStatusSpec{
				Available:          true,
				Phase:              specs.ClusterStatusSpec_RUNNING,
				Ready:              true,
				ControlplaneReady:  true,
				KubernetesAPIReady: true,
				Machines: &specs.Machines{
					Total:   5,
					Healthy: 5,
				},
			},
		},
		{
			name: "cp-not-healthy",
			cpMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:   3,
					Healthy: 3,
				},
			},
			workerMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:   2,
					Healthy: 2,
				},
			},
			cpStatus: &specs.ControlPlaneStatusSpec{
				Conditions: []*specs.ControlPlaneStatusSpec_Condition{
					{
						Type:   specs.ConditionType_Etcd,
						Reason: "",
						Status: specs.ControlPlaneStatusSpec_Condition_NotReady,
					},
				},
			},
			lbStatus: &specs.LoadBalancerStatusSpec{
				Healthy: false,
			},
			expected: &specs.ClusterStatusSpec{
				Available:          true,
				Phase:              specs.ClusterStatusSpec_RUNNING,
				Ready:              true,
				ControlplaneReady:  false,
				KubernetesAPIReady: false,
				Machines: &specs.Machines{
					Total:   5,
					Healthy: 5,
				},
			},
		},
		{
			name: "scaling-up",
			cpMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_ScalingUp,
				Ready: false,
				Machines: &specs.Machines{
					Total:   2,
					Healthy: 1,
				},
			},
			cpStatus: &specs.ControlPlaneStatusSpec{},
			lbStatus: &specs.LoadBalancerStatusSpec{
				Healthy: true,
			},
			expected: &specs.ClusterStatusSpec{
				Available:          true,
				Phase:              specs.ClusterStatusSpec_SCALING_UP,
				Ready:              false,
				ControlplaneReady:  false,
				KubernetesAPIReady: true,
				Machines: &specs.Machines{
					Total:   2,
					Healthy: 1,
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			testutils.WithRuntime(
				ctx,
				t,
				testutils.TestOptions{},
				func(_ context.Context, testContext testutils.TestContext) {
					require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
				},
				func(ctx context.Context, testContext testutils.TestContext) {
					st := testContext.State
					clusterName := tt.name

					cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))

					if tt.cpMachineSet != nil {
						cpMS := rmock.Mock[*omni.MachineSet](ctx, t, st,
							options.WithID(clusterName+"-cp"),
							options.LabelCluster(cluster),
							options.EmptyLabel(omni.LabelControlPlaneRole),
						)

						rmock.Mock[*omni.MachineSetStatus](ctx, t, st,
							options.SameID(cpMS),
							options.Modify(func(res *omni.MachineSetStatus) error {
								res.TypedSpec().Value = tt.cpMachineSet

								return nil
							}),
						)
					}

					if tt.workerMachineSet != nil {
						workerMS := rmock.Mock[*omni.MachineSet](ctx, t, st,
							options.WithID(omni.WorkersResourceID(clusterName)),
							options.LabelCluster(cluster),
							options.EmptyLabel(omni.LabelWorkerRole),
						)

						rmock.Mock[*omni.MachineSetStatus](ctx, t, st,
							options.SameID(workerMS),
							options.Modify(func(res *omni.MachineSetStatus) error {
								res.TypedSpec().Value = tt.workerMachineSet

								return nil
							}),
						)
					}

					if tt.cpStatus != nil {
						rmock.Mock[*omni.ControlPlaneStatus](ctx, t, st,
							options.WithID(clusterName+"-cp"),
							options.Modify(func(res *omni.ControlPlaneStatus) error {
								res.Metadata().Labels().Set(omni.LabelCluster, clusterName)
								res.TypedSpec().Value = tt.cpStatus

								return nil
							}),
						)
					}

					if tt.lbStatus != nil {
						rmock.Mock[*omni.LoadBalancerStatus](ctx, t, st,
							options.WithID(clusterName),
							options.Modify(func(res *omni.LoadBalancerStatus) error {
								res.TypedSpec().Value = tt.lbStatus

								return nil
							}),
						)
					}

					expected := tt.expected

					rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
						func(status *omni.ClusterStatus, a *assert.Assertions) {
							a.Equal(expected.Available, status.TypedSpec().Value.Available)
							a.Equal(expected.Phase, status.TypedSpec().Value.Phase)
							a.Equal(expected.ControlplaneReady, status.TypedSpec().Value.ControlplaneReady)
							a.Equal(expected.KubernetesAPIReady, status.TypedSpec().Value.KubernetesAPIReady)
							a.Equal(expected.Machines, status.TypedSpec().Value.Machines)
						})
				},
			)
		})
	}
}

func TestClusterStatusImportedTaint(t *testing.T) {
	t.Parallel()

	t.Run("imported cluster has taint when CAs not rotated", func(t *testing.T) {
		t.Parallel()

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "imported-no-rotation"

				cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))

				rmock.Mock[*omni.ClusterSecrets](ctx, t, st,
					options.SameID(cluster),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.Imported = true

						return nil
					}),
				)

				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(status *omni.ClusterStatus, a *assert.Assertions) {
						_, tainted := status.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
						a.True(tainted, "imported cluster without rotated CAs should have taint label")
					})
			},
		)
	})

	t.Run("imported cluster taint removed after both CAs rotated", func(t *testing.T) {
		t.Parallel()

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "imported-both-rotated"

				cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))

				rmock.Mock[*omni.ClusterSecrets](ctx, t, st,
					options.SameID(cluster),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.Imported = true

						return nil
					}),
				)

				// First, verify the taint is present
				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(status *omni.ClusterStatus, a *assert.Assertions) {
						_, tainted := status.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
						a.True(tainted)
					})

				// Set both rotation timestamps on ClusterSecrets
				now := strconv.Itoa(int(time.Now().Unix()))

				rmock.Mock[*omni.ClusterSecrets](ctx, t, st,
					options.SameID(cluster),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.Metadata().Annotations().Set(omni.RotateTalosCATimestamp, now)
						res.Metadata().Annotations().Set(omni.RotateKubernetesCATimestamp, now)

						return nil
					}),
				)

				// Verify the taint is removed
				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(status *omni.ClusterStatus, a *assert.Assertions) {
						_, tainted := status.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
						a.False(tainted, "imported cluster with both CAs rotated should not have taint label")
					})
			},
		)
	})

	t.Run("imported cluster keeps taint when only Talos CA rotated", func(t *testing.T) {
		t.Parallel()

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "imported-talos-only"

				cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))

				rmock.Mock[*omni.ClusterSecrets](ctx, t, st,
					options.SameID(cluster),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.Imported = true
						res.Metadata().Annotations().Set(omni.RotateTalosCATimestamp, strconv.Itoa(int(time.Now().Unix())))

						return nil
					}),
				)

				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(status *omni.ClusterStatus, a *assert.Assertions) {
						_, tainted := status.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
						a.True(tainted, "imported cluster with only Talos CA rotated should still have taint label")
					})
			},
		)
	})
}

func TestClusterStatusBreakGlassTaint(t *testing.T) {
	t.Parallel()

	t.Run("break glass taint removed after both CAs rotated", func(t *testing.T) {
		t.Parallel()

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "breakglass-both-rotated"

				breakGlassTime := time.Now().Add(-1 * time.Hour)
				rotationTime := time.Now()

				cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))

				rmock.Mock[*omni.ClusterSecrets](ctx, t, st,
					options.SameID(cluster),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.Metadata().Annotations().Set(omni.RotateTalosCATimestamp, strconv.Itoa(int(rotationTime.Unix())))
						res.Metadata().Annotations().Set(omni.RotateKubernetesCATimestamp, strconv.Itoa(int(rotationTime.Unix())))

						return nil
					}),
				)

				// Wait for ClusterStatus to be created first
				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(_ *omni.ClusterStatus, _ *assert.Assertions) {})

				// Set the break glass taint on ClusterStatus (this is normally done by another controller)
				rmock.Mock[*omni.ClusterStatus](ctx, t, st,
					options.WithID(clusterName),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.Metadata().Labels().Set(omni.LabelClusterTaintedByBreakGlass, "")
						res.Metadata().Annotations().Set(omni.TaintedByBreakGlassTimestamp, strconv.Itoa(int(breakGlassTime.Unix())))

						return nil
					}),
				)

				// Verify the break glass taint is removed because both rotations happened after break glass
				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(status *omni.ClusterStatus, a *assert.Assertions) {
						_, tainted := status.Metadata().Labels().Get(omni.LabelClusterTaintedByBreakGlass)
						a.False(tainted, "break glass taint should be removed when both CAs rotated after break glass")

						_, hasTimestamp := status.Metadata().Annotations().Get(omni.TaintedByBreakGlassTimestamp)
						a.False(hasTimestamp, "break glass timestamp annotation should be removed")
					})
			},
		)
	})

	t.Run("break glass taint kept when rotations happened before break glass", func(t *testing.T) {
		t.Parallel()

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "breakglass-before-rotation"

				rotationTime := time.Now().Add(-1 * time.Hour)
				breakGlassTime := time.Now()

				cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))

				rmock.Mock[*omni.ClusterSecrets](ctx, t, st,
					options.SameID(cluster),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.Metadata().Annotations().Set(omni.RotateTalosCATimestamp, strconv.Itoa(int(rotationTime.Unix())))
						res.Metadata().Annotations().Set(omni.RotateKubernetesCATimestamp, strconv.Itoa(int(rotationTime.Unix())))

						return nil
					}),
				)

				// Wait for ClusterStatus to be created
				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(_ *omni.ClusterStatus, _ *assert.Assertions) {})

				// Set the break glass taint with timestamp AFTER the rotations
				rmock.Mock[*omni.ClusterStatus](ctx, t, st,
					options.WithID(clusterName),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.Metadata().Labels().Set(omni.LabelClusterTaintedByBreakGlass, "")
						res.Metadata().Annotations().Set(omni.TaintedByBreakGlassTimestamp, strconv.Itoa(int(breakGlassTime.Unix())))

						return nil
					}),
				)

				// Trigger a reconcile by updating the cluster
				rmock.Mock[*omni.Cluster](ctx, t, st,
					options.WithID(clusterName),
					options.Modify(func(res *omni.Cluster) error {
						res.Metadata().Annotations().Set("trigger", "reconcile")

						return nil
					}),
				)

				// The break glass taint should still be present
				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(status *omni.ClusterStatus, a *assert.Assertions) {
						_, tainted := status.Metadata().Labels().Get(omni.LabelClusterTaintedByBreakGlass)
						a.True(tainted, "break glass taint should remain when rotations happened before break glass")
					})
			},
		)
	})

	t.Run("break glass taint kept when only one CA rotated after break glass", func(t *testing.T) {
		t.Parallel()

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "breakglass-one-rotated"

				breakGlassTime := time.Now().Add(-1 * time.Hour)

				cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))

				// Only Talos CA rotated
				rmock.Mock[*omni.ClusterSecrets](ctx, t, st,
					options.SameID(cluster),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.Metadata().Annotations().Set(omni.RotateTalosCATimestamp, strconv.Itoa(int(time.Now().Unix())))

						return nil
					}),
				)

				// Wait for ClusterStatus to be created
				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(_ *omni.ClusterStatus, _ *assert.Assertions) {})

				// Set the break glass taint
				rmock.Mock[*omni.ClusterStatus](ctx, t, st,
					options.WithID(clusterName),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.Metadata().Labels().Set(omni.LabelClusterTaintedByBreakGlass, "")
						res.Metadata().Annotations().Set(omni.TaintedByBreakGlassTimestamp, strconv.Itoa(int(breakGlassTime.Unix())))

						return nil
					}),
				)

				// Trigger a reconcile
				rmock.Mock[*omni.Cluster](ctx, t, st,
					options.WithID(clusterName),
					options.Modify(func(res *omni.Cluster) error {
						res.Metadata().Annotations().Set("trigger", "reconcile")

						return nil
					}),
				)

				// The break glass taint should still be present
				rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName},
					func(status *omni.ClusterStatus, a *assert.Assertions) {
						_, tainted := status.Metadata().Labels().Get(omni.LabelClusterTaintedByBreakGlass)
						a.True(tainted, "break glass taint should remain when only Talos CA rotated")
					})
			},
		)
	})
}
