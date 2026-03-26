// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	commonOmni "github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	talosruntime "github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestMachinePowerOffUsesInternalContextAfterACLAuthorization(t *testing.T) {
	const (
		clusterID  = "cluster-1"
		identityID = "user@example.com"
		machineID  = "machine-1"
	)

	st := newManagementPowerTestState(t, clusterID, identityID, machineID)
	ctx := managementPowerTestContext(t.Context(), identityID, role.Reader)

	talosRuntime := &capturingTalosRuntime{}
	auditor := &capturingAuditLogger{err: errors.New("stop before shutdown")}

	server := grpcomni.NewManagementServer(
		st,
		nil,
		zaptest.NewLogger(t),
		false,
		nil,
		nil,
		grpcomni.WithTalosRuntime(talosRuntime),
		grpcomni.WithAuditLogger(auditor),
	)

	_, err := server.MachinePowerOff(ctx, &management.MachinePowerOffRequest{MachineId: machineID})
	require.ErrorIs(t, err, auditor.err)

	require.True(t, talosRuntime.machineCtx.captured)
	require.True(t, talosRuntime.machineCtx.internal)
	require.True(t, talosRuntime.machineCtx.hasRole)
	require.Equal(t, role.Operator, talosRuntime.machineCtx.role)

	require.True(t, auditor.ctx.captured)
	require.False(t, auditor.ctx.internal)
	require.True(t, auditor.ctx.hasRole)
	require.Equal(t, role.Operator, auditor.ctx.role)
	require.Equal(t, "machine.MachineService/Shutdown", auditor.fullMethod)
	require.Equal(t, clusterID, auditor.clusterID)
	require.Equal(t, machineID, auditor.nodeID)
}

func newManagementPowerTestState(t *testing.T, clusterID, identityID, machineID string) state.State {
	runtimeState, err := omniruntime.NewTestState(zaptest.NewLogger(t))
	require.NoError(t, err)

	st := runtimeState.Default()
	ctx := actor.MarkContextAsInternalActor(t.Context())

	clusterMachine := omnires.NewClusterMachine(machineID)
	clusterMachine.Metadata().Labels().Set(omnires.LabelCluster, clusterID)
	require.NoError(t, st.Create(ctx, clusterMachine))

	require.NoError(t, st.Create(ctx, siderolinkres.NewLink(machineID, &specs.SiderolinkSpec{})))
	require.NoError(t, st.Create(ctx, authres.NewIdentity(identityID)))

	accessPolicy := authres.NewAccessPolicy()
	accessPolicy.TypedSpec().Value.Rules = []*specs.AccessPolicyRule{
		{
			Users:    []string{identityID},
			Clusters: []string{clusterID},
			Role:     string(role.Operator),
		},
	}

	require.NoError(t, st.Create(ctx, accessPolicy))

	return st
}

func managementPowerTestContext(ctx context.Context, identityID string, r role.Role) context.Context {
	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: true})
	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: identityID})
	ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: r})

	return ctx
}

type capturingTalosRuntime struct {
	machineCtx capturedContext
}

func (c *capturingTalosRuntime) GetTalosconfigRaw(*commonOmni.Context, string) ([]byte, error) {
	return []byte{}, nil
}

func (c *capturingTalosRuntime) GetClientForCluster(context.Context, string) (*talosruntime.Client, error) {
	return nil, errors.New("not implemented")
}

func (c *capturingTalosRuntime) GetClientForMachine(ctx context.Context, _ string) (*talosruntime.Client, error) {
	c.machineCtx = captureContext(ctx)

	return talosruntime.NewClient(nil, "", ""), nil
}

type capturingAuditLogger struct {
	err        error
	fullMethod string
	clusterID  string
	nodeID     string
	ctx        capturedContext
}

func (c *capturingAuditLogger) Reader(context.Context, auditlog.ReadFilters) (auditlog.Reader, error) {
	return nil, errors.New("not implemented")
}

func (c *capturingAuditLogger) AuditTalosAccess(ctx context.Context, fullMethodName, clusterID, nodeID string) error {
	c.ctx = captureContext(ctx)
	c.fullMethod = fullMethodName
	c.clusterID = clusterID
	c.nodeID = nodeID

	return c.err
}

type capturedContext struct {
	role     role.Role
	captured bool
	internal bool
	hasRole  bool
}

func captureContext(ctx context.Context) capturedContext {
	roleVal, ok := ctxstore.Value[auth.RoleContextKey](ctx)

	return capturedContext{
		captured: true,
		internal: actor.ContextIsInternalActor(ctx),
		hasRole:  ok,
		role:     roleVal.Role,
	}
}
