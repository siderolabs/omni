// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	cosiv1alpha1 "github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	rt "github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
)

const (
	testInstallDisk  = "/dev/vda"
	TalosVersion     = "1.3.0"
	unixSocket       = "unix://"
	defaultSchematic = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"
)

//nolint:govet
type machineService struct {
	lock sync.Mutex

	machine.UnimplementedMachineServiceServer
	storage.UnimplementedStorageServiceServer

	etcdMembers             *machine.EtcdMemberListResponse
	resetChan               chan *machine.ResetRequest
	applyRequests           []*machine.ApplyConfigurationRequest
	bootstrapRequests       []*machine.BootstrapRequest
	upgradeRequests         []*machine.UpgradeRequest
	resetRequests           []*machine.ResetRequest
	etcdRecoverRequestCount atomic.Uint64
	files                   map[string][]string
	serviceList             *machine.ServiceListResponse
	etcdLeaveClusterHandler func(context.Context, *machine.EtcdLeaveClusterRequest) (*machine.EtcdLeaveClusterResponse, error)

	address string
	state   state.State
}

func (ms *machineService) getUpgradeRequests() []*machine.UpgradeRequest {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return slices.Clone(ms.upgradeRequests)
}

func (ms *machineService) clearUpgradeRequests() {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.upgradeRequests = nil
}

func (ms *machineService) ApplyConfiguration(_ context.Context, req *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.applyRequests = append(ms.applyRequests, req)

	return &machine.ApplyConfigurationResponse{
		Messages: []*machine.ApplyConfiguration{
			{
				Mode: machine.ApplyConfigurationRequest_NO_REBOOT,
			},
		},
	}, nil
}

func (ms *machineService) getApplyRequests() []*machine.ApplyConfigurationRequest {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return ms.applyRequests
}

func (ms *machineService) Bootstrap(_ context.Context, req *machine.BootstrapRequest) (*machine.BootstrapResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.bootstrapRequests = append(ms.bootstrapRequests, req)

	return &machine.BootstrapResponse{}, nil
}

func (ms *machineService) getBootstrapRequests() []*machine.BootstrapRequest {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return ms.bootstrapRequests
}

func (ms *machineService) Reset(_ context.Context, req *machine.ResetRequest) (*machine.ResetResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.resetRequests = append(ms.resetRequests, req)

	select {
	case ms.resetChan <- req:
	default:
	}

	return &machine.ResetResponse{}, nil
}

func (ms *machineService) getResetRequests() []*machine.ResetRequest {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return ms.resetRequests
}

func (ms *machineService) EtcdMemberList(context.Context, *machine.EtcdMemberListRequest) (*machine.EtcdMemberListResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if ms.etcdMembers == nil {
		return nil, status.Error(codes.Unavailable, "member list is not initialized yet")
	}

	return ms.etcdMembers, nil
}

func (ms *machineService) EtcdForfeitLeadership(context.Context, *machine.EtcdForfeitLeadershipRequest) (*machine.EtcdForfeitLeadershipResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return &machine.EtcdForfeitLeadershipResponse{}, nil
}

func (ms *machineService) EtcdLeaveCluster(ctx context.Context, req *machine.EtcdLeaveClusterRequest) (*machine.EtcdLeaveClusterResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if ms.etcdLeaveClusterHandler != nil {
		return ms.etcdLeaveClusterHandler(ctx, req)
	}

	return &machine.EtcdLeaveClusterResponse{}, nil
}

func (ms *machineService) EtcdRemoveMember(_ context.Context, req *machine.EtcdRemoveMemberRequest) (*machine.EtcdRemoveMemberResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	for _, msg := range ms.etcdMembers.Messages {
		for i, member := range msg.Members {
			if member.Hostname == req.Member {
				msg.Members = append(msg.Members[:i], msg.Members[i+1:]...)

				return &machine.EtcdRemoveMemberResponse{}, nil
			}
		}
	}

	return nil, fmt.Errorf("member with hostname %s not found", req.Member)
}

