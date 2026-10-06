// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	talosutils "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
)

func TestApplyMayHaveLanded(t *testing.T) {
	t.Parallel()

	assert.True(t, talosutils.ApplyMayHaveLanded(status.Error(codes.DeadlineExceeded, "no answer")))
	assert.True(t, talosutils.ApplyMayHaveLanded(status.Error(codes.Canceled, "canceled")))
	assert.True(t, talosutils.ApplyMayHaveLanded(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)))

	assert.False(t, talosutils.ApplyMayHaveLanded(status.Error(codes.Unavailable, "connection refused")))
	assert.False(t, talosutils.ApplyMayHaveLanded(status.Error(codes.InvalidArgument, "bad config")))
	assert.False(t, talosutils.ApplyMayHaveLanded(errors.New("something else")))
}

func TestCheckBeforeApply(t *testing.T) {
	t.Parallel()

	readErr := errors.New("unreachable")

	for _, tt := range []struct {
		read         func(context.Context) (string, error)
		active       func(context.Context) (bool, error)
		name         string
		expectBootID string
		wantBootID   string
		mode         machine.ApplyConfigurationRequest_Mode
		wantErr      bool
	}{
		{
			name: "plain apply does not read",
			read: func(context.Context) (string, error) { return "", readErr },
			mode: machine.ApplyConfigurationRequest_AUTO,
		},
		{
			name:       "try records the boot ID",
			read:       func(context.Context) (string, error) { return "boot-1", nil },
			mode:       machine.ApplyConfigurationRequest_TRY,
			wantBootID: "boot-1",
		},
		{
			name:    "try fails when the read fails",
			read:    func(context.Context) (string, error) { return "", readErr },
			mode:    machine.ApplyConfigurationRequest_TRY,
			wantErr: true,
		},
		{
			name:    "try fails on an empty boot ID",
			read:    func(context.Context) (string, error) { return "", nil },
			mode:    machine.ApplyConfigurationRequest_TRY,
			wantErr: true,
		},
		{
			name:         "confirm on the same boot",
			read:         func(context.Context) (string, error) { return "boot-1", nil },
			mode:         machine.ApplyConfigurationRequest_NO_REBOOT,
			expectBootID: "boot-1",
			wantBootID:   "boot-1",
		},
		{
			name:         "confirm without the tried config active",
			read:         func(context.Context) (string, error) { return "boot-1", nil },
			active:       func(context.Context) (bool, error) { return false, nil },
			mode:         machine.ApplyConfigurationRequest_NO_REBOOT,
			expectBootID: "boot-1",
			wantErr:      true,
		},
		{
			name:         "confirm after a reboot",
			read:         func(context.Context) (string, error) { return "boot-2", nil },
			mode:         machine.ApplyConfigurationRequest_NO_REBOOT,
			expectBootID: "boot-1",
			wantErr:      true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			active := tt.active
			if active == nil {
				active = func(context.Context) (bool, error) { return true, nil }
			}

			bootID, err := talosutils.CheckBeforeApply(t.Context(), tt.read, active, tt.mode, tt.expectBootID)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantBootID, bootID)
		})
	}
}

func TestSameConfig(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte("version: v1alpha1\nmachine:\n  type: worker\n  sysctls:\n    net.core.somaxconn: \"1\"\n"))
	require.NoError(t, err)

	same, err := talosutils.SameConfig(provider, []byte("# a comment\nmachine:\n  sysctls:\n    net.core.somaxconn: \"1\"\n  type: worker\nversion: v1alpha1\n"))
	require.NoError(t, err)
	assert.True(t, same, "formatting and comments must not matter")

	same, err = talosutils.SameConfig(provider, []byte("version: v1alpha1\nmachine:\n  type: worker\n  sysctls:\n    net.core.somaxconn: \"2\"\n"))
	require.NoError(t, err)
	assert.False(t, same)
}
