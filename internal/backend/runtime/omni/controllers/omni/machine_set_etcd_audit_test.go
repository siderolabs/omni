// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	talosruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

func setupEtcdAuditCluster(
	ctx context.Context,
	t *testing.T,
	st state.State,
	machineServices *testutils.MachineServices,
	clusterName string,
	machineIDs []string,
) (*omni.Cluster, *omni.MachineSet) {
	cluster := rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterName))
	rmock.Mock[*omni.ClusterSecrets](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.TalosConfig](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.ClusterStatus](ctx, t, st, options.SameID(cluster))

	machineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.ControlPlanesResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelControlPlaneRole),
		options.Modify(func(res *omni.MachineSet) error {
			res.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling

			return nil
		}),
	)

	rmock.MockList[*omni.MachineSetNode](ctx, t, st,
		options.IDs(machineIDs),
		options.ItemOptions(
			options.LabelCluster(cluster),
			options.LabelMachineSet(machineSet),
			options.EmptyLabel(omni.LabelControlPlaneRole),
		),
	)

	for _, id := range machineIDs {
		rmock.Mock[*omni.MachineStatus](ctx, t, st,
			options.WithID(id),
			options.WithMachineServices(ctx, machineServices),
		)

		rmock.Mock[*omni.ClusterMachine](ctx, t, st, options.WithID(id))

		rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st, options.WithID(id))
	}

	return cluster, machineSet
}

func addClusterMachine(
	ctx context.Context,
	t *testing.T,
	st state.State,
	machineServices *testutils.MachineServices,
	cluster *omni.Cluster,
	machineSet *omni.MachineSet,
	id string,
) {
	rmock.Mock[*omni.MachineSetNode](ctx, t, st,
		options.WithID(id),
		options.LabelCluster(cluster),
		options.LabelMachineSet(machineSet),
		options.EmptyLabel(omni.LabelControlPlaneRole),
	)

	rmock.Mock[*omni.MachineStatus](ctx, t, st,
		options.WithID(id),
		options.WithMachineServices(ctx, machineServices),
	)

	rmock.Mock[*omni.ClusterMachine](ctx, t, st, options.WithID(id))
	rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st, options.WithID(id))
}

func TestMachineSetEtcdAudit(t *testing.T) {
	t.Parallel()

	machines := []string{"n1", "n2", "n3"}

	registerControllers := func(t *testing.T) testutils.TestFunc {
		return func(_ context.Context, tc testutils.TestContext) {
			clientFactory := talos.NewClientFactory(tc.State, tc.Logger)

			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewClusterEndpointController()))
			require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewMachineSetEtcdAuditController(
				clientFactory,
				100*time.Millisecond,
			)))
		}
	}

	setupTalosState := func(ctx context.Context, t *testing.T, machineServices *testutils.MachineServices, machineIDs []string, etcdResp *machine.EtcdMemberListResponse) {
		for i, id := range machineIDs {
			svc := machineServices.Get(id)
			svc.SetEtcdMembers(etcdResp)

			require.NoError(t, svc.State.Create(ctx, talosruntime.NewMountStatus(talosruntime.NamespaceName, "EPHEMERAL")))

			member := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
			member.TypedSpec().MemberID = etcd.FormatMemberID(uint64(i + 1))

			require.NoError(t, svc.State.Create(ctx, member))
		}
	}

	t.Run("unchanged", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerControllers(t),
			func(ctx context.Context, tc testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, tc.State)
				setupEtcdAuditCluster(ctx, t, tc.State, machineServices, "etcd_audit/unchanged", machines)

				etcdResp := &machine.EtcdMemberListResponse{
					Messages: []*machine.EtcdMembers{{
						Members: []*machine.EtcdMember{
							{Id: 1, Hostname: "n1"},
							{Id: 2, Hostname: "n2"},
							{Id: 3, Hostname: "n3"},
						},
					}},
				}

				setupTalosState(ctx, t, machineServices, machines, etcdResp)

				// This is a negative assertion: the audit controller should NOT remove any members
				// since the three cluster machines (n1, n2, n3) match the three etcd members exactly.
				// There is no observable state change to poll on, so a brief sleep gives the controller
				// time to run before we verify the member count is unchanged.
				time.Sleep(500 * time.Millisecond)

				for _, m := range machines {
					assert.Equal(t, 3, machineServices.Get(m).GetEtcdMemberCount())
				}
			},
		)
	})

	t.Run("cleanedUp", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerControllers(t),
			func(ctx context.Context, tc testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, tc.State)
				setupEtcdAuditCluster(ctx, t, tc.State, machineServices, "etcd_audit/cleanedUp", machines)

				etcdResp := &machine.EtcdMemberListResponse{
					Messages: []*machine.EtcdMembers{{
						Members: []*machine.EtcdMember{
							{Id: 1, Hostname: "n1"},
							{Id: 2, Hostname: "n2"},
							{Id: 3, Hostname: "n3"},
							{Id: 4, Hostname: "n4"},
						},
					}},
				}

				setupTalosState(ctx, t, machineServices, machines, etcdResp)

				assert.NoError(t, retry.Constant(20*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
					var err error

					for _, m := range machines {
						count := machineServices.Get(m).GetEtcdMemberCount()
						if count == 3 {
							return nil
						}

						err = retry.ExpectedErrorf("expected to have 3 etcd members got %d", count)
					}

					return err
				}))
			},
		)
	})
}