func (ms *machineService) EtcdRemoveMemberByID(_ context.Context, req *machine.EtcdRemoveMemberByIDRequest) (*machine.EtcdRemoveMemberByIDResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	for _, msg := range ms.etcdMembers.Messages {
		for i, member := range msg.Members {
			if member.Id == req.MemberId {
				msg.Members = append(msg.Members[:i], msg.Members[i+1:]...)

				return &machine.EtcdRemoveMemberByIDResponse{}, nil
			}
		}
	}

	return nil, fmt.Errorf("member with id %s not found", etcd.FormatMemberID(req.MemberId))
}

func (ms *machineService) EtcdRecover(serv machine.MachineService_EtcdRecoverServer) error {
	ms.etcdRecoverRequestCount.Add(1)

	return serv.SendAndClose(&machine.EtcdRecoverResponse{})
}

func (ms *machineService) List(req *machine.ListRequest, serv machine.MachineService_ListServer) error {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	for _, f := range ms.files[req.GetRoot()] {
		if err := serv.Send(&machine.FileInfo{
			Name: f,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (ms *machineService) Upgrade(_ context.Context, request *machine.UpgradeRequest) (*machine.UpgradeResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.upgradeRequests = append(ms.upgradeRequests, request)

	return &machine.UpgradeResponse{}, nil
}

func (ms *machineService) Disks(context.Context, *emptypb.Empty) (*storage.DisksResponse, error) {
	return &storage.DisksResponse{}, nil
}

func (ms *machineService) Version(context.Context, *emptypb.Empty) (*machine.VersionResponse, error) {
	return &machine.VersionResponse{
		Messages: []*machine.Version{
			{
				Version: &machine.VersionInfo{
					Tag: "v" + TalosVersion,
				},
			},
		},
	}, nil
}

func (ms *machineService) setServiceList(value *machine.ServiceListResponse) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.serviceList = value
}

func (ms *machineService) ServiceList(context.Context, *emptypb.Empty) (*machine.ServiceListResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if (ms.serviceList) == nil {
		return nil, status.Errorf(codes.Internal, "service list is not mocked")
	}

	return ms.serviceList, nil
}

type OmniSuite struct { //nolint:govet
	suite.Suite

	state        state.State
	stateBuilder dynamicStateBuilder

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	grpcServers            []*grpc.Server
	socketPath             string
	socketConnectionString string

	machineService *machineService

	statesMu sync.Mutex
	states   map[string]*server.State
}

// Start a mock gRPC server on the unix socket which is using a temp file,
// to avoid clashing with other parallel test runs.
// This server is used as a fake endpoint for each node we create for the cluster.
// The single server is used for all nodes.
func (suite *OmniSuite) newServer(suffix string, opts ...grpc.ServerOption) (*machineService, error) {
	address := suite.socketPath + suffix

	listener, err := net.Listen("unix", address)
	if err != nil {
		return nil, err
	}

	machineServer := grpc.NewServer(opts...)
	suite.grpcServers = append(suite.grpcServers, machineServer)

	st, err := newTalosState(suite.ctx)
	suite.Require().NoError(err)

	machineService := &machineService{
		resetChan: make(chan *machine.ResetRequest, 10),
		address:   address,
		state:     st,
	}

	suite.statesMu.Lock()
	defer suite.statesMu.Unlock()

	if suite.states == nil {
		suite.states = make(map[string]*server.State)
	}

	resourceState := server.NewState(machineService.state)

	machine.RegisterMachineServiceServer(machineServer, machineService)
	storage.RegisterStorageServiceServer(machineServer, machineService)
	cosiv1alpha1.RegisterStateServer(machineServer, resourceState)

	go func() {
		for {
			err := machineServer.Serve(listener)
			if err == nil || errors.Is(err, grpc.ErrServerStopped) {
				break
			}
		}
	}()

	return machineService, nil
}

func (suite *OmniSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 20*time.Second)

	suite.stateBuilder = dynamicStateBuilder{m: map[resource.Namespace]state.CoreState{}}

	suite.state = state.WrapCore(namespaced.NewState(suite.stateBuilder.Builder))

	var err error

	logger := zaptest.NewLogger(suite.T())

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
	suite.Require().NoError(err)

	k8s, err := kubernetes.New(suite.state)
	suite.Require().NoError(err)
	rt.Install(kubernetes.Name, k8s)

	if stdruntime.GOOS == "darwin" {
		var temp string

		// check if OS is macOS, because of 108 byte limit on unix socket path
		// apply custom folder creation logic
		temp, err = os.MkdirTemp("", "test-*****")
		suite.Require().NoError(err)

		suite.T().Cleanup(func() {
			suite.Require().NoError(os.RemoveAll(temp))
		})

		suite.socketPath = filepath.Join(temp, "socket")
	} else {
		suite.socketPath = filepath.Join(suite.T().TempDir(), "socket")
	}

	suite.socketConnectionString = unixSocket + suite.socketPath
	suite.machineService, err = suite.newServer("")
	suite.Require().NoError(err)
}

func (suite *OmniSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *OmniSuite) assertNoResource(md resource.Metadata) func() error {
	return func() error {
		_, err := suite.state.Get(suite.ctx, md)
		if err == nil {
			return retry.ExpectedErrorf("resource %s still exists", md)
		}

		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}
}

func (suite *OmniSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	for _, s := range suite.grpcServers {
		s.Stop()
	}
}

func assertResource[R rtestutils.ResourceWithRD](suite *OmniSuite, md interface{ ID() resource.ID }, assertionFunc func(r R, assertion *assert.Assertions)) {
	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []resource.ID{md.ID()}, assertionFunc)
}

func assertNoResource[R rtestutils.ResourceWithRD](suite *OmniSuite, r R) {
	rtestutils.AssertNoResource[R](suite.ctx, suite.T(), suite.state, r.Metadata().ID())
}

func (suite *OmniSuite) createCluster(clusterName string, controlPlanes, workers int) (*omni.Cluster, []*omni.ClusterMachine) {
	cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)
	cluster.TypedSpec().Value.TalosVersion = TalosVersion
	cluster.TypedSpec().Value.KubernetesVersion = "1.24.1"

	machines := make([]*omni.ClusterMachine, controlPlanes+workers)

	cpMachineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(clusterName))
	workersMachineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.WorkersResourceID(clusterName))

	cpMachineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	cpMachineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	workersMachineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	workersMachineSet.Metadata().Labels().Set(omni.LabelWorkerRole, "")

	cpMachineSet.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling
	workersMachineSet.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling

	suite.Require().NoError(suite.state.Create(suite.ctx, cpMachineSet))
	suite.Require().NoError(suite.state.Create(suite.ctx, workersMachineSet))

	for i := range machines {
		clusterMachine := omni.NewClusterMachine(
			resources.DefaultNamespace,
			fmt.Sprintf("%s-node-%d", clusterName, i),
		)

		clusterMachineConfigPatches := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, clusterMachine.Metadata().ID())
		clusterMachineConfigPatches.TypedSpec().Value.Patches = []string{
			`machine:
      install:
        disk: ` + testInstallDisk,
		}

		clusterMachine.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

		var machineSetNode *omni.MachineSetNode

		machineStatus := omni.NewMachineStatus(
			resources.DefaultNamespace,
			clusterMachine.Metadata().ID(),
		)

		machineState := omni.NewMachine(
			resources.DefaultNamespace,
			clusterMachine.Metadata().ID(),
		)

		machineStatus.TypedSpec().Value.ManagementAddress = suite.socketConnectionString
		machineStatus.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
			Id: defaultSchematic,
		}
		machineStatus.TypedSpec().Value.InitialTalosVersion = cluster.TypedSpec().Value.TalosVersion
		machineStatus.TypedSpec().Value.SecureBootStatus = &specs.SecureBootStatus{
			Enabled: false,
		}

		if i < controlPlanes {
			clusterMachine.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
			clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, cpMachineSet.Metadata().ID())

			machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_CONTROL_PLANE

			machineSetNode = omni.NewMachineSetNode(resources.DefaultNamespace, clusterMachine.Metadata().ID(), cpMachineSet)
		} else {
			clusterMachine.Metadata().Labels().Set(omni.LabelWorkerRole, "")
			clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, workersMachineSet.Metadata().ID())

			machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_WORKER

			machineSetNode = omni.NewMachineSetNode(resources.DefaultNamespace, clusterMachine.Metadata().ID(), workersMachineSet)
		}

		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, clusterName)

		machineStatus.TypedSpec().Value.Cluster = clusterName

		machines[i] = clusterMachine

		suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachineConfigPatches))
		suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachine))
		suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))
		suite.Require().NoError(suite.state.Create(suite.ctx, machineState))
		suite.Require().NoError(suite.state.Create(suite.ctx, machineSetNode))
	}

	// create loadbalancer lbConfig as it's port is used while generating kubernetes endpoint
	lbConfig := omni.NewLoadBalancerConfig(resources.DefaultNamespace, clusterName)
	lbConfig.TypedSpec().Value.BindPort = "6443"
	lbConfig.TypedSpec().Value.SiderolinkEndpoint = "https://siderolink:6443"

	if err := suite.state.Create(suite.ctx, lbConfig); err != nil && !state.IsConflictError(err) {
		suite.Assert().NoError(err)
	}

	suite.Assert().NoError(suite.state.Create(suite.ctx, cluster))

	return cluster, machines
}

