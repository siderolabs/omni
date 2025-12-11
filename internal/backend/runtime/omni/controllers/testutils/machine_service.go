// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testutils

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

func NewMachineServices(t *testing.T, st state.State) *MachineServices {
	return &MachineServices{
		items: map[string]*MachineServiceMock{},
		st:    st,
		t:     t,
	}
}

// MachineServices keeps the individual machine services.
type MachineServices struct {
	items map[string]*MachineServiceMock
	st    state.State
	t     *testing.T
}

// Get returns the machine service mock.
func (ms *MachineServices) Get(id string) *MachineServiceMock {
	return ms.items[id]
}

// Create the machine service with id.
func (ms *MachineServices) Create(ctx context.Context, id string) *MachineServiceMock {
	ms.items[id] = NewMachineServiceMock(ctx, ms.t, id, ms.st)

	return ms.items[id]
}

// ForEach iterates through the services and calls the function for each.
func (ms *MachineServices) ForEach(cb func(m *MachineServiceMock)) {
	for _, m := range ms.items {
		cb(m)
	}
}

// NewMachineServiceMock creates a new machine service mock.
func NewMachineServiceMock(ctx context.Context, t *testing.T, id string, omniState state.State) *MachineServiceMock {
	var socketPath string

	if stdruntime.GOOS == "darwin" {
		// check if OS is macOS, because of 108 byte limit on unix socket path
		// apply custom folder creation logic
		temp, err := os.MkdirTemp("", "test-*****")
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, os.RemoveAll(temp))
		})

		socketPath = filepath.Join(temp, "socket")
	} else {
		socketPath = filepath.Join(t.TempDir(), "socket")
	}

	address := socketPath + id

	listener, err := (&net.ListenConfig{}).Listen(ctx, "unix", address)
	require.NoError(t, err)

	machineServer := grpc.NewServer()

	st, err := newTalosState(ctx)
	require.NoError(t, err)

	machineService := &MachineServiceMock{
		address:                address,
		State:                  st,
		id:                     id,
		SocketConnectionString: "unix://" + address,
		omniState:              omniState,
	}

	resourceState := server.NewState(machineService.State)

	machine.RegisterMachineServiceServer(machineServer, machineService)
	storage.RegisterStorageServiceServer(machineServer, machineService)
	v1alpha1.RegisterStateServer(machineServer, resourceState)

	eg := panichandler.NewErrGroup()

	eg.Go(func() error {
		for {
			err := machineServer.Serve(listener)
			if err == nil || errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}
		}
	})

	t.Cleanup(func() {
		machineServer.Stop()

		require.NoError(t, eg.Wait())
	})

	return machineService
}

//nolint:govet
type MachineServiceMock struct {
	lock sync.Mutex

	machine.UnimplementedMachineServiceServer
	storage.UnimplementedStorageServiceServer

	id string

	etcdMembers             *machine.EtcdMemberListResponse
	applyRequests           []*machine.ApplyConfigurationRequest
	bootstrapRequests       []*machine.BootstrapRequest
	upgradeRequests         []*machine.UpgradeRequest
	resetRequests           []*machine.ResetRequest
	etcdRecoverRequestCount atomic.Uint64
	files                   map[string][]string
	serviceList             *machine.ServiceListResponse
	etcdLeaveClusterHandler func(context.Context, *machine.EtcdLeaveClusterRequest) (*machine.EtcdLeaveClusterResponse, error)
	versionHandler          func(ctx context.Context, _ *emptypb.Empty) (*machine.VersionResponse, error)

	metaDeleteKeyToCount map[uint32]int
	metaKeys             map[uint32]string

	address                string
	omniState              state.State
	State                  state.State
	SocketConnectionString string

	OnReset  func(context.Context, *machine.ResetRequest, state.State, string) (*machine.ResetResponse, error)
	OnUpdate func(context.Context, *machine.UpgradeRequest, state.State, string) (*machine.UpgradeResponse, error)
}

func (ms *MachineServiceMock) SetEtcdLeaveHandler(callback func(context.Context, *machine.EtcdLeaveClusterRequest) (*machine.EtcdLeaveClusterResponse, error)) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.etcdLeaveClusterHandler = callback
}

func (ms *MachineServiceMock) SetVersionHandler(callback func(ctx context.Context, _ *emptypb.Empty) (*machine.VersionResponse, error)) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.versionHandler = callback
}

func (ms *MachineServiceMock) GetUpgradeRequests() []*machine.UpgradeRequest {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return slices.Clone(ms.upgradeRequests)
}

func (ms *MachineServiceMock) ClearUpgradeRequests() {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.upgradeRequests = nil
}

func (ms *MachineServiceMock) GetMetaDeleteKeyToCount() map[uint32]int {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return maps.Clone(ms.metaDeleteKeyToCount)
}

func (ms *MachineServiceMock) GetMetaKeys() map[uint32]string {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return maps.Clone(ms.metaKeys)
}

