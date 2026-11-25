// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

const dryRunClusterName = "cluster"

// dryRunRefusingState refuses the dry-run watch the way an older Omni with the same API version does.
type dryRunRefusingState struct {
	state.State
}

func (s dryRunRefusingState) WatchKind(ctx context.Context, kind resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	if kind.Type() == omni.DryRunStatusType {
		return status.Error(codes.PermissionDenied, "access is not permitted")
	}

	return s.State.WatchKind(ctx, kind, ch, opts...)
}

// createDryRunCluster creates a healthy cluster with two machines, of which only machine-1 has a failed dry run.
func createDryRunCluster(ctx context.Context, t *testing.T, st state.State) {
	t.Helper()

	clusterStatus := omni.NewClusterStatus(dryRunClusterName)
	clusterStatus.TypedSpec().Value.Phase = specs.ClusterStatusSpec_RUNNING
	clusterStatus.TypedSpec().Value.Ready = true

	machineSetStatus := omni.NewMachineSetStatus(omni.ControlPlanesResourceID(dryRunClusterName))
	machineSetStatus.Metadata().Labels().Set(omni.LabelCluster, dryRunClusterName)
	machineSetStatus.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
	machineSetStatus.TypedSpec().Value.Phase = specs.MachineSetPhase_Running
	machineSetStatus.TypedSpec().Value.Ready = true

	newMachineStatus := func(machineID string) *omni.ClusterMachineStatus {
		machineStatus := omni.NewClusterMachineStatus(machineID)
		machineStatus.Metadata().Labels().Set(omni.LabelCluster, dryRunClusterName)
		machineStatus.Metadata().Labels().Set(omni.LabelMachineSet, machineSetStatus.Metadata().ID())

		return machineStatus
	}

	dryRun := omni.NewDryRunStatus("machine-1")
	dryRun.Metadata().Labels().Set(omni.LabelCluster, dryRunClusterName)
	dryRun.TypedSpec().Value.Outcome = specs.DryRunStatusSpec_FAILED
	dryRun.TypedSpec().Value.Error = "2 errors occurred:\n\t* first\n\t* second"

	for _, res := range []resource.Resource{clusterStatus, machineSetStatus, newMachineStatus("machine-1"), newMachineStatus("machine-2"), dryRun} {
		require.NoError(t, st.Create(ctx, res))
	}
}

//nolint:paralleltest // color.NoColor is global
func TestStatusClusterShowsDryRun(t *testing.T) {
	color.NoColor = true

	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())

	createDryRunCluster(ctx, t, st)

	var out bytes.Buffer

	require.NoError(t, operations.StatusCluster(ctx, dryRunClusterName, &out, st, operations.StatusOptions{}))

	lines := strings.Split(out.String(), "\n")

	machineLine := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, `Machine "machine-1"`) })
	require.NotEqual(t, -1, machineLine, out.String())
	require.Greater(t, len(lines), machineLine+5, out.String())

	machineIndent := strings.Index(lines[machineLine], "Machine")

	assert.Greater(t, strings.Index(lines[machineLine+1], "Dry run: failed"), machineIndent, "the dry run should be a child of its machine:\n%s", out.String())
	assert.Contains(t, lines[machineLine+2], "2 errors occurred:")
	assert.Contains(t, lines[machineLine+3], "* first")
	assert.Contains(t, lines[machineLine+4], "* second")
	assert.Contains(t, lines[machineLine+5], `Machine "machine-2"`)
	assert.Equal(t, 1, strings.Count(out.String(), "Dry run:"), "only machine-1 has a dry run:\n%s", out.String())
}

//nolint:paralleltest // color.NoColor is global
func TestStatusClusterWithoutDryRunAccess(t *testing.T) {
	color.NoColor = true

	ctx := t.Context()
	st := dryRunRefusingState{State: state.WrapCore(inmem.NewState())}

	createDryRunCluster(ctx, t, st)

	var out bytes.Buffer

	require.NoError(t, operations.StatusCluster(ctx, dryRunClusterName, &out, st, operations.StatusOptions{}))

	assert.Contains(t, out.String(), `Machine "machine-1"`)
	assert.NotContains(t, out.String(), "Dry run:")
}
