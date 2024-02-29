// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

func TestServiceRegistry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	serviceRegistry, serviceRegistryErr := workloadproxy.NewServiceRegistry(st, zaptest.NewLogger(t))
	require.NoError(t, serviceRegistryErr)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return serviceRegistry.Start(ctx)
	})

	t.Cleanup(func() {
		cancel()
		require.NoError(t, eg.Wait())
	})

	t.Run("get non-existing service handler", func(t *testing.T) {
		t.Parallel()

		proxy, clusterID, err := serviceRegistry.GetProxy("service1")
		require.NoError(t, err)

		require.Nil(t, proxy)
		require.Zero(t, clusterID)
	})

	t.Run("service exists but feature is disabled on cluster", func(t *testing.T) {
		t.Parallel()

		cluster := omni.NewCluster(resources.DefaultNamespace, "cluster1")

		require.NoError(t, st.Create(ctx, cluster))

		exposedService := omni.NewExposedService(resources.DefaultNamespace, "cluster1/service1")
		exposedService.Metadata().Labels().Set(omni.LabelCluster, "cluster1")
		exposedService.Metadata().Labels().Set(omni.LabelExposedServiceAlias, "service1")

		require.NoError(t, st.Create(ctx, exposedService))

		clusterMachineStatus := omni.NewClusterMachineStatus(resources.DefaultNamespace, "clustermachine-1")
		clusterMachineStatus.Metadata().Labels().Set(omni.LabelCluster, "cluster1")

		clusterMachineStatus.TypedSpec().Value.ManagementAddress = "127.0.0.42"
		clusterMachineStatus.TypedSpec().Value.Ready = true

		require.NoError(t, st.Create(ctx, clusterMachineStatus))

		sleepWithContext(ctx, 2*time.Second)

		proxy, clusterID, err := serviceRegistry.GetProxy("service1")
		require.NoError(t, err)
		require.Nil(t, proxy)
		require.Zero(t, clusterID)
	})

	t.Run("service exists and feature is enabled on cluster", func(t *testing.T) {
		t.Parallel()

		cluster := omni.NewCluster(resources.DefaultNamespace, "cluster2")
		cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
			EnableWorkloadProxy: true,
		}

		require.NoError(t, st.Create(ctx, cluster))

		exposedService := omni.NewExposedService(resources.DefaultNamespace, "cluster2/service2")
		exposedService.Metadata().Labels().Set(omni.LabelCluster, "cluster2")
		exposedService.Metadata().Labels().Set(omni.LabelExposedServiceAlias, "service2")

		require.NoError(t, st.Create(ctx, exposedService))

		clusterMachineStatus := omni.NewClusterMachineStatus(resources.DefaultNamespace, "clustermachine-2")
		clusterMachineStatus.Metadata().Labels().Set(omni.LabelCluster, "cluster2")

		clusterMachineStatus.TypedSpec().Value.ManagementAddress = "127.0.0.43"
		clusterMachineStatus.TypedSpec().Value.Ready = true

		require.NoError(t, st.Create(ctx, clusterMachineStatus))

		require.NoError(t, retry.Constant(3*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(func() error {
			proxy, clusterID, err := serviceRegistry.GetProxy("service2")
			if err != nil {
				return retry.ExpectedError(err)
			}

			if proxy == nil {
				return retry.ExpectedError(errors.New("proxy is nil"))
			}

			if clusterID != "cluster2" {
				return retry.ExpectedError(fmt.Errorf("wrong cluster id: %s", clusterID))
			}

			return nil
		}))

		// kill the single healthy upstream
		clusterMachineStatus.TypedSpec().Value.Ready = false

		require.NoError(t, st.Update(ctx, clusterMachineStatus))

		require.NoError(t, retry.Constant(3*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(func() error {
			_, _, proxyErr := serviceRegistry.GetProxy("service2")
			if proxyErr == nil {
				return retry.ExpectedError(errors.New("proxy error is nil"))
			}

			return nil
		}))

		// remove exposed service
		require.NoError(t, st.Destroy(ctx, exposedService.Metadata()))

		require.NoError(t, retry.Constant(3*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(func() error {
			proxy, clusterID, proxyErr := serviceRegistry.GetProxy("service2")
			if proxyErr != nil {
				return retry.ExpectedError(errors.New("proxy error is nil"))
			}

			if proxy != nil {
				return retry.ExpectedError(errors.New("proxy is not nil"))
			}

			if clusterID != "" {
				return retry.ExpectedError(errors.New("clusterID is not empty"))
			}

			return nil
		}))
	})
}

func sleepWithContext(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
	}
}
