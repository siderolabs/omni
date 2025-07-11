// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	omniresources "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/loadbalancer"
)

type mockLoadBalancer struct {
	startMethodCalled    chan struct{}
	shutdownMethodCalled chan struct{}
	upstreamsCh          chan []string
	ctxCancel            context.CancelFunc
	wg                   sync.WaitGroup
}

func (lb *mockLoadBalancer) Start(upstreamCh <-chan []string) error {
	var ctx context.Context

	ctx, lb.ctxCancel = context.WithCancel(context.Background())

	lb.wg.Add(2)

	go func() {
		defer lb.wg.Done()

		select {
		case lb.startMethodCalled <- struct{}{}:
		case <-ctx.Done():
			return
		}
	}()

	go func() {
		defer lb.wg.Done()

		var upstreams []string

		for {
			// copy upstreams from the channel so that the writing goroutine isn't blocked
			select {
			case upstreams = <-upstreamCh:
			case <-ctx.Done():
				return
			}

			// write copied upstreams to mock load balancer
		restart:
			select {
			case upstreams = <-upstreamCh:
				goto restart
			case lb.upstreamsCh <- upstreams:
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (lb *mockLoadBalancer) Shutdown() error {
	lb.ctxCancel()
	lb.wg.Wait()

	go func() {
		lb.shutdownMethodCalled <- struct{}{}
	}()

	return nil
}

func (lb *mockLoadBalancer) Healthy() (bool, error) {
	return true, nil
}

type LoadBalancerSuite struct {
	OmniSuite
}

func TestLoadBalancerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(LoadBalancerSuite))
}

func expectSignal[T comparable](suite *LoadBalancerSuite, signalChan <-chan T, expectedSignal T) {
	select {
	case receivedSignal := <-signalChan:
		suite.Assert().Equal(expectedSignal, receivedSignal)
	case <-time.After(3 * time.Second):
		suite.FailNow("expected Signal %#v on channel %#v", signalChan, expectedSignal)
	}
}

type newLoadBalancerSignature struct {
	bindAddress string
	bindPort    int
}

func (suite *LoadBalancerSuite) setupMock() (<-chan *mockLoadBalancer, <-chan newLoadBalancerSignature) {
	mockCh := make(chan *mockLoadBalancer)
	newLoadBalancerMethodCalled := make(chan newLoadBalancerSignature)

	newMockFunc := func(bindAddress string, bindPort int, _ *zap.Logger) (loadbalancer.LoadBalancer, error) {
		mock := &mockLoadBalancer{
			startMethodCalled:    make(chan struct{}),
			shutdownMethodCalled: make(chan struct{}),
			upstreamsCh:          make(chan []string),
		}

		go func() {
			newLoadBalancerMethodCalled <- newLoadBalancerSignature{
				bindAddress: bindAddress,
				bindPort:    bindPort,
			}
		}()

		mockCh <- mock

		return mock, nil
	}

	suite.newLoadBalancerSetup(newMockFunc)

	return mockCh, newLoadBalancerMethodCalled
}

func (suite *LoadBalancerSuite) TestLoadBalancers() {
	mockCh, newLoadBalancerMethodCalled := suite.setupMock()

	suite.startRuntime()

	createLoadBalancer := func(bindPort int) {
		loadBalancer := omniresources.NewLoadBalancerConfig(resources.DefaultNamespace, strconv.Itoa(bindPort))
		loadBalancer.TypedSpec().Value.BindPort = strconv.Itoa(bindPort)

		suite.Require().NoError(suite.state.Create(suite.ctx, loadBalancer))

		clusterStatus := omniresources.NewClusterStatus(resources.DefaultNamespace, strconv.Itoa(bindPort))
		clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true

		suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))
	}

	bindPort1 := 20000
	bindPort2 := 12345
	bindPort3 := 11111

	createLoadBalancer(bindPort1)

	lb1 := <-mockCh

	expectSignal(suite, newLoadBalancerMethodCalled, newLoadBalancerSignature{
		bindAddress: "0.0.0.0",
		bindPort:    bindPort1,
	})
	expectSignal(suite, lb1.startMethodCalled, struct{}{})

	createLoadBalancer(bindPort2)

	lb2 := <-mockCh

	expectSignal(suite, newLoadBalancerMethodCalled, newLoadBalancerSignature{
		bindAddress: "0.0.0.0",
		bindPort:    bindPort2,
	})
	expectSignal(suite, lb2.startMethodCalled, struct{}{})

	createLoadBalancer(bindPort3)

	lb3 := <-mockCh

	expectSignal(suite, newLoadBalancerMethodCalled, newLoadBalancerSignature{
		bindAddress: "0.0.0.0",
		bindPort:    bindPort3,
	})
	expectSignal(suite, lb3.startMethodCalled, struct{}{})

	suite.assertStatus(strconv.Itoa(bindPort1), true, false)
	suite.assertStatus(strconv.Itoa(bindPort2), true, false)
	suite.assertStatus(strconv.Itoa(bindPort3), true, false)

	// update cluster status to not have connected control planes
	clusterStatus1, err := safe.StateUpdateWithConflicts(
		suite.ctx, suite.state,
		omniresources.NewClusterStatus(resources.DefaultNamespace, strconv.Itoa(bindPort1)).Metadata(),
		func(r *omniresources.ClusterStatus) error {
			r.TypedSpec().Value.HasConnectedControlPlanes = false

			return nil
		})
	suite.Require().NoError(err)

	// the loadbalancer should be stopped
	expectSignal(suite, lb1.shutdownMethodCalled, struct{}{})

	suite.assertStatus(strconv.Itoa(bindPort1), false, true)

	// update cluster status to have connected control planes again
	clusterStatus1.TypedSpec().Value.HasConnectedControlPlanes = true
	suite.Require().NoError(suite.state.Update(suite.ctx, clusterStatus1))

	// the loadbalancer should be started again
	lb1 = <-mockCh

	expectSignal(suite, newLoadBalancerMethodCalled, newLoadBalancerSignature{
		bindAddress: "0.0.0.0",
		bindPort:    bindPort1,
	})
	expectSignal(suite, lb1.startMethodCalled, struct{}{})

	suite.assertStatus(strconv.Itoa(bindPort1), true, false)

	suite.ctxCancel()
	suite.wg.Wait()

	expectSignal(suite, lb1.shutdownMethodCalled, struct{}{})
	expectSignal(suite, lb2.shutdownMethodCalled, struct{}{})
	expectSignal(suite, lb3.shutdownMethodCalled, struct{}{})
}