//nolint:maintidx
func TestEtcdStatus(t *testing.T) {
	t.Parallel()

	machines := []string{"n1", "n2", "n3"}

	memberIDs := map[string]uint64{
		"n1": 1,
		"n2": 2,
		"n3": 3,
		"n4": 4,
	}

	allMembers := []*machine.EtcdMember{
		{Id: memberIDs["n1"], Hostname: "n1"},
		{Id: memberIDs["n2"], Hostname: "n2"},
		{Id: memberIDs["n3"], Hostname: "n3"},
	}

	extraEtcdMembers := []*machine.EtcdMember{
		{Id: memberIDs["n1"], Hostname: "n1"},
		{Id: memberIDs["n2"], Hostname: "n2"},
		{Id: memberIDs["n3"], Hostname: "n3"},
		{Id: memberIDs["n4"], Hostname: "n4"},
	}

	etcdRunning := &machine.ServiceListResponse{
		Messages: []*machine.ServiceList{
			{
				Services: []*machine.ServiceInfo{
					{
						Id: "etcd",
						Health: &machine.ServiceHealth{
							Healthy: true,
						},
					},
				},
			},
		},
	}

	etcdDown := &machine.ServiceListResponse{
		Messages: []*machine.ServiceList{
			{
				Services: []*machine.ServiceInfo{
					{
						Id: "etcd",
						Health: &machine.ServiceHealth{
							Healthy: false,
						},
					},
				},
			},
		},
	}

	initIdentities := func(ctx context.Context, t *testing.T, st state.State, items ...string) {
		for _, m := range items {
			rmock.Mock[*omni.ClusterMachineIdentity](ctx, t, st,
				options.WithID(m),
				options.Modify(func(res *omni.ClusterMachineIdentity) error {
					res.TypedSpec().Value.EtcdMemberId = memberIDs[m]

					return nil
				}),
			)
		}
	}

	setConnected := func(ctx context.Context, t *testing.T, st state.State, items ...string) {
		for _, m := range items {
			rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st,
				options.WithID(m),
				options.Modify(func(res *omni.ClusterMachineStatus) error {
					res.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")

					return nil
				}),
			)
		}
	}

	setServiceList := func(machineServices *testutils.MachineServices, response *machine.ServiceListResponse, items ...string) {
		for _, m := range items {
			machineServices.Get(m).SetServiceList(response)
		}
	}

	type etcdStatusSetup func(
		ctx context.Context,
		t *testing.T,
		st state.State,
		machineServices *testutils.MachineServices,
		cluster *omni.Cluster,
		machineSet *omni.MachineSet,
	)

	//nolint:govet
	for _, tt := range []struct {
		setup   etcdStatusSetup
		name    string
		members []*machine.EtcdMember
		check   func(*testing.T, *check.EtcdStatusResult, error)
	}{
		{
			name:    "identity not found",
			members: allMembers,
			check: func(t *testing.T, _ *check.EtcdStatusResult, err error) {
				require.Error(t, err)
			},
		},
		{
			name:    "healthy",
			members: allMembers,
			setup: func(ctx context.Context, t *testing.T, st state.State, machineServices *testutils.MachineServices, _ *omni.Cluster, _ *omni.MachineSet) {
				initIdentities(ctx, t, st, machines...)
				setConnected(ctx, t, st, machines...)
				setServiceList(machineServices, etcdRunning, machines...)
			},
			check: func(t *testing.T, esr *check.EtcdStatusResult, err error) {
				require.NoError(t, err)

				require.Equal(t, 3, esr.HealthyMembers, "unhealthy, %#v", esr.Members)
			},
		},
		{
			name:    "2 of 3 reachable, etcd down",
			members: allMembers,
			setup: func(ctx context.Context, t *testing.T, st state.State, machineServices *testutils.MachineServices, _ *omni.Cluster, _ *omni.MachineSet) {
				initIdentities(ctx, t, st, machines...)
				setConnected(ctx, t, st, machines...)
				setServiceList(machineServices, etcdRunning, machines[:2]...)
				setServiceList(machineServices, etcdDown, machines[2])
			},
			check: func(t *testing.T, esr *check.EtcdStatusResult, err error) {
				require.NoError(t, err)

				require.Equal(t, 2, esr.HealthyMembers, "unhealthy, %#v", esr.Members)
				require.False(t, esr.Members[etcd.FormatMemberID(3)].Healthy, "the machine %q was expected to be unhealthy", machines[2])
			},
		},
		{
			name:    "2 of 4 reachable, identity not ready for a machine",
			members: allMembers,
			setup: func(ctx context.Context, t *testing.T, st state.State, machineServices *testutils.MachineServices, cluster *omni.Cluster, machineSet *omni.MachineSet) {
				initIdentities(ctx, t, st, machines...)
				setConnected(ctx, t, st, machines...)
				setServiceList(machineServices, etcdRunning, machines[:2]...)
				setServiceList(machineServices, etcdDown, machines[2])

				addClusterMachine(ctx, t, st, machineServices, cluster, machineSet, "n4")
			},
			check: func(t *testing.T, esr *check.EtcdStatusResult, err error) {
				require.NoError(t, err)

				require.Equal(t, 2, esr.HealthyMembers, "unhealthy, %#v", esr.Members)
				require.False(t, esr.Members[etcd.FormatMemberID(3)].Healthy, "the machine %q was expected to be unhealthy", machines[2])

				require.Len(t, esr.Members, 3)
			},
		},
		{
			name:    "3 of 4 reachable, non-member is not connected",
			members: allMembers,
			setup: func(ctx context.Context, t *testing.T, st state.State, machineServices *testutils.MachineServices, cluster *omni.Cluster, machineSet *omni.MachineSet) {
				initIdentities(ctx, t, st, machines...)
				setConnected(ctx, t, st, machines...)
				setServiceList(machineServices, etcdRunning, machines...)

				addClusterMachine(ctx, t, st, machineServices, cluster, machineSet, "n4")
			},
			check: func(t *testing.T, esr *check.EtcdStatusResult, err error) {
				require.NoError(t, err)

				require.Equal(t, 3, esr.HealthyMembers, "unhealthy, %#v", esr.Members)
				require.Len(t, esr.Members, 3)
			},
		},
		{
			name:    "4 of 4 reachable, but splitbrain",
			members: allMembers,
			setup: func(ctx context.Context, t *testing.T, st state.State, machineServices *testutils.MachineServices, cluster *omni.Cluster, machineSet *omni.MachineSet) {
				initIdentities(ctx, t, st, machines...)
				setConnected(ctx, t, st, machines...)
				setServiceList(machineServices, etcdRunning, machines...)

				addClusterMachine(ctx, t, st, machineServices, cluster, machineSet, "n4")

				setConnected(ctx, t, st, "n4")
				initIdentities(ctx, t, st, "n4")
				setServiceList(machineServices, etcdRunning, "n4")
			},
			check: func(t *testing.T, esr *check.EtcdStatusResult, err error) {
				require.NoError(t, err)

				require.Equal(t, 3, esr.HealthyMembers, "unhealthy, %#v", esr.Members)
				require.Len(t, esr.Members, 3)
			},
		},
		{
			name:    "extra etcd member",
			members: extraEtcdMembers,
			setup: func(ctx context.Context, t *testing.T, st state.State, machineServices *testutils.MachineServices, _ *omni.Cluster, _ *omni.MachineSet) {
				initIdentities(ctx, t, st, machines...)
				setConnected(ctx, t, st, machines...)
				setServiceList(machineServices, etcdRunning, machines...)
			},
			check: func(t *testing.T, _ *check.EtcdStatusResult, err error) {
				require.Error(t, err)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(ctx, t, testutils.TestOptions{},
				func(context.Context, testutils.TestContext) {},
				func(ctx context.Context, tc testutils.TestContext) {
					machineServices := testutils.NewMachineServices(t, tc.State)
					clusterName := "etcd_status/" + tt.name

					cluster, machineSet := setupEtcdAuditCluster(ctx, t, tc.State, machineServices, clusterName, machines)

					machineServices.Get(machines[0]).SetEtcdMembers(&machine.EtcdMemberListResponse{
						Messages: []*machine.EtcdMembers{{
							Members: tt.members,
						}},
					})

					if tt.setup != nil {
						tt.setup(ctx, t, tc.State, machineServices, cluster, machineSet)
					}

					status, err := check.EtcdStatus(ctx, tc.State, machineSet)
					tt.check(t, status, err)
				},
			)
		})
	}
}
