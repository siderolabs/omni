// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/grpc-proxy/proxy"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
)

const sensitiveType = "ApiCertificates.secrets.talos.dev"

// fakeNode records the redaction header of every request.
type fakeNode struct {
	address string
	modes   []string
	mu      sync.Mutex
}

// lastMode returns the redaction header of the last request.
func (n *fakeNode) lastMode() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.modes) == 0 {
		return ""
	}

	return n.modes[len(n.modes)-1]
}

// fakeMachine answers Version once per entry of redacts, with the entry as the ResourceRedaction feature. Without
// redacts, the fake node doesn't serve Version.
type fakeMachine struct {
	machine.UnimplementedMachineServiceServer

	redacts []bool
}

func (m *fakeMachine) Version(context.Context, *emptypb.Empty) (*machine.VersionResponse, error) {
	resp := &machine.VersionResponse{}

	for _, redacts := range m.redacts {
		resp.Messages = append(resp.Messages, &machine.Version{Features: &machine.FeaturesInfo{ResourceRedaction: redacts}})
	}

	return resp, nil
}

// startFakeNode serves the COSI resource API of a node, with a sensitive resource type registered, without any
// access control: as a node in maintenance mode does for a SideroLink peer, and as a node must not be relied on for.
func startFakeNode(ctx context.Context, t *testing.T, redacts ...bool) *fakeNode {
	t.Helper()

	st := state.WrapCore(inmem.NewState())
	require.NoError(t, registry.NewNamespaceRegistry(st).RegisterDefault(ctx))
	require.NoError(t, registry.NewResourceRegistry(st).RegisterDefault(ctx))

	rd, err := meta.NewResourceDefinition(meta.ResourceDefinitionSpec{
		Type:             sensitiveType,
		DefaultNamespace: "secrets",
		Sensitivity:      meta.Sensitive,
	})
	require.NoError(t, err)
	require.NoError(t, st.Create(ctx, rd))

	lis := must.Value((&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0"))(t)

	node := &fakeNode{}

	record := func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error { //nolint:contextcheck // a stream carries its own context
		md, _ := metadata.FromIncomingContext(ss.Context())

		node.mu.Lock()
		node.modes = append(node.modes, strings.Join(md.Get(redactable.Header), ","))
		node.mu.Unlock()

		return handler(srv, ss)
	}

	srv := grpc.NewServer(grpc.StreamInterceptor(record))
	v1alpha1.RegisterStateServer(srv, server.NewState(st))

	if redacts != nil {
		machine.RegisterMachineServiceServer(srv, &fakeMachine{redacts: redacts})
	}

	errCh := make(chan error, 1)

	go func() { errCh <- srv.Serve(lis) }()

	t.Cleanup(func() {
		srv.Stop()

		if err := <-errCh; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			require.NoError(t, err)
		}
	})

	node.address = lis.Addr().String()

	return node
}

// versionResolver resolves every node to a machine recorded as running talosVersion.
type versionResolver struct {
	talosVersion string
}

func (r versionResolver) Resolve(_, _ string) (dns.Info, error) {
	return dns.Info{TalosVersion: r.talosVersion}, nil
}

// guardedDirector proxies to the fake node over a connection guarded like the router's connections to the nodes.
type guardedDirector struct {
	omniState    state.State
	resolver     versionResolver
	nodeEndpoint string
}

func (d *guardedDirector) Director(context.Context, string) (proxy.Mode, []proxy.Backend, error) {
	conn, err := grpc.NewClient(d.nodeEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(proxy.Codec())),
		grpc.WithChainStreamInterceptor(router.SensitiveReadGuard()),
	)
	if err != nil {
		return 0, nil, err
	}

	backend := router.NewTalosBackend("maintenance", "", d.resolver, conn, operatorVerifier, d.omniState, nil)

	return proxy.One2One, []proxy.Backend{backend}, nil
}

// startGuardedProxy starts a guarded proxy to the fake node and returns a COSI client for it, like talosctl.
func startGuardedProxy(ctx context.Context, t *testing.T, nodeEndpoint string, resolver versionResolver) state.State {
	t.Helper()

	st, err := omniruntime.NewTestState(zaptest.NewLogger(t))
	require.NoError(t, err)

	proxyServer := router.NewServer(&guardedDirector{omniState: st.Default(), nodeEndpoint: nodeEndpoint, resolver: resolver})
	proxyLis := must.Value((&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0"))(t)

	proxyErr := make(chan error, 1)

	go func() { proxyErr <- proxyServer.Serve(proxyLis) }()

	t.Cleanup(func() {
		proxyServer.Stop()

		if err := <-proxyErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			require.NoError(t, err)
		}
	})

	conn := must.Value(grpc.NewClient(proxyLis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials())))(t)
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck

	return state.WrapCore(client.NewAdapter(v1alpha1.NewStateClient(conn), client.WithDisableWatchRetry()))
}

