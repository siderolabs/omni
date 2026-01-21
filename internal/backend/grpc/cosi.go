// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// COSIResourceServer implements installation of COSI resource API server.
type COSIResourceServer struct {
	State state.State
}

func (s *COSIResourceServer) register(srv grpc.ServiceRegistrar) {
	v1alpha1.RegisterStateServer(srv, server.NewState(s.State))
}

func (s *COSIResourceServer) gateway(ctx context.Context, mux *gateway.ServeMux, address string, opts []grpc.DialOption) error {
	return v1alpha1.RegisterStateHandlerFromEndpoint(ctx, mux, address, opts)
}
