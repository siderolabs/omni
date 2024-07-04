// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package grpc implements gRPC server.
package grpc

import (
	"compress/gzip"
	"context"
	"fmt"
	"math"
	"net"
	"net/http"

	"github.com/cosi-project/runtime/pkg/state"
	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/omni/client/api/talos/machine"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/monitoring"
	"github.com/siderolabs/omni/internal/memconn"
	"github.com/siderolabs/omni/internal/pkg/compress"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// ServiceServer is a gRPC service server.
type ServiceServer interface {
	register(grpc.ServiceRegistrar)
	gateway(context.Context, *gateway.ServeMux, string, []grpc.DialOption) error
}

// MakeServiceServers creates a list of service servers.
func MakeServiceServers(
	state state.State,
	cachedState state.State,
	logHandler *siderolink.LogHandler,
	oidcProvider OIDCProvider,
	jwtSigningKeyProvider JWTSigningKeyProvider,
	dnsService *dns.Service,
	imageFactoryClient *imagefactory.Client,
	logger *zap.Logger,
) ([]ServiceServer, error) {
	dest, err := generateDest(config.Config.APIURL)
	if err != nil {
		return nil, err
	}

	return []ServiceServer{
		&ResourceServer{},
		&oidcServer{
			provider: oidcProvider,
		},
		&managementServer{
			logHandler:            logHandler,
			omniconfigDest:        dest,
			omniState:             state,
			dnsService:            dnsService,
			jwtSigningKeyProvider: jwtSigningKeyProvider,
			imageFactoryClient:    imageFactoryClient,
			logger:                logger.With(logging.Component("management_server")),
		},
		&authServer{
			state:  state,
			logger: logger.With(logging.Component("auth_server")),
		},
		&COSIResourceServer{
			State: cachedState,
		},
	}, nil
}

// New creates new grpc server and registers all routes.
func New(ctx context.Context, mux *http.ServeMux, servers []ServiceServer, transport *memconn.Transport, logger *zap.Logger, options ...grpc.ServerOption) (*grpc.Server, error) {
	server := grpc.NewServer(options...)

	for _, srv := range servers {
		srv.register(server)
	}

	marshaller := &gateway.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames:  true,
			UseEnumNumbers: true,
		},
	}
	runtimeMux := gateway.NewServeMux(
		gateway.WithMarshalerOption(gateway.MIMEWildcard, marshaller),
	)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// we are proxying requests to ourselves, so we don't need to impose a limit
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return transport.Dial()
		}),
		grpc.WithSharedWriteBuffer(true),
	}

	for _, srv := range servers {
		err := srv.gateway(ctx, runtimeMux, transport.Address(), opts)
		if err != nil {
			return nil, fmt.Errorf("error registering gateway: %w", err)
		}
	}

	if err := machine.RegisterMachineServiceHandlerFromEndpoint(ctx, runtimeMux, transport.Address(), opts); err != nil {
		return nil, fmt.Errorf("error registering gateway: %w", err)
	}

	mux.Handle("/api/",
		compress.Handler(
			monitoring.NewHandler(
				logging.NewHandler(
					http.StripPrefix("/api", runtimeMux),
					logger.With(zap.String("handler", "grpc_gateway")),
				),
				prometheus.Labels{"handler": "grpc_gateway"},
			),
			gzip.DefaultCompression,
		),
	)

	return server, nil
}
