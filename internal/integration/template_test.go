// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	_ "embed"
	"os"
	"testing"
	"text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

//go:embed testdata/cluster-1.tmpl.yaml
var cluster1Tmpl []byte

//go:embed testdata/cluster-2.tmpl.yaml
var cluster2Tmpl []byte

type tmplOptions struct {
	KubernetesVersion               string
	TalosVersion                    string
	MachineClass                    string
	MachineClassBasedMachineSetName string

	CP []string
	W  []string
}

func renderTemplate(t *testing.T, tmpl []byte, opts tmplOptions) []byte {
	var b bytes.Buffer

	require.NoError(t, template.Must(template.New("cluster").Parse(string(tmpl))).Execute(&b, opts))

	return b.Bytes()
}

// AssertClusterTemplateFlow verifies cluster template operations.
func AssertClusterTemplateFlow(testCtx context.Context, st state.State, options MachineOptions) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 20*time.Minute)
		defer cancel()

		const (
			clusterName                     = "tmpl-cluster"
			additionalWorkersName           = "additional-workers"
			lockDelete                      = "lockDelete"
			machineClassBasedMachineSetName = "additional-workers-machine-class"
			pickedByManualAllocation        = "picked-by-manual-allocation"
		)

		require := require.New(t)

		machineClassName := "tmpl-cluster-machine-class"

		err := safe.StateModify(ctx, st, omni.NewMachineClass(resources.DefaultNamespace, machineClassName), func(r *omni.MachineClass) error {
			r.TypedSpec().Value.MatchLabels = []string{"!" + pickedByManualAllocation}

			return nil
		})

		require.NoError(err)

		t.Cleanup(func() {
			rtestutils.Destroy[*omni.MachineClass](testCtx, t, st, []string{machineClassName})
		})

		var (
			machineIDs                     []resource.ID
			opts                           tmplOptions
			tmpl1                          []byte
			automaticallyAllocatedMachines []string
		)

		pickUnallocatedMachines(ctx, t, st, 5, nil, func(mIDs []resource.ID) {
			machineIDs = mIDs

			opts = tmplOptions{
				KubernetesVersion:               "v" + options.KubernetesVersion,
				TalosVersion:                    "v" + options.TalosVersion,
				MachineClass:                    machineClassName,
				MachineClassBasedMachineSetName: machineClassBasedMachineSetName,

				CP: machineIDs[:3],
				W:  machineIDs[3:],
			}

			for _, m := range mIDs {
				err = safe.StateModify(ctx, st, omni.NewMachineLabels(resources.DefaultNamespace, m), func(labels *omni.MachineLabels) error {
					labels.Metadata().Labels().Set(pickedByManualAllocation, "")

					return nil
				})

				require.NoError(err)

				t.Cleanup(func() {
					err = safe.StateModify(testCtx, st, omni.NewMachineLabels(resources.DefaultNamespace, m), func(labels *omni.MachineLabels) error {
						labels.Metadata().Labels().Delete(pickedByManualAllocation)

						return nil
					})

					require.NoError(err)
				})
			}

			rtestutils.AssertResources(ctx, t, st, mIDs, func(status *omni.MachineStatus, assert *assert.Assertions) {
				_, ok := status.Metadata().Labels().Get(pickedByManualAllocation)

				assert.True(ok)
			})

			tmpl1 = renderTemplate(t, cluster1Tmpl, opts)

			require.NoError(operations.ValidateTemplate(bytes.NewReader(tmpl1)))

			t.Log("creating template cluster")

			require.NoError(operations.SyncTemplate(ctx, bytes.NewReader(tmpl1), os.Stderr, st, operations.SyncOptions{
				Verbose: true,
			}))

			// assert that machines got allocated (label available is removed)
			rtestutils.AssertResources(ctx, t, st, machineIDs, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
				assert.True(machineStatus.Metadata().Labels().Matches(
					resource.LabelTerm{
						Key:    omni.MachineStatusLabelAvailable,
						Op:     resource.LabelOpExists,
						Invert: true,
					},
				), resourceDetails(machineStatus))
			})

			machineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, st, clusterName+"-"+machineClassBasedMachineSetName)
			require.NoError(err)

			automaticallyAllocatedMachines = rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(
				resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()),
			))

			require.Greater(len(automaticallyAllocatedMachines), 0)

			// add finalizer on the machine set node to make the test fail if it tries to delete the machine set node unexpectedly
			require.NoError(st.AddFinalizer(ctx,
				omni.NewMachineSetNode(resources.DefaultNamespace, automaticallyAllocatedMachines[0], machineSet).Metadata(),
				lockDelete,
			))

			t.Cleanup(func() {
				err = st.RemoveFinalizer(testCtx,
					omni.NewMachineSetNode(
						resources.DefaultNamespace,
						automaticallyAllocatedMachines[0],
						omni.NewMachineSet(resources.DefaultNamespace, ""),
					).Metadata(),
					lockDelete,
				)
			})
		})

		t.Log("wait for cluster to be ready")

		// wait using the status command
		require.NoError(operations.StatusTemplate(ctx, bytes.NewReader(tmpl1), os.Stderr, st, operations.StatusOptions{
			Wait: true,
		}))

		// re-check with short timeout to make sure the cluster is ready
		checkCtx, checkCancel := context.WithTimeout(ctx, 30*time.Second)
		defer checkCancel()

		rtestutils.AssertResources(checkCtx, t, st, []string{clusterName}, func(status *omni.ClusterStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.True(spec.Available, "not available: %s", resourceDetails(status))
			assert.Equal(specs.ClusterStatusSpec_RUNNING, spec.Phase, "cluster is not in phase running: %s", resourceDetails(status))
			assert.Equal(spec.GetMachines().Total, spec.GetMachines().Healthy, "not all machines are healthy: %s", resourceDetails(status))
			assert.True(spec.Ready, "cluster is not ready: %s", resourceDetails(status))
			assert.True(spec.ControlplaneReady, "cluster controlplane is not ready: %s", resourceDetails(status))
			assert.True(spec.KubernetesAPIReady, "cluster kubernetes API is not ready: %s", resourceDetails(status))
			assert.EqualValues(len(opts.CP)+len(opts.W)+1, spec.GetMachines().Total, "total machines is not the same as in the machine sets: %s", resourceDetails(status))
		})

		rtestutils.AssertResources(checkCtx, t, st, []string{
			omni.ControlPlanesResourceID(clusterName),
			omni.WorkersResourceID(clusterName),
			omni.AdditionalWorkersResourceID(clusterName, additionalWorkersName),
		}, func(*omni.MachineSet, *assert.Assertions) {})

		t.Log("updating template cluster")

		opts.CP = opts.CP[:1]

		tmpl2 := renderTemplate(t, cluster2Tmpl, opts)

		require.NoError(operations.SyncTemplate(ctx, bytes.NewReader(tmpl2), os.Stderr, st, operations.SyncOptions{
			Verbose: true,
		}))

		t.Log("waiting for cluster operations to apply")

		time.Sleep(10 * time.Second)

		t.Log("wait for cluster to be ready")

		// wait using the status command
		require.NoError(operations.StatusTemplate(ctx, bytes.NewReader(tmpl2), os.Stderr, st, operations.StatusOptions{
			Wait: true,
		}))

		newAutomaticallyALlocatedMachines := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelMachineSet, clusterName+"-"+machineClassBasedMachineSetName),
		))

		require.Greater(len(newAutomaticallyALlocatedMachines), 1)

		for _, id := range automaticallyAllocatedMachines {
			require.Contains(newAutomaticallyALlocatedMachines, id)

			require.NoError(st.RemoveFinalizer(ctx, omni.NewMachineSetNode(resources.DefaultNamespace, id, omni.NewMachineSet(resources.DefaultNamespace, "")).Metadata(),
				lockDelete,
			))
		}

		// re-check with short timeout to make sure the cluster is ready
		checkCtx, checkCancel = context.WithTimeout(ctx, 10*time.Second)
		defer checkCancel()

		rtestutils.AssertResources(checkCtx, t, st, []string{clusterName}, func(status *omni.ClusterStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.True(spec.Available, "not available: %s", resourceDetails(status))
			assert.Equal(specs.ClusterStatusSpec_RUNNING, spec.Phase, "cluster is not in phase running: %s", resourceDetails(status))
			assert.Equal(spec.GetMachines().Total, spec.GetMachines().Healthy, "not all machines are healthy: %s", resourceDetails(status))
			assert.True(spec.Ready, "cluster is not ready: %s", resourceDetails(status))
			assert.True(spec.ControlplaneReady, "cluster controlplane is not ready: %s", resourceDetails(status))
			assert.True(spec.KubernetesAPIReady, "cluster kubernetes API is not ready: %s", resourceDetails(status))
			assert.EqualValues(len(opts.CP)+len(opts.W)+2, spec.GetMachines().Total, "total machines is not the same as in the machine sets: %s", resourceDetails(status))
		})

		require.NoError(operations.ValidateTemplate(bytes.NewReader(tmpl1)))

		t.Log("deleting template cluster")

		require.NoError(operations.DeleteTemplate(ctx, bytes.NewReader(tmpl1), os.Stderr, st, operations.SyncOptions{
			Verbose: true,
		}))

		rtestutils.AssertNoResource[*omni.Cluster](ctx, t, st, clusterName)

		// make sure machines are returned to the pool or allocated into another cluster
		rtestutils.AssertResources(ctx, t, st, machineIDs, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
			assert.True(machineStatus.Metadata().Labels().Matches(resource.LabelTerm{
				Key: omni.MachineStatusLabelAvailable,
				Op:  resource.LabelOpExists,
			}) || machineStatus.Metadata().Labels().Matches(resource.LabelTerm{
				Key:    omni.LabelCluster,
				Op:     resource.LabelOpEqual,
				Value:  []string{clusterName},
				Invert: true,
			}), resourceDetails(machineStatus))
		})
	}
}
