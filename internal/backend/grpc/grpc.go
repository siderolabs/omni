// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package grpc implements gRPC server.
package grpc

import (
	"compress/gzip"
	"context"
	"fmt"
	"iter"
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
	auditor AuditLogger,
) iter.Seq2[ServiceServer, error] {
	dest, err := generateDest(config.Config.Services.API.URL())
	if err != nil {
		return func(yield func(ServiceServer, error) bool) {
			yield(nil, fmt.Errorf("error generating destination: %w", err))
		}
	}

	servers := []ServiceServer{
		&ResourceServer{},
		&oidcServer{
			provider: oidcProvider,
		},
		newManagementServer(
			state,
			jwtSigningKeyProvider,
			logHandler,
			logger.With(logging.Component("management_server")),
			dnsService,
			imageFactoryClient,
			auditor,
			dest,
		),
		&authServer{
			state:  state,
			logger: logger.With(logging.Component("auth_server")),
		},
		&COSIResourceServer{
			State: cachedState,
		},
		&machineService{},
	}

	return func(yield func(ServiceServer, error) bool) {
		for _, server := range servers {
			if !yield(server, err) {
				break
			}
		}
	}
}

// RegisterGateway registers all routes and returns connection fwhich gRPC server should listen on.
func RegisterGateway(
	ctx context.Context,
	servers iter.Seq2[ServiceServer, error],
	registerTo *http.ServeMux,
	logger *zap.Logger,
) (*memconn.Transport, error) {
	marshaller := &gateway.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames:  true,
			UseEnumNumbers: true,
		},
	}
	runtimeMux := gateway.NewServeMux(
		gateway.WithMarshalerOption(gateway.MIMEWildcard, marshaller),
	)
	memtrans := memconn.NewTransport("gateway-conn")
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// we are proxying requests to ourselves, so we don't need to impose a limit
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
		grpc.WithContextDialer(func(dctx context.Context, _ string) (net.Conn, error) {
			return memtrans.DialContext(dctx)
		}),
		grpc.WithSharedWriteBuffer(true),
	}

	for srv, err := range servers {
		if err != nil {
			return nil, fmt.Errorf("error creating service server: %w", err)
		}

		err = srv.gateway(ctx, runtimeMux, memtrans.Address(), opts)
		if err != nil {
			return nil, fmt.Errorf("error registering gateway: %w", err)
		}
	}

	registerTo.Handle("/api/",
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

	return memtrans, nil
}

// NewServer creates new grpc server.
func NewServer(servers iter.Seq2[ServiceServer, error], options ...grpc.ServerOption) (*grpc.Server, error) {
	server := grpc.NewServer(options...)

	for srv, err := range servers {
		if err != nil {
			return nil, fmt.Errorf("error creating service server: %w", err)
		}

		srv.register(server)
	}

	return server, nil
}

type machineService struct{}

func (*machineService) register(grpc.ServiceRegistrar) {}

func (*machineService) gateway(ctx context.Context, runtimeMux *gateway.ServeMux, addr string, opts []grpc.DialOption) error {
	if err := machine.RegisterMachineServiceHandlerFromEndpoint(ctx, runtimeMux, addr, opts); err != nil {
		return fmt.Errorf("error registering gateway: %w", err)
	}

	return nil
}