func (ms *MachineServiceMock) ApplyConfiguration(_ context.Context, req *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
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

func (ms *MachineServiceMock) GetApplyRequests() []*machine.ApplyConfigurationRequest {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return ms.applyRequests
}

func (ms *MachineServiceMock) Bootstrap(_ context.Context, req *machine.BootstrapRequest) (*machine.BootstrapResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.bootstrapRequests = append(ms.bootstrapRequests, req)

	return &machine.BootstrapResponse{}, nil
}

func (ms *MachineServiceMock) GetBootstrapRequests() []*machine.BootstrapRequest {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return ms.bootstrapRequests
}

func (ms *MachineServiceMock) Reset(ctx context.Context, req *machine.ResetRequest) (*machine.ResetResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.resetRequests = append(ms.resetRequests, req)

	if ms.OnReset != nil {
		return ms.OnReset(ctx, req, ms.omniState, ms.id)
	}

	return &machine.ResetResponse{}, nil
}

func (ms *MachineServiceMock) GetResetRequests() []*machine.ResetRequest {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return ms.resetRequests
}

func (ms *MachineServiceMock) EtcdMemberList(context.Context, *machine.EtcdMemberListRequest) (*machine.EtcdMemberListResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if ms.etcdMembers == nil {
		return nil, status.Error(codes.Unavailable, "member list is not initialized yet")
	}

	return ms.etcdMembers, nil
}

func (ms *MachineServiceMock) EtcdForfeitLeadership(context.Context, *machine.EtcdForfeitLeadershipRequest) (*machine.EtcdForfeitLeadershipResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	return &machine.EtcdForfeitLeadershipResponse{}, nil
}

func (ms *MachineServiceMock) EtcdLeaveCluster(ctx context.Context, req *machine.EtcdLeaveClusterRequest) (*machine.EtcdLeaveClusterResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if ms.etcdLeaveClusterHandler != nil {
		return ms.etcdLeaveClusterHandler(ctx, req)
	}

	return &machine.EtcdLeaveClusterResponse{}, nil
}

func (ms *MachineServiceMock) EtcdRemoveMember(_ context.Context, req *machine.EtcdRemoveMemberRequest) (*machine.EtcdRemoveMemberResponse, error) {
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

func (ms *MachineServiceMock) EtcdRemoveMemberByID(_ context.Context, req *machine.EtcdRemoveMemberByIDRequest) (*machine.EtcdRemoveMemberByIDResponse, error) {
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

func (ms *MachineServiceMock) EtcdRecover(serv machine.MachineService_EtcdRecoverServer) error {
	ms.etcdRecoverRequestCount.Add(1)

	return serv.SendAndClose(&machine.EtcdRecoverResponse{})
}

func (ms *MachineServiceMock) List(req *machine.ListRequest, serv machine.MachineService_ListServer) error {
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

func (ms *MachineServiceMock) Upgrade(ctx context.Context, request *machine.UpgradeRequest) (*machine.UpgradeResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.upgradeRequests = append(ms.upgradeRequests, request)

	if ms.OnUpdate != nil {
		return ms.OnUpdate(ctx, request, ms.omniState, ms.id)
	}

	return &machine.UpgradeResponse{}, nil
}

func (ms *MachineServiceMock) Disks(context.Context, *emptypb.Empty) (*storage.DisksResponse, error) {
	return &storage.DisksResponse{}, nil
}

func (ms *MachineServiceMock) Version(ctx context.Context, _ *emptypb.Empty) (*machine.VersionResponse, error) {
	talosVersion := constants.DefaultTalosVersion

	m, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, ms.omniState, ms.id)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if m != nil {
		talosVersion = m.TypedSpec().Value.TalosVersion
	}

	if ms.versionHandler != nil {
		return ms.versionHandler(ctx, nil)
	}

	return &machine.VersionResponse{
		Messages: []*machine.Version{
			{
				Version: &machine.VersionInfo{
					Tag: "v" + talosVersion,
				},
			},
		},
	}, nil
}

func (ms *MachineServiceMock) GetServiceList(value *machine.ServiceListResponse) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	ms.serviceList = value
}

func (ms *MachineServiceMock) ServiceList(context.Context, *emptypb.Empty) (*machine.ServiceListResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if (ms.serviceList) == nil {
		return nil, status.Errorf(codes.Internal, "service list is not mocked")
	}

	return ms.serviceList, nil
}

func (ms *MachineServiceMock) MetaWrite(_ context.Context, req *machine.MetaWriteRequest) (*machine.MetaWriteResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if ms.metaKeys == nil {
		ms.metaKeys = map[uint32]string{}
	}

	ms.metaKeys[req.Key] = string(req.Value)

	return &machine.MetaWriteResponse{}, nil
}

func (ms *MachineServiceMock) MetaDelete(_ context.Context, req *machine.MetaDeleteRequest) (*machine.MetaDeleteResponse, error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	if ms.metaDeleteKeyToCount == nil {
		ms.metaDeleteKeyToCount = map[uint32]int{}
	}

	ms.metaDeleteKeyToCount[req.Key]++

	delete(ms.metaKeys, req.Key)

	return &machine.MetaDeleteResponse{}, nil
}
