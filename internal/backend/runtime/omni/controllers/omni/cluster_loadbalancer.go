// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// NewClusterLoadBalancerController creates new ClusterLoadBalancerController.
func NewClusterLoadBalancerController(minPort, maxPort int) *ClusterLoadBalancerController {
	return &ClusterLoadBalancerController{
		minPort: minPort,
		maxPort: maxPort,
	}
}

// ClusterLoadBalancerController manages ClusterStatus resource lifecycle.
//
// ClusterLoadBalancerController applies the generated machine config  on each corresponding machine.
type ClusterLoadBalancerController struct {
	minPort int
	maxPort int
}

// Name implements controller.Controller interface.
func (ctrl *ClusterLoadBalancerController) Name() string {
	return "ClusterLoadBalancerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ClusterLoadBalancerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterEndpointType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      siderolink.ConfigType,
			ID:        optional.Some(siderolink.ConfigID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ClusterLoadBalancerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.LoadBalancerConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocognit
func (ctrl *ClusterLoadBalancerController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		siderolink, err := safe.ReaderGet[*siderolink.Config](ctx, r, siderolink.NewConfig(resources.DefaultNamespace).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		list, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
		if err != nil {
			return err
		}

		configs, err := safe.ReaderListAll[*omni.LoadBalancerConfig](ctx, r)
		if err != nil {
			return err
		}

		allocatedPorts := map[string]struct{}{}
		for iter := configs.Iterator(); iter.Next(); {
			allocatedPorts[iter.Value().TypedSpec().Value.BindPort] = struct{}{}
		}

		tracker := trackResource(r, resources.DefaultNamespace, omni.LoadBalancerConfigType)

		for iter := list.Iterator(); iter.Next(); {
			cluster := iter.Value()

			tracker.keep(cluster)

			var endpoints []string

			endpoints, err = ctrl.getEndpoints(ctx, r, cluster.Metadata().ID())
			if err != nil {
				return err
			}

			err = safe.WriterModify(ctx, r, omni.NewLoadBalancerConfig(resources.DefaultNamespace, cluster.Metadata().ID()), func(config *omni.LoadBalancerConfig) error {
				spec := config.TypedSpec().Value

				spec.Endpoints = endpoints

				if spec.BindPort == "" {
					spec.BindPort, err = ctrl.getPort(allocatedPorts)
					if err != nil {
						return err
					}
				}

				lbEndpoint := url.URL{
					Scheme: "https",
					Host:   net.JoinHostPort(siderolink.TypedSpec().Value.ServerAddress, spec.BindPort),
				}

				spec.SiderolinkEndpoint = lbEndpoint.String()

				return nil
			})
			if err != nil {
				return err
			}
		}

		if err = tracker.cleanup(ctx); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ClusterLoadBalancerController) getEndpoints(ctx context.Context, r controller.Runtime, clusterName string) ([]string, error) {
	clusterEndpoints, err := safe.ReaderGet[*omni.ClusterEndpoint](ctx, r, omni.NewClusterEndpoint(resources.DefaultNamespace, clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	return xslices.Map(clusterEndpoints.TypedSpec().Value.ManagementAddresses, func(addr string) string {
		return net.JoinHostPort(addr, "6443")
	}), nil
}

func (ctrl *ClusterLoadBalancerController) getPort(allocatedPorts map[string]struct{}) (string, error) {
	for i := ctrl.minPort; i <= ctrl.maxPort; i++ {
		port := strconv.FormatInt(int64(i), 10)

		if _, ok := allocatedPorts[port]; ok {
			continue
		}

		allocatedPorts[port] = struct{}{}

		return port, nil
	}

	return "", errors.New("no more ports available for the loadbalancer to allocate")
}
