// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package layeredresource_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/layeredresource"
)

func TestHelperGetSkipsDisabled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	const (
		clusterName    = "c1"
		machineSetName = "ms1"
		machineID      = "m1"
	)

	clusterMachine := omni.NewClusterMachine(machineID)
	clusterMachine.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, machineSetName)

	machineSet := omni.NewMachineSet(machineSetName)

	// cluster-level patch that should be applied
	enabled := omni.NewConfigPatch("cluster-enabled")
	enabled.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	// cluster-level patch that is disabled and must be skipped
	disabled := omni.NewConfigPatch("cluster-disabled")
	disabled.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	disabled.Metadata().Labels().Set(omni.LabelDisabled, "")

	// machine-level patch that should be applied
	machineLevel := omni.NewConfigPatch("machine-enabled")
	machineLevel.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	machineLevel.Metadata().Labels().Set(omni.LabelMachine, machineID)

	// machine-level patch that is disabled and must be skipped
	machineLevelDisabled := omni.NewConfigPatch("machine-disabled")
	machineLevelDisabled.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	machineLevelDisabled.Metadata().Labels().Set(omni.LabelMachine, machineID)
	machineLevelDisabled.Metadata().Labels().Set(omni.LabelDisabled, "")

	for _, res := range []resource.Resource{enabled, disabled, machineLevel, machineLevelDisabled} {
		require.NoError(t, st.Create(ctx, res))
	}

	helper, err := layeredresource.NewHelper[*omni.ConfigPatch](ctx, st)
	require.NoError(t, err)

	patches, err := helper.Get(clusterMachine, machineSet)
	require.NoError(t, err)

	ids := xslices.Map(patches, func(p *omni.ConfigPatch) string { return p.Metadata().ID() })

	require.ElementsMatch(t, []string{"cluster-enabled", "machine-enabled"}, ids)
}
