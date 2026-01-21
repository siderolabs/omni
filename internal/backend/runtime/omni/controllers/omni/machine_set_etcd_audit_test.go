// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	stdruntime "runtime"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	talosruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

type MachineSetEtcdAuditSuite struct {
	OmniSuite

	clientFactory *talos.ClientFactory
	resources     []resource.Resource
}

func (suite *MachineSetEtcdAuditSuite) createMachineSet(clusterName string, machineSetName string, machines []string) (map[string]*machineService, *omni.MachineSet) {
	cluster := omni.NewCluster(clusterName)

	services := map[string]*machineService{}

	cluster.TypedSpec().Value.TalosVersion = "1.2.2"
	cluster.TypedSpec().Value.KubernetesVersion = "1.25.0"

	suite.createResource(cluster)

	clusterStatus := omni.NewClusterStatus(clusterName)

	clusterStatus.TypedSpec().Value.Available = true
	clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true

	suite.createResource(clusterStatus)

	machineSet := omni.NewMachineSet(machineSetName)
	machineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	machineSet.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling

	config := omni.NewTalosConfig(clusterName)

	suite.createResource(config)

	for _, machine := range machines {
		services[machine] = suite.createClusterMachineStatus(machine, clusterName, machineSet)

		msn := omni.NewMachineSetNode(machine, machineSet)

		suite.createResource(msn)
	}

	suite.createResource(machineSet)

	return services, machineSet
}

func (suite *MachineSetEtcdAuditSuite) createClusterMachineStatus(machine, clusterName string, machineSet *omni.MachineSet) *machineService {
	serverSuffix := uuid.NewString()

	// If the OS is macOS, trim the random server suffix to avoid hitting the 108 char unix socket path limit
	if stdruntime.GOOS == "darwin" {
		serverSuffix = serverSuffix[:6]
	}

	service, err := suite.newServer(serverSuffix)

	suite.Require().NoError(err)

	clusterMachineStatus := omni.NewClusterMachineStatus(machine)
	clusterMachineStatus.TypedSpec().Value.ManagementAddress = unixSocket + service.address

	clusterMachineStatus.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	clusterMachineStatus.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
	clusterMachineStatus.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())

	suite.createResource(clusterMachineStatus)

	return service
}

func (suite *MachineSetEtcdAuditSuite) SetupTest() {
	suite.OmniSuite.SetupTest()

	logger := zaptest.NewLogger(suite.T())

	suite.clientFactory = talos.NewClientFactory(suite.state, logger)

	if _, err := runtime.Get(talos.Name); err != nil {
		runtime.Install(talos.Name, talos.New(suite.clientFactory, logger))
	}

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterEndpointController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineSetEtcdAuditController(
		suite.clientFactory,
		time.Millisecond*100,
	)))
}

func (suite *MachineSetEtcdAuditSuite) TearDownTest() {
	suite.cleanupResources()

	suite.OmniSuite.TearDownTest()
}

func (suite *MachineSetEtcdAuditSuite) TestEtcdAudit() {
	machines := []string{
		"n1",
		"n2",
		"n3",
	}

	extraMembers := []*machine.EtcdMember{
		{
			Id:       1,
			Hostname: "n1",
		},
		{
			Id:       2,
			Hostname: "n2",
		},
		{
			Id:       3,
			Hostname: "n3",
		},
		{
			Id:       4,
			Hostname: "n4",
		},
	}

	//nolint:govet
	for _, tt := range []struct {
		setup   func(*omni.MachineSet)
		name    string
		members []*machine.EtcdMember
		check   func(*machineService) error
	}{
		{
			name:    "unchanged",
			members: extraMembers,
			check: func(machineService *machineService) error {
				time.Sleep(time.Millisecond * 500)
				machineService.lock.Lock()
				defer machineService.lock.Unlock()

				members := machineService.etcdMembers.Messages[0].Members

				if len(members) != 4 {
					return retry.ExpectedErrorf("expected to have 4 etcd members got %d", len(members))
				}

				return nil
			},
		},
		{
			name:    "cleanedUp",
			members: extraMembers,
			setup: func(ms *omni.MachineSet) {
				for _, id := range machines {
					cm := omni.NewClusterMachine(id)
					cm.Metadata().Labels().Set(omni.LabelMachineSet, ms.Metadata().ID())
					suite.createResource(cm)
				}
			},
			check: func(machineService *machineService) error {
				machineService.lock.Lock()
				defer machineService.lock.Unlock()

				members := machineService.etcdMembers.Messages[0].Members
				if len(members) != 3 {
					return retry.ExpectedErrorf("expected to have 3 etcd members got %d", len(members))
				}

				return nil
			},
		},
	} {
		if !suite.T().Run(tt.name, func(t *testing.T) {
			suite.cleanupResources()

			clusterName := "etcd_audit/" + tt.name

			services, machineSet := suite.createMachineSet(clusterName, tt.name, machines)

			for _, m := range machines {
				svc := services[m]

				svc.lock.Lock()
				svc.etcdMembers = &machine.EtcdMemberListResponse{
					Messages: []*machine.EtcdMembers{
						{
							Members: tt.members,
						},
					},
				}
				svc.lock.Unlock()
			}

			for i, m := range machines {
				suite.Require().NoError(services[m].state.Create(suite.ctx, talosruntime.NewMountStatus(talosruntime.NamespaceName, "EPHEMERAL")))
				member := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
				member.TypedSpec().MemberID = etcd.FormatMemberID(uint64(i + 1))

				suite.Require().NoError(services[m].state.Create(suite.ctx, member))
			}

			if tt.setup != nil {
				tt.setup(machineSet)
			}

			assert.NoError(t, retry.Constant(20*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
				var err error
				for _, m := range machines {
					err = tt.check(services[m])
					if err == nil {
						return nil
					}
				}

				return err
			}))
		}) {
			break
		}
	}
}

