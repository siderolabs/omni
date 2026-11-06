// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/config/validations"
)

func TestEnsureNoMachinesBelowTalosVersionFloor(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		versions    map[string]string
		name        string
		agreedFloor string
		expectErr   bool
	}{
		{
			name: "no machines",
		},
		{
			name:        "machine below the agreed floor",
			versions:    map[string]string{"m1": "v1.4.3"},
			agreedFloor: "1.5.0",
		},
		{
			name:        "machine between the agreed floor and the floor",
			versions:    map[string]string{"m1": "v1.5.3"},
			agreedFloor: "1.5.0",
			expectErr:   true,
		},
		{
			name:        "machine below the floor agreed to",
			versions:    map[string]string{"m1": "v1.5.3"},
			agreedFloor: "1.6.0",
		},
		{
			name:      "machine below the floor",
			versions:  map[string]string{"m1": "v1.5.0"},
			expectErr: true,
		},
		{
			name:     "machine at the floor",
			versions: map[string]string{"m1": "v1.6.0"},
		},
		{
			name:     "machine below MinTalosVersion but above the floor",
			versions: map[string]string{"m1": "v1.9.0"},
		},
		{
			name:      "mix of below and above",
			versions:  map[string]string{"m1": "v1.5.0", "m2": "v1.11.0"},
			expectErr: true,
		},
		{
			name:     "machine with empty version is ignored",
			versions: map[string]string{"m1": ""},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)

			st := state.WrapCore(inmem.NewState())

			for id, version := range tt.versions {
				ms := omni.NewMachineStatus(id)
				ms.TypedSpec().Value.TalosVersion = version

				require.NoError(t, st.Create(ctx, ms))
			}

			err := validations.EnsureNoMachinesBelowTalosVersionFloor(ctx, st, tt.agreedFloor)
			if tt.expectErr {
				assert.ErrorContains(t, err, "--siderolink-disconnect-talos-below=1.6.0")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnsureNoNonImageFactoryMachines(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		machines  map[string]*specs.MachineStatusSpec_Schematic
		name      string
		expectErr bool
	}{
		{
			name:     "no machines",
			machines: nil,
		},
		{
			name: "valid schematic",
			machines: map[string]*specs.MachineStatusSpec_Schematic{
				"m1": {FullId: "abc123"},
			},
		},
		{
			name: "no schematic (not ready)",
			machines: map[string]*specs.MachineStatusSpec_Schematic{
				"m1": nil,
			},
		},
		{
			name: "invalid schematic",
			machines: map[string]*specs.MachineStatusSpec_Schematic{
				"m1": {Invalid: true},
			},
			expectErr: true,
		},
		{
			name: "mix of valid and invalid",
			machines: map[string]*specs.MachineStatusSpec_Schematic{
				"m1": {FullId: "abc123"},
				"m2": {Invalid: true},
			},
			expectErr: true,
		},
		{
			name: "agent mode is not counted",
			machines: map[string]*specs.MachineStatusSpec_Schematic{
				"m1": {InAgentMode: true, Invalid: true},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)

			st := state.WrapCore(inmem.NewState())

			for id, schematic := range tt.machines {
				ms := omni.NewMachineStatus(id)
				ms.TypedSpec().Value.Schematic = schematic

				require.NoError(t, st.Create(ctx, ms))
			}

			err := validations.EnsureNoNonImageFactoryMachines(ctx, st)
			if tt.expectErr {
				assert.ErrorContains(t, err, "provisioned without ImageFactory")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
