// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/pkg/config"
)

type ManagementServer = managementServer

type AuthServer = authServer

// ManagementServerOption configures a test management server.
type ManagementServerOption func(*ManagementServer)

func NewManagementServer(st state.State, imageFactoryClient *imagefactory.Client, logger *zap.Logger,
	enableBreakGlassConfigs bool, kubernetesRuntime KubernetesRuntime, talosconfigProvider TalosconfigProvider,
	opts ...ManagementServerOption,
) *ManagementServer {
	server := &ManagementServer{
		omniState:           st,
		imageFactoryClient:  imageFactoryClient,
		logger:              logger,
		cfg:                 &config.Params{Features: config.Features{EnableBreakGlassConfigs: new(enableBreakGlassConfigs)}},
		kubernetesRuntime:   kubernetesRuntime,
		talosconfigProvider: talosconfigProvider,
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

// WithTalosRuntime configures the Talos runtime on a test management server.
func WithTalosRuntime(talosRuntime TalosRuntime) ManagementServerOption {
	return func(server *ManagementServer) {
		server.talosRuntime = talosRuntime
	}
}

// WithAuditLogger configures the audit logger on a test management server.
func WithAuditLogger(auditor AuditLogger) ManagementServerOption {
	return func(server *ManagementServer) {
		server.auditor = auditor
	}
}

// CheckMaintenanceLifecycleTalosVersion exposes the in-memory version gate for testing.
func CheckMaintenanceLifecycleTalosVersion(version string) error {
	return checkMaintenanceLifecycleTalosVersion(version)
}

// ClaimMaintenanceLifecycleSlot pre-claims the in-flight slot for a machine, simulating a concurrent lifecycle operation.
func (s *ManagementServer) ClaimMaintenanceLifecycleSlot(machineID string) {
	s.maintenanceLifecycleInFlight.Store(machineID, struct{}{})
}

func NewAuthServer(st state.State, services config.Services, logger *zap.Logger) (*AuthServer, error) {
	return newAuthServer(st, services, logger)
}

func NewResourceServer(st state.State, runtimes map[string]runtime.Runtime, depGrapher DependencyGrapher) *ResourceServer {
	return newResourceServer(st, runtimes, depGrapher)
}