func (suite *LoadBalancerSuite) newLoadBalancerSetup(newLoadBalancerFunc loadbalancer.NewFunc) {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&omni.LoadBalancerController{
				NewLoadBalancer: newLoadBalancerFunc,
			},
		),
	)
}

func (suite *LoadBalancerSuite) assertStatus(id string, expectedHealthy, expectedStopped bool) {
	assertResource(
		&suite.OmniSuite,
		*omniresources.NewLoadBalancerStatus(resources.DefaultNamespace, id).Metadata(),
		func(res *omniresources.LoadBalancerStatus, assertions *assert.Assertions) {
			lbStatus := res.TypedSpec().Value
			assertions.Equal(expectedHealthy, lbStatus.Healthy)
			assertions.Equal(expectedStopped, lbStatus.Stopped)
		},
	)
}

func (suite *LoadBalancerSuite) TestChangeAddress() {
	mockCh, newLoadBalancerMethodCalled := suite.setupMock()

	suite.startRuntime()

	const testID = ""

	loadBalancer := omniresources.NewLoadBalancerConfig(resources.DefaultNamespace, testID)
	expectedBindPort := 20000
	loadBalancer.TypedSpec().Value.BindPort = strconv.Itoa(expectedBindPort)
	suite.Require().NoError(suite.state.Create(suite.ctx, loadBalancer))

	clusterStatus := omniresources.NewClusterStatus(resources.DefaultNamespace, testID)
	clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	mock := <-mockCh

	expectSignal(suite, newLoadBalancerMethodCalled, newLoadBalancerSignature{
		bindAddress: "0.0.0.0",
		bindPort:    expectedBindPort,
	})
	expectSignal(suite, mock.startMethodCalled, struct{}{})
	suite.assertStatus(testID, true, false)

	newExpectedBindPort := 123123

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, loadBalancer.Metadata(), func(r *omniresources.LoadBalancerConfig) error {
		spec := r.TypedSpec().Value
		spec.BindPort = strconv.Itoa(newExpectedBindPort)

		return nil
	})
	suite.Require().NoError(err)

	expectSignal(suite, mock.shutdownMethodCalled, struct{}{})

	mock = <-mockCh

	expectSignal(suite, newLoadBalancerMethodCalled, newLoadBalancerSignature{
		bindAddress: "0.0.0.0",
		bindPort:    newExpectedBindPort,
	})
	expectSignal(suite, mock.startMethodCalled, struct{}{})

	assertResource(
		&suite.OmniSuite,
		*omniresources.NewLoadBalancerStatus(resources.EphemeralNamespace, testID).Metadata(),
		func(res *omniresources.LoadBalancerStatus, assertions *assert.Assertions) {
			lbStatus := res.TypedSpec().Value

			assertions.True(lbStatus.Healthy)
		},
	)

	suite.ctxCancel()
	suite.wg.Wait()

	expectSignal(suite, mock.shutdownMethodCalled, struct{}{})
}

