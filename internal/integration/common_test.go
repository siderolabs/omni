// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	talosclientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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
		network := res.Spec().(*omni.MachineStatusSpec).Value.Network //nolint:forcetypeassert,errcheck
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

func nodes(ctx context.Context, t *testing.T, options *TestOptions, clusterName string, labels ...resource.LabelQueryOption) ([]node, error) {
	talosClient := getTalosClientForCluster(ctx, t, options, clusterName)

	st := options.omniClient.Omni().State()

	nodeIPs, err := talosNodeIPs(ctx, talosClient.COSI)
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

	for clusterMachine := range clusterMachineList.All() {
		machine, err := safe.StateGet[*omni.Machine](ctx, st, omni.NewMachine(clusterMachine.Metadata().ID()).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return nil, err
		}

		machineStatus, err := safe.StateGet[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(clusterMachine.Metadata().ID()).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return nil, err
		}

		clusterMachineStatus, err := safe.StateGet[*omni.ClusterMachineStatus](ctx, st, omni.NewClusterMachineStatus(clusterMachine.Metadata().ID()).Metadata())
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

func getTalosClient(ctx context.Context, t *testing.T, options *TestOptions) *talosclient.Client {
	return getTalosClientForCluster(ctx, t, options, "")
}

func getTalosClientForCluster(ctx context.Context, t *testing.T, options *TestOptions, clusterName string) *talosclient.Client {
	data, err := options.omniClient.Management().Talosconfig(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, data, "talosconfig for cluster %q is empty", clusterName)

	config, err := talosclientconfig.FromBytes(data)
	require.NoError(t, err)

	opts := []talosclient.OptionFunc{
		talosclient.WithConfig(config),
		talosclient.WithServiceAccount(options.serviceAccountKey),
	}

	if clusterName != "" {
		opts = append(opts, talosclient.WithCluster(clusterName))
	}

	client, err := talosclient.New(ctx, opts...)
	require.NoError(t, err)

	return client
}

func talosNodeIPs(ctx context.Context, talosState state.State) ([]string, error) {
	list, err := safe.StateListAll[*cluster.Member](ctx, talosState)
	if err != nil {
		return nil, err
	}

	nodeIPs := make([]string, 0, list.Len())

	for member := range list.All() {
		if len(member.TypedSpec().Addresses) == 0 {
			return nil, fmt.Errorf("no addresses for member %q", member.Metadata().ID())
		}

		nodeIPs = append(nodeIPs, member.TypedSpec().Addresses[0].String())
	}

	return nodeIPs, nil
}

//nolint:govet
type testGroup struct {
	Name         string
	Description  string
	Parallel     bool
	MachineClaim int
	Subtests     []subTest
	Finalizer    func(t *testing.T)
}

//nolint:govet
type subTest struct {
	Name string
	F    func(t *testing.T)
}

type subTestList []subTest

func subTests(items ...subTest) subTestList {
	return items
}

func (l subTestList) Append(items ...subTest) subTestList {
	return append(l, items...)
}

// MachineOptions are the options for machine creation.
type MachineOptions struct {
	TalosVersion      string
	KubernetesVersion string
}

// TestFunc is a testing function prototype.
type TestFunc func(t *testing.T)

// RestartAMachineFunc is a function to restart a machine by UUID.
type RestartAMachineFunc func(ctx context.Context, uuid string) error

// WipeAMachineFunc is a function to wipe a machine by UUID.
type WipeAMachineFunc func(ctx context.Context, uuid string) error

// FreezeAMachineFunc is a function to freeze a machine by UUID.
type FreezeAMachineFunc func(ctx context.Context, uuid string) error

// Options for the test runner.
//
//nolint:govet
type Options struct {
	CleanupLinks                bool
	SkipExtensionsCheckOnCreate bool
	RunStatsCheck               bool
	ExpectedMachines            int

	RestartAMachineFunc RestartAMachineFunc
	WipeAMachineFunc    WipeAMachineFunc
	FreezeAMachineFunc  FreezeAMachineFunc
	ProvisionConfigs    []MachineProvisionConfig

	MachineOptions MachineOptions

	HTTPEndpoint             string
	AnotherTalosVersion      string
	StableTalosVersion       string
	AnotherKubernetesVersion string
	OmnictlPath              string
	ScalingTimeout           time.Duration
	SleepAfterFailure        time.Duration
	StaticInfraProvider      string
	OutputDir                string
	TalosconfigPath          string
	ImportedClusterStatePath string
}

func (o Options) defaultInfraProvider() string {
	if len(o.ProvisionConfigs) == 0 {
		return ""
	}

	return o.ProvisionConfigs[0].Provider.ID
}

func (o Options) defaultProviderData() string {
	if len(o.ProvisionConfigs) == 0 {
		return "{}"
	}

	return o.ProvisionConfigs[0].Provider.Data
}

func (o Options) provisionMachines() bool {
	var totalMachineCount int

	for _, cfg := range o.ProvisionConfigs {
		totalMachineCount += cfg.MachineCount
	}

	return totalMachineCount > 0
}

// MachineProvisionConfig tells the test to provision machines from the infra provider.
type MachineProvisionConfig struct {
	Provider     MachineProviderConfig `yaml:"provider"`
	MachineCount int                   `yaml:"count"`
}

// MachineProviderConfig keeps the configuration of the infra provider for the machine provision config.
type MachineProviderConfig struct {
	ID     string `yaml:"id"`
	Data   string `yaml:"data"`
	Static bool   `yaml:"static"`
}

// TestOptions constains all common data that might be required to run the tests.
type TestOptions struct {
	Options
	omniClient        *client.Client
	serviceAccountKey string

	machineSemaphore *semaphore.Weighted
}
