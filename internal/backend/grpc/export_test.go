// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
	"github.com/siderolabs/omni/internal/pkg/config"
)

type ManagementServer = managementServer

// AuditLogFollowBatchSize is exported for testing.
const AuditLogFollowBatchSize = auditLogFollowBatchSize

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
		lifecycleManager:    lifecycle.NewManager(logger, "test-factory", "ghcr.io/siderolabs/installer", nil, nil, nil),

		auditLogFollowLease: auditLogFollowDefaultLease,
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

// WithAuditLogFollowLease configures the stream lease of audit log follow streams on a test
// management server.
func WithAuditLogFollowLease(lease time.Duration) ManagementServerOption {
	return func(server *ManagementServer) {
		server.auditLogFollowLease = lease
	}
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

// WithLifecycleManager overrides the lifecycle manager on a test management server.
func WithLifecycleManager(m LifecycleManager) ManagementServerOption {
	return func(server *ManagementServer) {
		server.lifecycleManager = m
	}
}

func NewAuthServer(st state.State, services config.Services, logger *zap.Logger) (*AuthServer, error) {
	return newAuthServer(st, services, logger)
}

func NewResourceServer(st state.State, runtimes map[string]runtime.Runtime, depGrapher DependencyGrapher) *ResourceServer {
	return newResourceServer(st, runtimes, depGrapher)
}
