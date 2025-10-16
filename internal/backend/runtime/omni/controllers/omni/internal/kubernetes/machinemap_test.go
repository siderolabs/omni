// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/kubernetes"
)

func TestNewMachineMap(t *testing.T) {
	t.Parallel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	ctx := t.Context()

	cpNodes := []string{"cp1", "cp2"}
	workerNodes := []string{"worker1", "worker2", "worker3"}
	lockedNodes := []string{"worker2"}

	machineID := func(name string) string {
		return hex.EncodeToString(sha256.New().Sum([]byte(name)))
	}

	for _, node := range cpNodes {
		cm := omni.NewClusterMachineStatus(resources.DefaultNamespace, machineID(node))
		cm.Metadata().Labels().Set(omni.LabelCluster, "cluster1")
		cm.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

		cm.Metadata().Labels().Set(omni.ClusterMachineStatusLabelNodeName, node)

		require.NoError(t, st.Create(ctx, cm))
	}

	for _, node := range workerNodes {
		cm := omni.NewClusterMachineStatus(resources.DefaultNamespace, machineID(node))
		cm.Metadata().Labels().Set(omni.LabelCluster, "cluster1")
		cm.Metadata().Labels().Set(omni.LabelWorkerRole, "")

		cm.Metadata().Labels().Set(omni.ClusterMachineStatusLabelNodeName, node)

		if slices.Contains(lockedNodes, node) {
			cm.Metadata().Annotations().Set(omni.MachineLocked, "")
		}

		require.NoError(t, st.Create(ctx, cm))
	}

	cluster := omni.NewCluster(resources.DefaultNamespace, "cluster1")

	machineMap, err := kubernetes.NewMachineMap(ctx, st, cluster)
	require.NoError(t, err)

	assert.Equal(t, xslices.ToMap(cpNodes, func(n string) (string, string) { return n, machineID(n) }), machineMap.ControlPlanes)
	assert.Equal(t, xslices.ToMap(workerNodes, func(n string) (string, string) { return n, machineID(n) }), machineMap.Workers)
	assert.Equal(t, xslices.ToMap(lockedNodes, func(n string) (string, string) { return n, machineID(n) }), machineMap.Locked)
}
