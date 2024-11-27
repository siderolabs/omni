// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"

	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jhump/grpctunnel"
	"github.com/jhump/grpctunnel/tunnelpb"
	"google.golang.org/grpc"
)

type tunnelServer struct {
	handler *grpctunnel.TunnelServiceHandler
}

func (t *tunnelServer) register(server grpc.ServiceRegistrar) {
	service := t.handler.Service()

	tunnelpb.RegisterTunnelServiceServer(server, service)
}

func (t *tunnelServer) gateway(context.Context, *gateway.ServeMux, string, []grpc.DialOption) error {
	// no-op - don't register a gateway for the tunnel service
	return nil
}