//nolint:maintidx
func (suite *MachineSetEtcdAuditSuite) TestEtcdStatus() {
	var (
		services   map[string]*machineService
		machineSet *omni.MachineSet
	)

	members := map[string]uint64{
		"n1": 1,
		"n2": 2,
		"n3": 3,
		"n4": 4,
	}

	allMembers := []*machine.EtcdMember{
		{
			Id:       members["n1"],
			Hostname: "n1",
		},
		{
			Id:       members["n2"],
			Hostname: "n2",
		},
		{
			Id:       members["n3"],
			Hostname: "n3",
		},
	}

	extraEtcdMembers := []*machine.EtcdMember{
		{
			Id:       members["n1"],
			Hostname: "n1",
		},
		{
			Id:       members["n2"],
			Hostname: "n2",
		},
		{
			Id:       members["n3"],
			Hostname: "n3",
		},
		{
			Id:       members["n4"],
			Hostname: "n4",
		},
	}

	machines := []string{
		"n1",
		"n2",
		"n3",
	}

	initIdentities := func(items ...string) {
		for _, m := range items {
			identity := omni.NewClusterMachineIdentity(m)
			identity.TypedSpec().Value.EtcdMemberId = members[m]

			suite.createResource(identity)
		}
	}

	setConnected := func(items ...string) {
		for _, m := range items {
			status := omni.NewClusterMachineStatus(m)

			_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, status.Metadata(), func(res *omni.ClusterMachineStatus) error {
				res.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")

				return nil
			})

			suite.Require().NoError(err)
		}
	}

	setServiceListResponse := func(response *machine.ServiceListResponse, items ...string) {
		for _, m := range items {
			svc, ok := services[m]
			suite.Require().Truef(ok, "the machine %q is not set up in the tests", m)

			svc.setServiceList(response)
		}
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

	//nolint:govet
	for _, tt := range []struct {
		setup   func(string)
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
			setup: func(string) {
				initIdentities(machines...)
				setConnected(machines...)
				setServiceListResponse(etcdRunning, machines...)
			},
			check: func(t *testing.T, esr *check.EtcdStatusResult, err error) {
				require.NoError(t, err)

				require.Equal(t, 3, esr.HealthyMembers, "unhealthy, %#v", esr.Members)
			},
		},
		{
			name:    "2 of 3 reachable, etcd down",
			members: allMembers,
			setup: func(string) {
				initIdentities(machines...)
				setConnected(machines...)
				setServiceListResponse(etcdRunning, machines[:2]...)
				setServiceListResponse(etcdDown, machines[2])
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
			setup: func(clusterName string) {
				initIdentities(machines...)
				setConnected(machines...)
				setServiceListResponse(etcdRunning, machines[:2]...)
				setServiceListResponse(etcdDown, machines[2])

				suite.createClusterMachineStatus("n4", clusterName, machineSet)
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
			setup: func(clusterName string) {
				initIdentities(machines...)
				setConnected(machines...)
				setServiceListResponse(etcdRunning, machines...)

				suite.createClusterMachineStatus("n4", clusterName, machineSet)
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
			setup: func(clusterName string) {
				initIdentities(machines...)
				setConnected(machines...)
				setServiceListResponse(etcdRunning, machines...)

				services["n4"] = suite.createClusterMachineStatus("n4", clusterName, machineSet)
				setConnected("n4")
				initIdentities("n4")
				setServiceListResponse(etcdRunning, "n4")
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
			setup: func(string) {
				initIdentities(machines...)
				setConnected(machines...)
				setServiceListResponse(etcdRunning, machines...)
			},
			check: func(t *testing.T, _ *check.EtcdStatusResult, err error) {
				require.Error(t, err)
			},
		},
	} {
		if !suite.T().Run(tt.name, func(t *testing.T) {
			suite.cleanupResources()

			clusterName := "etcd_status/" + tt.name

			services, machineSet = suite.createMachineSet(clusterName, tt.name, machines)

			primary := services[machines[0]]

			primary.lock.Lock()
			primary.etcdMembers = &machine.EtcdMemberListResponse{
				Messages: []*machine.EtcdMembers{
					{
						Members: tt.members,
					},
				},
			}
			primary.lock.Unlock()

			for i, m := range machines {
				member := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
				member.TypedSpec().MemberID = etcd.FormatMemberID(uint64(i + 1))

				require.NoError(t, services[m].state.Create(suite.ctx, member))
			}

			if tt.setup != nil {
				tt.setup(clusterName)
			}

			status, err := check.EtcdStatus(suite.ctx, suite.state, machineSet)
			tt.check(t, status, err)
		}) {
			break
		}
	}
}

func (suite *MachineSetEtcdAuditSuite) createResource(res resource.Resource) {
	suite.Require().NoError(suite.state.Create(suite.ctx, res))

	suite.resources = append(suite.resources, res)
}

func ignoreNotFound(err error) error {
	if state.IsNotFoundError(err) {
		return nil
	}

	return err
}

func (suite *MachineSetEtcdAuditSuite) cleanupResources() {
	for _, r := range suite.resources {
		_, err := suite.state.Teardown(suite.ctx, r.Metadata())
		suite.Require().NoError(ignoreNotFound(err))

		_, err = suite.state.WatchFor(suite.ctx, r.Metadata(), state.WithFinalizerEmpty())
		suite.Require().NoError(ignoreNotFound(err))

		suite.Require().NoError(ignoreNotFound(suite.state.Destroy(suite.ctx, r.Metadata())))
	}

	suite.resources = nil
}

func TestMachineSetEtcdAuditSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineSetEtcdAuditSuite))
}