func (suite *OmniSuite) destroyCluster(cluster *omni.Cluster) {
	suite.destroyClusterByID(cluster.Metadata().ID())
}

func (suite *OmniSuite) destroyClusterByID(clusterID string) {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	list, err := safe.StateListAll[*omni.ClusterMachine](ctx, suite.state,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)),
	)

	suite.Require().NoError(err)

	for iter := list.Iterator(); iter.Next(); {
		rtestutils.Destroy[*omni.MachineSetNode](ctx, suite.T(), suite.state, []string{iter.Value().Metadata().ID()})
		rtestutils.Destroy[*omni.ClusterMachine](ctx, suite.T(), suite.state, []string{iter.Value().Metadata().ID()})
		rtestutils.Destroy[*omni.ClusterMachineStatus](ctx, suite.T(), suite.state, []string{iter.Value().Metadata().ID()})
		rtestutils.Destroy[*omni.ClusterMachineConfigPatches](ctx, suite.T(), suite.state, []string{iter.Value().Metadata().ID()})
		rtestutils.Destroy[*omni.MachineStatus](ctx, suite.T(), suite.state, []string{iter.Value().Metadata().ID()})
		rtestutils.Destroy[*omni.Machine](ctx, suite.T(), suite.state, []string{iter.Value().Metadata().ID()})
	}

	machineSets, err := safe.StateListAll[*omni.MachineSet](ctx, suite.state,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)),
	)

	suite.Require().NoError(err)

	for iter := machineSets.Iterator(); iter.Next(); {
		rtestutils.Destroy[*omni.MachineSet](ctx, suite.T(), suite.state, []string{iter.Value().Metadata().ID()})
	}

	rtestutils.Destroy[*omni.Cluster](ctx, suite.T(), suite.state, []string{clusterID})

	assertNoResource[*omni.BackupData](suite, omni.NewBackupData(clusterID))
	assertNoResource[*omni.EtcdBackupStatus](suite, omni.NewEtcdBackupStatus(clusterID))
}

type dynamicStateBuilder struct { //nolint:govet
	mx sync.Mutex
	m  map[resource.Namespace]state.CoreState
}

func (b *dynamicStateBuilder) Builder(ns resource.Namespace) state.CoreState {
	b.mx.Lock()
	defer b.mx.Unlock()

	if s, ok := b.m[ns]; ok {
		return s
	}

	s := inmem.Build(ns)

	b.m[ns] = s

	return s
}

func (b *dynamicStateBuilder) Set(ns resource.Namespace, state state.CoreState) {
	b.mx.Lock()
	defer b.mx.Unlock()

	if _, ok := b.m[ns]; ok {
		panic(fmt.Errorf("state for namespace %s already exists", ns))
	}

	b.m[ns] = state
}
