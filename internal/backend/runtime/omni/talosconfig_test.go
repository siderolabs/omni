// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/prometheus/client_golang/prometheus"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/dns"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

func TestOperatorTalosconfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)
	clientFactory := talos.NewClientFactory(st, logger)
	dnsService := dns.NewService(st, logger)
	discoveryServiceClient := &discoveryClientMock{}
	workloadProxyReconciler := workloadproxy.NewReconciler(logger, zapcore.InfoLevel)

	r, err := omniruntime.New(clientFactory, dnsService, workloadProxyReconciler, nil, nil, nil, nil, nil,
		st, nil, prometheus.NewRegistry(), discoveryServiceClient, nil, logger)

	require.NoError(t, err)

	_, err = r.OperatorTalosconfig(ctx, "cluster1")
	require.Error(t, err)
	require.True(t, state.IsNotFoundError(err))

	secrets := omni.NewClusterSecrets(resources.DefaultNamespace, "cluster1")

	bundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_7)
	require.NoError(t, err)

	data, err := json.Marshal(bundle)
	require.NoError(t, err)

	secrets.TypedSpec().Value.Data = data

	require.NoError(t, st.Create(ctx, secrets))

	cfg, err := r.OperatorTalosconfig(ctx, "cluster1")
	require.NoError(t, err)

	config, err := clientconfig.FromBytes(cfg)
	require.NoError(t, err)

	require.NotEmpty(t, config.Contexts)

	m1 := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "3")
	m2 := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "2")
	m3 := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "1")

	m1.Metadata().Labels().Set(omni.LabelCluster, "cluster1")
	m2.Metadata().Labels().Set(omni.LabelCluster, "cluster1")
	m3.Metadata().Labels().Set(omni.LabelCluster, "cluster1")

	m1.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
	m2.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	m1.TypedSpec().Value.NodeIps = []string{"10.1.0.2"}
	m2.TypedSpec().Value.NodeIps = []string{"10.1.0.3"}
	m3.TypedSpec().Value.NodeIps = []string{"10.1.0.4"}

	require.NoError(t, st.Create(ctx, m1))
	require.NoError(t, st.Create(ctx, m2))
	require.NoError(t, st.Create(ctx, m3))

	cfg, err = r.OperatorTalosconfig(ctx, "cluster1")
	require.NoError(t, err)

	config, err = clientconfig.FromBytes(cfg)
	require.NoError(t, err)

	require.NotEmpty(t, config.Contexts)
	require.Equal(t, []string{"10.1.0.3", "10.1.0.2"}, config.Contexts["cluster1"].Endpoints)
}
