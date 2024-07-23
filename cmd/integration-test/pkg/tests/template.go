// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	clientconsts "github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/operations"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

//go:embed testdata/cluster-1.tmpl.yaml
var cluster1Tmpl []byte

//go:embed testdata/cluster-2.tmpl.yaml
var cluster2Tmpl []byte

type tmplOptions struct {
	KubernetesVersion string
	TalosVersion      string

	CP []string
	W  []string
}

func renderTemplate(t *testing.T, tmpl []byte, opts tmplOptions) []byte {
	var b bytes.Buffer

	require.NoError(t, template.Must(template.New("cluster").Parse(string(tmpl))).Execute(&b, opts))

	return b.Bytes()
}

// AssertClusterTemplateFlow verifies cluster template operations.
func AssertClusterTemplateFlow(testCtx context.Context, st state.State) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 20*time.Minute)
		defer cancel()

		const (
			clusterName           = "tmpl-cluster"
			additionalWorkersName = "additional-workers"
		)

		require := require.New(t)

		var (
			machineIDs []resource.ID
			opts       tmplOptions
			tmpl1      []byte
		)

		pickUnallocatedMachines(ctx, t, st, 5, func(mIDs []resource.ID) {
			machineIDs = mIDs

			opts = tmplOptions{
				KubernetesVersion: "v" + constants.DefaultKubernetesVersion,
				TalosVersion:      "v" + clientconsts.DefaultTalosVersion,

				CP: machineIDs[:3],
				W:  machineIDs[3:],
			}

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
		})

		t.Log("wait for cluster to be ready")

		// wait using the status command
		require.NoError(operations.StatusTemplate(ctx, bytes.NewReader(tmpl1), os.Stderr, st, operations.StatusOptions{
			Wait: true,
		}))

		// re-check with short timeout to make sure the cluster is ready
		checkCtx, checkCancel := context.WithTimeout(ctx, 10*time.Second)
		defer checkCancel()

		rtestutils.AssertResources(checkCtx, t, st, []string{clusterName}, func(status *omni.ClusterStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.Truef(spec.Available, "not available: %s", resourceDetails(status))
			assert.Equalf(specs.ClusterStatusSpec_RUNNING, spec.Phase, "cluster is not in phase running: %s", resourceDetails(status))
			assert.Equalf(spec.GetMachines().Total, spec.GetMachines().Healthy, "not all machines are healthy: %s", resourceDetails(status))
			assert.Truef(spec.Ready, "cluster is not ready: %s", resourceDetails(status))
			assert.Truef(spec.ControlplaneReady, "cluster controlplane is not ready: %s", resourceDetails(status))
			assert.Truef(spec.KubernetesAPIReady, "cluster kubernetes API is not ready: %s", resourceDetails(status))
			assert.EqualValuesf(len(opts.CP)+len(opts.W), spec.GetMachines().Total, "total machines is not the same as in the machine sets: %s", resourceDetails(status))
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

		// re-check with short timeout to make sure the cluster is ready
		checkCtx, checkCancel = context.WithTimeout(ctx, 10*time.Second)
		defer checkCancel()

		rtestutils.AssertResources(checkCtx, t, st, []string{clusterName}, func(status *omni.ClusterStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.Truef(spec.Available, "not available: %s", resourceDetails(status))
			assert.Equalf(specs.ClusterStatusSpec_RUNNING, spec.Phase, "cluster is not in phase running: %s", resourceDetails(status))
			assert.Equalf(spec.GetMachines().Total, spec.GetMachines().Healthy, "not all machines are healthy: %s", resourceDetails(status))
			assert.Truef(spec.Ready, "cluster is not ready: %s", resourceDetails(status))
			assert.Truef(spec.ControlplaneReady, "cluster controlplane is not ready: %s", resourceDetails(status))
			assert.Truef(spec.KubernetesAPIReady, "cluster kubernetes API is not ready: %s", resourceDetails(status))
			assert.EqualValuesf(len(opts.CP)+len(opts.W), spec.GetMachines().Total, "total machines is not the same as in the machine sets: %s", resourceDetails(status))
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