// TestSensitiveReadGuard checks that the COSI reads proxied to a node are refused for the sensitive resource types,
// and pass for the others, before they reach the node, whatever the node itself enforces.
func TestSensitiveReadGuard(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	// a node with redaction support, and a client that didn't ask for it
	fake := startFakeNode(ctx, t, true)
	node := startGuardedProxy(ctx, t, fake.address, versionResolver{talosVersion: "v1.15.0"})
	ctx = metadata.AppendToOutgoingContext(ctx, "node", "node-1")

	sensitive := resource.NewMetadata("secrets", sensitiveType, "api", resource.VersionUndefined)

	t.Run("non-sensitive reads pass through", func(t *testing.T) {
		rds, err := safe.StateListAll[*meta.ResourceDefinition](ctx, node)
		require.NoError(t, err)
		assert.NotZero(t, rds.Len())

		_, err = safe.StateGetByID[*meta.ResourceDefinition](ctx, node, "namespaces.meta.cosi.dev")
		require.NoError(t, err)
	})

	t.Run("sensitive get is refused", func(t *testing.T) {
		_, err := node.Get(ctx, sensitive)
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err), "%v", err)
	})

	t.Run("sensitive list is refused", func(t *testing.T) {
		_, err := node.List(ctx, resource.NewMetadata("secrets", sensitiveType, "", resource.VersionUndefined))
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err), "%v", err)
	})

	t.Run("sensitive watch is refused", func(t *testing.T) {
		events := make(chan state.Event, 1)

		err := node.Watch(ctx, sensitive, events)
		if err == nil {
			// the adapter reports the errors of an established watch as events
			select {
			case event := <-events:
				err = event.Error
			case <-ctx.Done():
				t.Fatal("no watch event")
			}
		}

		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err), "%v", err)
	})

	t.Run("unknown type is refused", func(t *testing.T) {
		_, err := node.Get(ctx, resource.NewMetadata("secrets", "Unknowns.secrets.talos.dev", "x", resource.VersionUndefined))
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err), "%v", err)
	})
}

// TestSensitiveReadGuardRedaction checks that sensitive reads pass only when the client asks for redaction and every
// node supports it, and that the nodes always get enforce.
func TestSensitiveReadGuardRedaction(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	sensitive := resource.NewMetadata("secrets", sensitiveType, "", resource.VersionUndefined)

	// talosVersion is what Omni recorded, redacts what the nodes answer
	for _, tc := range []struct {
		name         string
		talosVersion string
		header       string
		nodes        string
		mode         redactable.Mode
		redacts      []bool
		code         codes.Code
	}{
		{
			name: "accept on a node which redacts", talosVersion: "v1.15.0", header: "node", nodes: "node-1",
			redacts: []bool{true}, mode: redactable.ModeAccept, code: codes.OK,
		},
		{
			name: "enforce on a node which redacts", talosVersion: "v1.15.0", header: "node", nodes: "node-1",
			redacts: []bool{true}, mode: redactable.ModeEnforce, code: codes.OK,
		},
		{
			name: "no redaction asked", talosVersion: "v1.15.0", header: "node", nodes: "node-1",
			redacts: []bool{true}, mode: redactable.ModeDisabled, code: codes.PermissionDenied,
		},
		{
			name: "node which doesn't redact, recorded as one which does", talosVersion: "v1.15.0", header: "node", nodes: "node-1",
			redacts: []bool{false}, mode: redactable.ModeAccept, code: codes.PermissionDenied,
		},
		{
			name: "recorded version is unknown", talosVersion: "", header: "node", nodes: "node-1",
			redacts: []bool{true}, mode: redactable.ModeAccept, code: codes.OK,
		},
		{
			name: "node which doesn't answer", talosVersion: "v1.15.0", header: "node", nodes: "node-1",
			redacts: []bool{}, mode: redactable.ModeAccept, code: codes.PermissionDenied,
		},
		{
			name: "node without Version", talosVersion: "v1.15.0", header: "node", nodes: "node-1",
			mode: redactable.ModeAccept, code: codes.PermissionDenied,
		},
		{
			name: "several nodes", talosVersion: "v1.15.0", header: "nodes", nodes: "node-1,node-2",
			redacts: []bool{true, true}, mode: redactable.ModeAccept, code: codes.PermissionDenied,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := startFakeNode(ctx, t, tc.redacts...)
			node := startGuardedProxy(ctx, t, fake.address, versionResolver{talosVersion: tc.talosVersion})

			reqCtx := redactable.WithMode(metadata.AppendToOutgoingContext(ctx, tc.header, tc.nodes), tc.mode)

			_, err := node.List(reqCtx, sensitive)
			assert.Equal(t, tc.code, status.Code(err), "%v", err)

			if tc.code == codes.OK {
				assert.Equal(t, string(redactable.ModeEnforce), fake.lastMode())
			}
		})
	}

	t.Run("invalid mode", func(t *testing.T) {
		fake := startFakeNode(ctx, t, true)
		node := startGuardedProxy(ctx, t, fake.address, versionResolver{talosVersion: "v1.15.0"})

		reqCtx := metadata.AppendToOutgoingContext(ctx, "node", "node-1", redactable.Header, "everything")

		_, err := node.List(reqCtx, sensitive)
		assert.Equal(t, codes.InvalidArgument, status.Code(err), "%v", err)
	})
}
