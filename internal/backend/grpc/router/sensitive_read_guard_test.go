// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/grpc-proxy/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/backend/grpc/router"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
)

const sensitiveType = "ApiCertificates.secrets.talos.dev"

// startFakeNode serves the COSI resource API of a node, with a sensitive resource type registered, without any
// access control: as a node in maintenance mode does for a SideroLink peer, and as a node must not be relied on for.
func startFakeNode(ctx context.Context, t *testing.T) string {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
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

	srv := grpc.NewServer()
	v1alpha1.RegisterStateServer(srv, server.NewState(st))

	errCh := make(chan error, 1)

	go func() { errCh <- srv.Serve(lis) }()

	t.Cleanup(func() {
		srv.Stop()

		if err := <-errCh; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			require.NoError(t, err)
		}
	})

	return lis.Addr().String()
}

// guardedDirector proxies to the fake node over a connection guarded like the router's connections to the nodes.
type guardedDirector struct {
	omniState    state.State
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

	backend := router.NewTalosBackend("maintenance", "", &testNodeResolver{}, conn, false,
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		},
		d.omniState, nil,
	)

	return proxy.One2One, []proxy.Backend{backend}, nil
}

// TestSensitiveReadGuard checks that the COSI reads proxied to a node are refused for the sensitive resource types,
// and pass for the others, before they reach the node, whatever the node itself enforces.
func TestSensitiveReadGuard(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	st, err := omniruntime.NewTestState(zaptest.NewLogger(t))
	require.NoError(t, err)

	nodeEndpoint := startFakeNode(ctx, t)

	proxyServer := router.NewServer(&guardedDirector{omniState: st.Default(), nodeEndpoint: nodeEndpoint})
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

	// a regular COSI client, as talosctl is
	node := state.WrapCore(client.NewAdapter(v1alpha1.NewStateClient(conn), client.WithDisableWatchRetry()))

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
