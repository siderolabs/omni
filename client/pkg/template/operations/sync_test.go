// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations_test

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/template/operations"
)

type writeRecordingState struct {
	state.State

	writes []string
}

func (s *writeRecordingState) Create(ctx context.Context, r resource.Resource, opts ...state.CreateOption) error {
	s.writes = append(s.writes, resource.String(r))

	return s.State.Create(ctx, r, opts...)
}

func (s *writeRecordingState) Update(ctx context.Context, r resource.Resource, opts ...state.UpdateOption) error {
	s.writes = append(s.writes, resource.String(r))

	return s.State.Update(ctx, r, opts...)
}

const (
	lockOrderCluster = "Clusters.omni.sidero.dev(default/lock-order)"
	lockOrderWorker  = "947f7a99-0f46-4a9e-ab02-205f4e1b7009"
)

func lockOrderTemplate(locked, withPatch, withWorker bool) string {
	var sb strings.Builder

	sb.WriteString("kind: Cluster\nname: lock-order\nkubernetes:\n  version: v1.33.0\ntalos:\n  version: v1.10.0\n")

	if locked {
		sb.WriteString("locked: true\n")
	}

	if withPatch {
		sb.WriteString("patches:\n  - name: kubespan\n    inline:\n      machine:\n        network:\n          kubespan:\n            enabled: true\n")
	}

	sb.WriteString("---\nkind: ControlPlane\nmachines:\n  - 4b9797ae-55d7-4717-a19e-b322a2d83d39\n")

	if withWorker {
		sb.WriteString("---\nkind: Workers\nmachines:\n  - " + lockOrderWorker + "\n")
	}

	return sb.String()
}

func syncLockOrderTemplate(t *testing.T, st state.State, tmpl string) {
	require.NoError(t, operations.SyncTemplate(t.Context(), strings.NewReader(tmpl), io.Discard, st, operations.SyncOptions{}, nil))
}

func TestSyncTemplate_ClusterLockWrittenFirst(t *testing.T) {
	t.Run("lock", func(t *testing.T) {
		st := &writeRecordingState{State: state.WrapCore(inmem.NewState())}

		syncLockOrderTemplate(t, st, lockOrderTemplate(false, false, false))

		st.writes = nil

		syncLockOrderTemplate(t, st, lockOrderTemplate(true, true, false))

		require.NotEmpty(t, st.writes)
		assert.Equal(t, lockOrderCluster, st.writes[0], "the lock must be written before the new config patch")
	})

	t.Run("unlock", func(t *testing.T) {
		st := &writeRecordingState{State: state.WrapCore(inmem.NewState())}

		syncLockOrderTemplate(t, st, lockOrderTemplate(false, false, false))
		syncLockOrderTemplate(t, st, lockOrderTemplate(true, false, false))

		st.writes = nil

		syncLockOrderTemplate(t, st, lockOrderTemplate(false, false, true))

		require.NotEmpty(t, st.writes)
		assert.Equal(t, lockOrderCluster, st.writes[0], "the unlock must be written before the new machine set")
		assert.True(t, slices.ContainsFunc(st.writes, func(w string) bool { return strings.Contains(w, lockOrderWorker) }))
	})
}

func TestDiffTemplate_LockedCluster(t *testing.T) {
	st := state.WrapCore(inmem.NewState())

	syncLockOrderTemplate(t, st, lockOrderTemplate(false, false, false))
	syncLockOrderTemplate(t, st, lockOrderTemplate(true, false, false))

	var out strings.Builder

	require.NoError(t, operations.DiffTemplate(t.Context(), strings.NewReader(lockOrderTemplate(true, false, true)), &out, st, nil))
	assert.Contains(t, out.String(), lockOrderWorker)
}
