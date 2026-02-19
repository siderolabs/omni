// Copyright (c) 2026 Sidero Labs, Inc.
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

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/grpc-proxy/proxy"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
)

type testNodeResolver struct{}

func (t *testNodeResolver) Resolve(_, _ string) (dns.Info, error) {
	return dns.Info{}, nil
}

func TestTalosBackendRoles(t *testing.T) {
	// Start mock server.
	const serverEndpoint = "127.0.0.1:10501"

	serverCloser := startTestServer(must.Value((&net.ListenConfig{}).Listen(t.Context(), "tcp", serverEndpoint))(t))

	t.Cleanup(func() { require.NoError(t, serverCloser()) })

	// Start mock proxy.
	const proxyEndpoint = "127.0.0.1:10500"

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Second)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	logger := zaptest.NewLogger(t)
	st, err := omniruntime.NewTestState(logger)
	require.NoError(t, err)

	grpcProxy, err := makeGRPCProxy(ctx, proxyEndpoint, serverEndpoint, st.Default())
	require.NoError(t, err)

	g.Go(grpcProxy)

	t.Cleanup(func() { require.NoError(t, g.Wait()) })

	conn := must.Value(grpc.NewClient(proxyEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials())))(t)
	rebootResult := must.Value(machine.NewMachineServiceClient(conn).Reboot(ctx, &machine.RebootRequest{Mode: 0}))(t)
	require.NotNil(t, rebootResult)

	hostnameResult := must.Value(machine.NewMachineServiceClient(conn).Hostname(ctx, &emptypb.Empty{}))(t)
	require.Equal(t, "talos-machine", hostnameResult.Messages[0].Hostname)
}

func TestTalosBackendHeaderDeletion(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	st, err := omniruntime.NewTestState(logger)
	require.NoError(t, err)

	// grpc.NewClient is lazy — no actual connection is made here.
	conn, err := dial("127.0.0.1:10501")
	require.NoError(t, err)

	newBackend := func() *router.TalosBackend {
		return router.NewTalosBackend(
			"test-backend",
			"test-cluster",
			&testNodeResolver{},
			conn,
			false,
			func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
				return handler(ctx, req)
			},
			st.Default(),
		)
	}

	t.Run("single node — both headers stripped", func(t *testing.T) {
		t.Parallel()

		incomingMD := metadata.Pairs("node", "some-node", "cluster", "test-cluster")
		ctx := metadata.NewIncomingContext(t.Context(), incomingMD)

		outCtx, _, err := newBackend().GetConnection(ctx, machine.MachineService_Hostname_FullMethodName)
		require.NoError(t, err)

		outMD, ok := metadata.FromOutgoingContext(outCtx)
		require.True(t, ok)
		require.Empty(t, outMD.Get("node"), "node header must be stripped for direct connection")
		require.Empty(t, outMD.Get("nodes"), "nodes header must be stripped for single-node call")
	})

	t.Run("single nodes header — node stripped, nodes preserved for apid loopback", func(t *testing.T) {
		t.Parallel()

		incomingMD := metadata.Pairs("nodes", "node-a", "cluster", "test-cluster")
		ctx := metadata.NewIncomingContext(t.Context(), incomingMD)

		outCtx, _, err := newBackend().GetConnection(ctx, machine.MachineService_Hostname_FullMethodName)
		require.NoError(t, err)

		outMD, ok := metadata.FromOutgoingContext(outCtx)
		require.True(t, ok)
		require.Empty(t, outMD.Get("node"), "node header must always be stripped")
		require.NotEmpty(t, outMD.Get("nodes"), "nodes header must be preserved even for single entry so apid sets Metadata.Hostname")
	})

	t.Run("multiple nodes — node stripped, nodes preserved for apid fan-out", func(t *testing.T) {
		t.Parallel()

		incomingMD := metadata.Pairs("nodes", "node-a,node-b", "cluster", "test-cluster")
		ctx := metadata.NewIncomingContext(t.Context(), incomingMD)

		outCtx, _, err := newBackend().GetConnection(ctx, machine.MachineService_Hostname_FullMethodName)
		require.NoError(t, err)

		outMD, ok := metadata.FromOutgoingContext(outCtx)
		require.True(t, ok)
		require.Empty(t, outMD.Get("node"), "node header must always be stripped")
		require.NotEmpty(t, outMD.Get("nodes"), "nodes header must be preserved so Talos apid can handle One2Many fan-out")
	})
}

func makeGRPCProxy(ctx context.Context, endpoint, serverEndpoint string, st state.State) (func() error, error) {
	grpcProxyServer := router.NewServer(&testDirector{serverEndpoint: serverEndpoint, omniState: st})

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", endpoint)
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
	omniState      state.State
	serverEndpoint string
}

func (t *testDirector) Director(context.Context, string) (proxy.Mode, []proxy.Backend, error) {
	conn, err := dial(t.serverEndpoint)
	if err != nil {
		return 0, nil, err
	}

	backend := router.NewTalosBackend(
		"test-backend",
		"test-backend",
		&testNodeResolver{},
		conn,
		false,
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
			return handler(ctx, req)
		},
		t.omniState,
	)

	return proxy.One2One, []proxy.Backend{backend}, nil
}

func dial(serverEndpoint string) (*grpc.ClientConn, error) {
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
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(proxy.Codec())),
	}

	return grpc.NewClient(serverEndpoint, opts...)
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
