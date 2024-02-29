// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/grpc-proxy/proxy"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
)

type testNodeResolver struct{}

func (t *testNodeResolver) Resolve(_, node string) dns.Info {
	return dns.Info{Address: node}
}

func TestTalosBackendRoles(t *testing.T) {
	// Start mock server.
	const serverEndpoint = "127.0.0.1:10501"
	serverCloser := startTestServer(must.Value(net.Listen("tcp", serverEndpoint))(t))

	t.Cleanup(func() { require.NoError(t, serverCloser()) })

	// Start mock proxy.
	const proxyEndpoint = "127.0.0.1:10500"

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	grpcProxy, err := makeGRPCProxy(ctx, proxyEndpoint, serverEndpoint)
	require.NoError(t, err)

	g.Go(grpcProxy)

	t.Cleanup(func() { require.NoError(t, g.Wait()) })

	conn := must.Value(grpc.DialContext(ctx, proxyEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials())))(t)
	rebootResult := must.Value(machine.NewMachineServiceClient(conn).Reboot(ctx, &machine.RebootRequest{Mode: 0}))(t)
	require.NotNil(t, rebootResult)

	hostnameResult := must.Value(machine.NewMachineServiceClient(conn).Hostname(ctx, &emptypb.Empty{}))(t)
	require.Equal(t, "talos-machine", hostnameResult.Messages[0].Hostname)
}

func makeGRPCProxy(ctx context.Context, endpoint, serverEndpoint string) (func() error, error) {
	grpcProxyServer := router.NewServer(&testDirector{serverEndpoint: serverEndpoint})

	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", endpoint, err)
	}

	return func() error {
		errCh := make(chan error, 1)

		go func() { errCh <- grpcProxyServer.Serve(lis) }()

		if err, ok := recvContext(ctx, errCh); ok {
			return fmt.Errorf("error running grpc proxy: %w", err)
		}

		grpcProxyServer.Stop()

		if srvErr := <-errCh; srvErr != nil && !errors.Is(srvErr, grpc.ErrServerStopped) {
			return fmt.Errorf("error stopping grpc proxy: %w", srvErr)
		}

		return nil
	}, nil
}

type testDirector struct {
	serverEndpoint string
}

func (t *testDirector) Director(ctx context.Context, _ string) (proxy.Mode, []proxy.Backend, error) {
	conn, err := dial(ctx, t.serverEndpoint)
	if err != nil {
		return 0, nil, err
	}

	backend := router.NewTalosBackend(
		"test-backend",
		&testNodeResolver{},
		conn,
		false,
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			return handler(ctx, req)
		},
	)

	return proxy.One2One, []proxy.Backend{backend}, nil
}

func dial(ctx context.Context, serverEndpoint string) (*grpc.ClientConn, error) {
	backoffConfig := backoff.DefaultConfig
	backoffConfig.MaxDelay = 15 * time.Second

	creds := insecure.NewCredentials()
	opts := []grpc.DialOption{
		grpc.WithInitialWindowSize(65535 * 32),
		grpc.WithInitialConnWindowSize(65535 * 16),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoffConfig,
			MinConnectTimeout: 20 * time.Second,
		}),
		grpc.WithTransportCredentials(creds),
		grpc.WithCodec(proxy.Codec()), //nolint:staticcheck
		grpc.WithBlock(),
	}

	return grpc.DialContext(ctx, serverEndpoint, opts...)
}

func startTestServer(lis net.Listener) (closer func() error) {
	server := grpc.NewServer()
	machine.RegisterMachineServiceServer(server, &testServer{})

	errCh := make(chan error, 1)

	go func() { errCh <- server.Serve(lis) }()

	return func() error {
		if err, ok := tryRecv(errCh); ok {
			return err
		}

		server.Stop()

		return <-errCh
	}
}

type testServer struct {
	machine.UnimplementedMachineServiceServer
}

func (ts *testServer) Reboot(ctx context.Context, _ *machine.RebootRequest) (*machine.RebootResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no metadata")
	}

	if got := md.Get("talos-role")[0]; got != string(role.Admin) {
		return nil, fmt.Errorf("unexpected role: %s", got)
	}

	return &machine.RebootResponse{}, nil
}

func (ts *testServer) Hostname(ctx context.Context, _ *emptypb.Empty) (*machine.HostnameResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no metadata")
	}

	if got := md.Get("talos-role")[0]; got != string(role.Reader) {
		return nil, fmt.Errorf("unexpected role: %s", got)
	}

	return &machine.HostnameResponse{
		Messages: []*machine.Hostname{
			{
				Hostname: "talos-machine",
			},
		},
	}, nil
}

func tryRecv[T any](ch <-chan T) (T, bool) {
	select {
	case v := <-ch:
		return v, true
	default:
		var zero T

		return zero, false
	}
}

func recvContext[T any](ctx context.Context, ch <-chan T) (T, bool) {
	select {
	case <-ctx.Done():
		return *new(T), false
	case v := <-ch:
		return v, true
	}
}
