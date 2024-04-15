// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/imagefactory"
)

type ManagementServer = managementServer

//nolint:revive
func NewManagementServer(st state.State, imageFactoryClient *imagefactory.Client, logger *zap.Logger) *ManagementServer {
	return &ManagementServer{
		omniState:          st,
		imageFactoryClient: imageFactoryClient,
		logger:             logger,
	}
}

func GenerateDest(apiurl string) (string, error) {
	return generateDest(apiurl)
}
