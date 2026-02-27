// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package kms implements Omni KMS server for disk encryption keys.
package kms

import (
	"context"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/kms-client/api/kms"
	"github.com/siderolabs/kms-client/pkg/server"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// Manager handles disk encryption keys seal/unseal operations using COSI state and existing machines.
type Manager struct {
	state  state.State
	logger *zap.Logger
}

// NewManager creates new KMS key manager.
func NewManager(state state.State, logger *zap.Logger) *Manager {
	return &Manager{
		state:  state,
		logger: logger,
	}
}

// Register manager to handle gRPC requests for seal and unseal.
func (m *Manager) Register(srv *grpc.Server) {
	grpcServer := server.NewServer(m.logger, func(ctx context.Context, nodeUUID string) ([]byte, error) {
		ctx = actor.MarkContextAsInternalActor(ctx)

		res, err := safe.StateGet[*omni.ClusterMachineEncryptionKey](ctx, m.state, omni.NewClusterMachineEncryptionKey(nodeUUID).Metadata())
		if err != nil {
			m.logger.Error("failed to get encryption key for node", zap.String("machine", nodeUUID), zap.Error(err))

			return nil, err
		}

		return res.TypedSpec().Value.Data, nil
	})

	kms.RegisterKMSServiceServer(srv, grpcServer)
}
