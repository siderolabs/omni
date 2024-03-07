// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

func resourceDetails(res resource.Resource) string {
	parts := []string{
		fmt.Sprintf("metadata: %s", res.Metadata().String()),
	}

	hostname, ok := res.Metadata().Labels().Get(omni.LabelHostname)
	if !ok {
		parts = append(parts, fmt.Sprintf("hostname: %s", hostname))
	}

	if res.Metadata().Type() == omni.MachineStatusType {
		network := res.Spec().(*omni.MachineStatusSpec).Value.Network //nolint:forcetypeassert
		if network != nil {
			parts = append(parts, fmt.Sprintf("hostname: %s", network.Hostname))
		}
	}

	return strings.Join(parts, ", ")
}

type node struct {
	machine              *omni.Machine
	machineStatus        *omni.MachineStatus
	clusterMachine       *omni.ClusterMachine
	clusterMachineStatus *omni.ClusterMachineStatus
	talosIP              string
}

func nodes(ctx context.Context, cli *client.Client, clusterName string, labels ...resource.LabelQueryOption) ([]node, error) {
	talosCli, err := talosClient(ctx, cli, clusterName)
	if err != nil {
		return nil, err
	}

	st := cli.Omni().State()

	nodeIPs, err := talosNodeIPs(ctx, talosCli.COSI)
	if err != nil {
		return nil, err
	}

	labelQueryOptions := make([]resource.LabelQueryOption, 0, len(labels)+1)
	labelQueryOptions = append(labelQueryOptions, labels...)
	labelQueryOptions = append(labelQueryOptions, resource.LabelEqual(omni.LabelCluster, clusterName))

	clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](
		ctx,
		st,
		state.WithLabelQuery(labelQueryOptions...),
	)
	if err != nil {
		return nil, err
	}

	nodeList := make([]node, 0, clusterMachineList.Len())

	for iter := clusterMachineList.Iterator(); iter.Next(); {
		clusterMachine := iter.Value()

		machine, err := safe.StateGet[*omni.Machine](ctx, st, omni.NewMachine(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return nil, err
		}

		machineStatus, err := safe.StateGet[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return nil, err
		}

		clusterMachineStatus, err := safe.StateGet[*omni.ClusterMachineStatus](ctx, st, omni.NewClusterMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return nil, err
		}

		var talosIP string

		for _, address := range machineStatus.TypedSpec().Value.GetNetwork().GetAddresses() {
			for _, nodeIP := range nodeIPs {
				ip, _, err := net.ParseCIDR(address)
				if err != nil {
					continue
				}

				if ip.String() == nodeIP {
					talosIP = nodeIP

					break
				}
			}
		}

		nodeList = append(nodeList, node{
			machine:              machine,
			machineStatus:        machineStatus,
			clusterMachine:       clusterMachine,
			clusterMachineStatus: clusterMachineStatus,
			talosIP:              talosIP,
		})
	}

	return nodeList, nil
}

func talosClient(ctx context.Context, cli *client.Client, clusterName string) (*talosclient.Client, error) {
	data, err := cli.Management().Talosconfig(ctx)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, errors.New("empty talosconfig")
	}

	config, err := clientconfig.FromBytes(data)
	if err != nil {
		return nil, err
	}

	return talosclient.New(
		ctx,
		talosclient.WithConfig(config),
		talosclient.WithCluster(clusterName),
	)
}

func talosClientMaintenance(ctx context.Context, endpoint string) (*talosclient.Client, error) {
	opts := talos.GetSocketOptions(endpoint)

	opts = append(opts, talosclient.WithTLSConfig(&tls.Config{
		InsecureSkipVerify: true,
	}), talosclient.WithEndpoints(endpoint))

	return talosclient.New(ctx, opts...)
}

func talosNodeIPs(ctx context.Context, talosState state.State) ([]string, error) {
	list, err := safe.StateListAll[*cluster.Member](ctx, talosState)
	if err != nil {
		return nil, err
	}

	nodeIPs := make([]string, 0, list.Len())

	for iter := list.Iterator(); iter.Next(); {
		member := iter.Value()

		if len(member.TypedSpec().Addresses) == 0 {
			return nil, fmt.Errorf("no addresses for member %q", member.Metadata().ID())
		}

		nodeIPs = append(nodeIPs, member.TypedSpec().Addresses[0].String())
	}

	return nodeIPs, nil
}