func (suite *LoadBalancerSuite) TestStopLoadBalancer() {
	mockCh, _ := suite.setupMock()

	suite.startRuntime()

	const testID = ""

	loadBalancer := omniresources.NewLoadBalancerConfig(resources.DefaultNamespace, testID)
	expectedBindPort := 20000
	loadBalancer.TypedSpec().Value.BindPort = strconv.Itoa(expectedBindPort)
	suite.Require().NoError(suite.state.Create(suite.ctx, loadBalancer))

	clusterStatus := omniresources.NewClusterStatus(resources.DefaultNamespace, testID)
	clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	mock := <-mockCh

	suite.assertStatus(testID, true, false)

	suite.Assert().NoError(suite.state.Destroy(suite.ctx, loadBalancer.Metadata()))

	expectSignal(suite, mock.shutdownMethodCalled, struct{}{})

	suite.assertNoResource(*omniresources.NewLoadBalancerStatus(resources.DefaultNamespace, testID).Metadata())
}

func (suite *LoadBalancerSuite) TestUpdateUpstreams() {
	mockCh, _ := suite.setupMock()

	suite.startRuntime()

	const testID = ""

	loadBalancer := omniresources.NewLoadBalancerConfig(resources.DefaultNamespace, testID)
	expectedBindPort := 20000
	loadBalancer.TypedSpec().Value.BindPort = strconv.Itoa(expectedBindPort)
	loadBalancer.TypedSpec().Value.Endpoints = []string{
		"testEndpoint",
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, loadBalancer))

	clusterStatus := omniresources.NewClusterStatus(resources.DefaultNamespace, testID)
	clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	mock := <-mockCh
	assertUpstreams := func(expectedUpstreams ...string) {
		for {
			select {
			case <-time.After(time.Second):
				suite.FailNow("timeout")
			case upstreamList := <-mock.upstreamsCh:
				if len(upstreamList) != len(expectedUpstreams) {
					continue
				}

				for i, value := range upstreamList {
					if expectedUpstreams[i] != value {
						continue
					}
				}

				return
			}
		}
	}

	assertUpstreams("testEndpoint")

	suite.assertStatus(testID, true, false)

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, loadBalancer.Metadata(), func(r *omniresources.LoadBalancerConfig) error {
		spec := r.TypedSpec().Value
		spec.Endpoints = append(spec.Endpoints, "newEndpoint")

		return nil
	})
	suite.Require().NoError(err)

	assertUpstreams("testEndpoint", "newEndpoint")

	suite.ctxCancel()
	suite.wg.Wait()
}
