// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

func newManagerForTest(t *testing.T) *lifecycle.Manager {
	t.Helper()

	c, err := imagefactory.NewClient("factory.talos.dev", "", "")
	require.NoError(t, err)

	return lifecycle.NewManager(zapNop(t), imagefactory.NewClients(
		state.WrapCore(namespaced.NewState(inmem.Build)),
		c,
	), "ghcr.io/siderolabs/installer", nil, nil)
}

func TestCheckTalosVersion(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

	for _, tc := range []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "supported 1.13", version: "1.13.0", wantErr: false},
		{name: "supported with v prefix", version: "v1.13.4", wantErr: false},
		{name: "supported 1.14 alpha", version: "1.14.0-alpha.1", wantErr: false},
		{name: "supported next major", version: "2.0.0", wantErr: false},
		{name: "unsupported 1.12", version: "1.12.5", wantErr: true},
		{name: "unsupported 1.11", version: "1.11.0", wantErr: true},
		{name: "unparseable", version: "garbage", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := m.SupportsLifecycleManagement(tc.version)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "install", lifecycle.KindInstall.String())
	assert.Equal(t, "upgrade", lifecycle.KindUpgrade.String())
}

func TestInFlightGuard(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"), "first acquire should succeed")
	require.False(t, m.AcquireForTest("machine-1"), "second acquire should be rejected while held")
	require.True(t, m.AcquireForTest("machine-2"), "a different machine is independent")

	m.ReleaseForTest("machine-1")
	require.True(t, m.AcquireForTest("machine-1"), "acquire should succeed again after release")
}

func TestRunRejectsWhenInFlight(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

	require.True(t, m.AcquireForTest("machine-1"))

	// MachineStatus is never touched: Run returns at the in-flight guard before building the image.
	err := m.Run(t.Context(), lifecycle.Operation{MachineID: "machine-1", Kind: lifecycle.KindUpgrade})
	require.ErrorIs(t, err, lifecycle.ErrAlreadyInFlight)
}

func TestRunReleasesSlotOnFailure(t *testing.T) {
	t.Parallel()

	m := newManagerForTest(t)

	// No platform metadata → the flow fails fast in buildInstallImage.
	ms := omni.NewMachineStatus("machine-1")
	ms.TypedSpec().Value.TalosVersion = "1.13.1"

	err := m.Run(t.Context(), lifecycle.Operation{
		MachineID:     "machine-1",
		MachineStatus: ms,
		Kind:          lifecycle.KindUpgrade,
	})
	require.Error(t, err)
	assert.NotErrorIs(t, err, lifecycle.ErrAlreadyInFlight)

	// The slot was released, so the machine can be acquired again.
	require.True(t, m.AcquireForTest("machine-1"))
}

func zapNop(t *testing.T) *zap.Logger {
	t.Helper()

	return zaptest.NewLogger(t)
}
