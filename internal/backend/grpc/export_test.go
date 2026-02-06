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

//nolint:revive
func NewManagementServer(st state.State, imageFactoryClient *imagefactory.Client, logger *zap.Logger,
	enableBreakGlassConfigs bool, kubernetesRuntime KubernetesRuntime, talosconfigProvider TalosconfigProvider,
) *ManagementServer {
	return &ManagementServer{
		omniState:               st,
		imageFactoryClient:      imageFactoryClient,
		logger:                  logger,
		enableBreakGlassConfigs: enableBreakGlassConfigs,
		kubernetesRuntime:       kubernetesRuntime,
		talosconfigProvider:     talosconfigProvider,
	}
}

func NewAuthServer(st state.State, services config.Services, logger *zap.Logger) (*AuthServer, error) {
	return newAuthServer(st, services, logger)
}

func NewResourceServer(st state.State, runtimes map[string]runtime.Runtime, depGrapher DependencyGrapher) *ResourceServer {
	return newResourceServer(st, runtimes, depGrapher)
}

func GenerateDest(apiurl string) (string, error) {
	return generateDest(apiurl)
}
