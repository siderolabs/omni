// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/go-api-signature/pkg/message"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	machineryconfig "github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	managementclient "github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

//go:embed testdata/admin-kubeconfig.yaml
var adminKubeconfig []byte

func TestGenerateConfigs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)

	rt, err := omniruntime.New(nil, nil, nil, nil,
		nil, nil, nil, st, nil, prometheus.NewRegistry(), nil, nil, logger)
	require.NoError(t, err)

	runtime.Install(omniruntime.Name, rt)

	k8s, err := kubernetes.New(st)
	require.NoError(t, err)

	runtime.Install(kubernetes.Name, k8s)

	clusterName := "cluster1"

	address := runServer(t, st)

	c, err := client.New(address)
	require.NoError(t, err)

	client := c.Management().WithCluster(clusterName)

	defer func() {
		require.NoError(t, c.Close())
	}()

	adminCtx := metadata.AppendToOutgoingContext(ctx, "role", string(role.Admin))
	readerCtx := metadata.AppendToOutgoingContext(ctx, "role", string(role.Reader))

	t.Run("kubeconfig not enabled", func(t *testing.T) {
		_, err = client.Kubeconfig(adminCtx, managementclient.WithBreakGlassKubeconfig(true))

		require.Error(t, err)
		require.Equal(t, codes.PermissionDenied, status.Code(err), err)
	})

	config.Config.EnableBreakGlassConfigs = true

	defer func() {
		config.Config.EnableBreakGlassConfigs = false
	}()

	t.Run("kubeconfig enabled no cluster", func(t *testing.T) {
		_, err = client.Kubeconfig(adminCtx, managementclient.WithBreakGlassKubeconfig(true))

		require.Error(t, err)
	})

	t.Run("kubeconfig enabled success", func(t *testing.T) {
		kubeconfigResource := omni.NewKubeconfig(resources.DefaultNamespace, "cluster1")

		kubeconfigResource.TypedSpec().Value.Data = adminKubeconfig

		require.NoError(t, st.Create(ctx, kubeconfigResource))

		var kubeconfig []byte

		kubeconfig, err = client.Kubeconfig(adminCtx, managementclient.WithBreakGlassKubeconfig(true))
		require.NoError(t, err)

		_, err = clientcmd.Load(kubeconfig)
		require.NoError(t, err)

		var taint *omni.ClusterTaint

		taint, err = safe.ReaderGetByID[*omni.ClusterTaint](ctx, st, clusterName)
		require.NoError(t, err)

		require.NoError(t, st.Destroy(ctx, taint.Metadata()))
	})

	t.Run("kubeconfig enabled no auth", func(t *testing.T) {
		_, err = client.Kubeconfig(readerCtx, managementclient.WithBreakGlassKubeconfig(true))

		require.Error(t, err)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("talosconfig enabled no auth", func(t *testing.T) {
		_, err = client.Talosconfig(readerCtx, managementclient.WithBreakGlassTalosconfig(true))

		require.Error(t, err)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("talosconfig enabled success", func(t *testing.T) {
		secrets := omni.NewClusterSecrets(resources.DefaultNamespace, "cluster1")

		bundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), machineryconfig.TalosVersion1_7)
		require.NoError(t, err)

		var data []byte

		data, err = json.Marshal(bundle)
		require.NoError(t, err)

		secrets.TypedSpec().Value.Data = data

		require.NoError(t, st.Create(ctx, secrets))

		var cfg []byte

		cfg, err = client.Talosconfig(adminCtx, managementclient.WithBreakGlassTalosconfig(true))
		require.NoError(t, err)

		var config *clientconfig.Config

		config, err = clientconfig.FromBytes(cfg)
		require.NoError(t, err)

		require.NotEmpty(t, config.Contexts)

		var taint *omni.ClusterTaint

		taint, err = safe.ReaderGetByID[*omni.ClusterTaint](ctx, st, clusterName)
		require.NoError(t, err)

		require.NoError(t, st.Destroy(ctx, taint.Metadata()))
	})
}

func runServer(t *testing.T, st state.State, opts ...grpc.ServerOption) string {
	var err error

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	grpcAddress := fmt.Sprintf("grpc://%s", listener.Addr())

	logger := zaptest.NewLogger(t)

	opts = append(opts,
		grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				md = metadata.New(nil)
			}

			ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: true})

			msg := message.NewGRPC(md, info.FullMethod)

			ctx = ctxstore.WithValue(ctx, auth.GRPCMessageContextKey{Message: msg})

			if r := md.Get("role"); len(r) > 0 {
				var parsed role.Role

				parsed, err = role.Parse(r[0])
				if err != nil {
					return nil, err
				}

				ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: parsed})
			}

			return handler(ctx, req)
		}),
	)

	server := grpc.NewServer(opts...)

	management.RegisterManagementServiceServer(server, grpcomni.NewManagementServer(
		st,
		nil,
		logger,
	))

	var eg errgroup.Group

	eg.Go(func() error {
		for {
			err = server.Serve(listener)
			if err == nil || errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}
		}
	})

	t.Cleanup(func() {
		server.Stop()

		require.NoError(t, eg.Wait())
	})

	return grpcAddress
}
