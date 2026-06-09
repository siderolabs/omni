// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

func TestCheckMaintenanceLifecycleTalosVersion(t *testing.T) {
	for _, tt := range []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "supported", version: "v1.13.0", wantErr: false},
		{name: "supported without v prefix", version: "1.13.2", wantErr: false},
		{name: "supported newer minor", version: "v1.14.0", wantErr: false},
		{name: "supported pre-release", version: "v1.13.0-beta.1", wantErr: false},
		{name: "too old", version: "v1.12.5", wantErr: true},
		{name: "much older", version: "v1.9.0", wantErr: true},
		{name: "unparseable", version: "not-a-version", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := grpcomni.CheckMaintenanceLifecycleTalosVersion(tt.version)

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, codes.FailedPrecondition, status.Code(err))

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestMaintenanceLifecycleGuards(t *testing.T) {
	const (
		identityID = "user@example.com"
		machineID  = "machine-1"
	)

	for _, tt := range []struct {
		modify   func(*specs.MachineStatusSpec)
		req      *management.MaintenanceLifecycleRequest
		name     string
		setLabel string
		wantCode codes.Code
		create   bool
	}{
		{
			name: "missing machine id",
			req: &management.MaintenanceLifecycleRequest{
				Operation: management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
				Disk:      "/dev/sda",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "unspecified operation",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Disk:      "/dev/sda",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "install missing disk",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
				Version:   "1.13.0",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "upgrade with disk",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_UPGRADE,
				Version:   "1.13.0",
				Disk:      "/dev/sda",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "upgrade missing version",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_UPGRADE,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "machine not found",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
				Version:   "1.13.0",
				Disk:      "/dev/sda",
			},
			wantCode: codes.NotFound,
		},
		{
			name: "not in maintenance",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
				Version:   "1.13.0",
				Disk:      "/dev/sda",
			},
			create: true,
			modify: func(s *specs.MachineStatusSpec) {
				s.Maintenance = false
				s.TalosVersion = "v1.13.0"
			},
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "install on already installed",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
				Version:   "1.13.0",
				Disk:      "/dev/sda",
			},
			create:   true,
			setLabel: omnires.MachineStatusLabelInstalled,
			modify: func(s *specs.MachineStatusSpec) {
				s.Maintenance = true
				s.TalosVersion = "v1.13.0"
			},
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "upgrade on not installed",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_UPGRADE,
				Version:   "1.13.0",
			},
			create: true,
			modify: func(s *specs.MachineStatusSpec) {
				s.Maintenance = true
				s.TalosVersion = "v1.13.0"
			},
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "talos too old",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
				Version:   "1.13.0",
				Disk:      "/dev/sda",
			},
			create: true,
			modify: func(s *specs.MachineStatusSpec) {
				s.Maintenance = true
				s.TalosVersion = "v1.12.0"
			},
			wantCode: codes.FailedPrecondition,
		},
		{
			// Talos's compatibility matrix rejects multi-minor backwards jumps; this guard exercises
			// the delegation to that matrix (the check itself doesn't enforce its own minor rule).
			name: "target too many minors older than in-memory",
			req: &management.MaintenanceLifecycleRequest{
				MachineId: machineID,
				Operation: management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
				Version:   "1.10.0",
				Disk:      "/dev/sda",
			},
			create: true,
			modify: func(s *specs.MachineStatusSpec) {
				s.Maintenance = true
				s.TalosVersion = "v1.15.0"
			},
			wantCode: codes.FailedPrecondition,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := newMaintenanceLifecycleTestState(t, identityID, machineID, tt.create, tt.setLabel, tt.modify)

			server := grpcomni.NewManagementServer(st, nil, zaptest.NewLogger(t), false, nil, nil)

			ctx := managementPowerTestContext(t.Context(), identityID, role.Operator)

			err := server.MaintenanceLifecycle(tt.req, &fakeLifecycleStream{ctx: ctx})

			require.Error(t, err)
			require.Equal(t, tt.wantCode, status.Code(err), "unexpected status: %v", err)
		})
	}
}

func TestMaintenanceLifecycleInFlightGuard(t *testing.T) {
	const (
		identityID = "user@example.com"
		machineID  = "machine-1"
	)

	// fixture passes every precondition up through SchematicReady, so the in-flight guard is the next check.
	st := newMaintenanceLifecycleTestState(t, identityID, machineID, true, "", func(s *specs.MachineStatusSpec) {
		s.Maintenance = true
		s.TalosVersion = "v1.13.0"
		s.Schematic = &specs.MachineStatusSpec_Schematic{FullId: "schematic-full-id"}
		s.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{Platform: "metal"}
		s.SecurityState = &specs.SecurityState{}
	})

	// TalosVersion[1.13.0] with the requested target on its upgradable list — needed so the target
	// version check passes and the in-flight guard fires (preconditions are ordered earlier).
	talosVersion := omnires.NewTalosVersion("1.13.0")
	talosVersion.TypedSpec().Value.Version = "1.13.0"
	talosVersion.TypedSpec().Value.UpgradableTalosVersions = []string{"1.13.0"}
	require.NoError(t, st.Create(actor.MarkContextAsInternalActor(t.Context()), talosVersion))

	server := grpcomni.NewManagementServer(st, nil, zaptest.NewLogger(t), false, nil, nil)

	server.ClaimMaintenanceLifecycleSlot(machineID)

	ctx := managementPowerTestContext(t.Context(), identityID, role.Operator)

	err := server.MaintenanceLifecycle(
		&management.MaintenanceLifecycleRequest{
			MachineId: machineID,
			Operation: management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
			Version:   "1.13.0",
			Disk:      "/dev/sda",
		},
		&fakeLifecycleStream{ctx: ctx},
	)

	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "unexpected status: %v", err)
	require.Contains(t, status.Convert(err).Message(), "already in progress")
}

func newMaintenanceLifecycleTestState(
	t *testing.T,
	identityID, machineID string,
	createMachine bool,
	label string,
	modify func(*specs.MachineStatusSpec),
) state.State {
	runtimeState, err := omniruntime.NewTestState(zaptest.NewLogger(t))
	require.NoError(t, err)

	st := runtimeState.Default()
	ctx := actor.MarkContextAsInternalActor(t.Context())

	require.NoError(t, st.Create(ctx, authres.NewIdentity(identityID)))

	if createMachine {
		machineStatus := omnires.NewMachineStatus(machineID)

		if modify != nil {
			modify(machineStatus.TypedSpec().Value)
		}

		if label != "" {
			machineStatus.Metadata().Labels().Set(label, "")
		}

		require.NoError(t, st.Create(ctx, machineStatus))
	}

	return st
}

// fakeLifecycleStream is a minimal grpc.ServerStreamingServer for MaintenanceLifecycle guard tests, which return
// before any response is sent.
type fakeLifecycleStream struct {
	grpc.ServerStream
	ctx  context.Context //nolint:containedctx
	sent []*management.MaintenanceLifecycleResponse
}

func (f *fakeLifecycleStream) Context() context.Context { return f.ctx }

func (f *fakeLifecycleStream) Send(resp *management.MaintenanceLifecycleResponse) error {
	f.sent = append(f.sent, resp)

	return nil
}
