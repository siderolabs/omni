// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router

import (
	"context"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

// operatorMethodSet is the set of methods that are allowed to be called by the minimum role of os:operator.
var operatorMethodSet = xslices.ToSet([]string{
	machine.MachineService_EtcdAlarmDisarm_FullMethodName,
	machine.MachineService_EtcdAlarmList_FullMethodName,
	machine.MachineService_EtcdDefragment_FullMethodName,
	machine.MachineService_EtcdStatus_FullMethodName,
	machine.MachineService_PacketCapture_FullMethodName,
	machine.MachineService_Reboot_FullMethodName,
	machine.MachineService_Restart_FullMethodName,
	machine.MachineService_ServiceRestart_FullMethodName,
	machine.MachineService_ServiceStart_FullMethodName,
	machine.MachineService_ServiceStop_FullMethodName,
	machine.MachineService_Shutdown_FullMethodName,
})

// adminMethodSet is the set of methods that are allowed to be called by the minimum role of os:admin.
var adminMethodSet = xslices.ToSet([]string{
	storage.StorageService_BlockDeviceWipe_FullMethodName,

	machine.MachineService_EtcdDowngradeCancel_FullMethodName,
	machine.MachineService_EtcdDowngradeEnable_FullMethodName,
	machine.MachineService_EtcdDowngradeValidate_FullMethodName,
	machine.MachineService_EtcdForfeitLeadership_FullMethodName,
	machine.MachineService_MetaWrite_FullMethodName,
	machine.MachineService_MetaDelete_FullMethodName,

	machine.DebugService_ContainerRun_FullMethodName,

	machine.ImageService_Remove_FullMethodName,
})

// adminMethodSet1_12 is the set of methods that are allowed to be called by the minimum role of os:admin for Talos versions >= 1.12.0.
var adminMethodSet1_12 = xslices.ToSet([]string{
	// read/copy APIs were not considered safe for older Talos versions, as the STATE partition has always been mounted
	machine.MachineService_Copy_FullMethodName,
	machine.MachineService_Read_FullMethodName,
})

// TalosBackend implements a backend (proxying directly to a single Talos node over SideroLink).
type TalosBackend struct {
	nodeResolver NodeResolver
	omniState    state.State
	conn         *grpc.ClientConn
	verifier     grpc.UnaryServerInterceptor
	talosAuditor TalosAuditor
	name         string
	clusterID    string
	authEnabled  bool
}

// NewTalosBackend builds new Talos API backend.
func NewTalosBackend(
	name, clusterID string,
	nodeResolver NodeResolver,
	conn *grpc.ClientConn,
	authEnabled bool,
	verifier grpc.UnaryServerInterceptor,
	st state.State,
	talosAuditor TalosAuditor,
) *TalosBackend {
	return &TalosBackend{
		name:         name,
		clusterID:    clusterID,
		nodeResolver: nodeResolver,
		conn:         conn,
		authEnabled:  authEnabled,
		verifier:     verifier,
		omniState:    st,
		talosAuditor: talosAuditor,
	}
}

func (backend *TalosBackend) String() string {
	return backend.name
}

// GetConnection returns a grpc connection to the backend.
func (backend *TalosBackend) GetConnection(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	grpcutil.SetShouldLog(ctx, "talos-backend")

	if backend.clusterID != "" {
		grpcutil.AddLogPair(ctx, "cluster", backend.clusterID)
	}

	ctx, err := authenticate(ctx, backend.verifier, backend.authEnabled, md, fullMethodName)
	if err != nil {
		return ctx, nil, err
	}

	ctx, err = accesspolicy.ApplyClusterAccessPolicy(ctx, backend.clusterID, backend.omniState)
	if err != nil {
		return ctx, nil, err
	}

	nodes, err := resolveNodes(backend.nodeResolver, md)
	if err != nil {
		return ctx, nil, err
	}

	hasModifyAccess, accessErr := backend.checkAccess(ctx)

	// the audit entry records the outcome, so it is written once the access is decided and for a refusal too
	if backend.talosAuditor != nil {
		if err = backend.talosAuditor.AuditTalosAccess(ctx, auditlog.TalosAccess{
			FullMethodName: strings.TrimLeft(fullMethodName, "/"),
			ClusterName:    backend.clusterID,
			MachineIP:      getNodeID(md),
			Denied:         accessErr != nil,
		}); err != nil {
			return ctx, nil, err
		}
	}

	if accessErr != nil {
		return ctx, nil, accessErr
	}

	md = md.Copy()

	// Always strip the "node" header — Omni has already resolved and routed directly.
	md.Delete(nodeHeaderKey)

	// Preserve the "nodes" header (rewritten with resolved node addresses) when
	// the original request had "nodes". Talos apid uses it for One2Many fan-out (2+ nodes)
	// or loopback (1 node pointing to self), which sets Metadata.Hostname in the response.
	// This preserves the response shape that talosctl and other API consumers expect.
	// GetAddress() returns the cluster-internal IP when available, falling back to the
	// SideroLink management address during early bootstrap before NodeIPs are populated.
	if len(md.Get(nodesHeaderKey)) > 0 {
		addresses := xslices.Map(nodes, func(info dns.Info) string {
			return info.GetAddress()
		})

		setHeaderData(ctx, md, nodesHeaderKey, addresses...)
	}

	backend.setRoleHeaders(ctx, md, fullMethodName, nodes, hasModifyAccess)

	outCtx := metadata.NewOutgoingContext(ctx, md)

	return outCtx, backend.conn, nil
}

// checkAccess decides whether the caller may reach this backend, and whether it may modify.
func (backend *TalosBackend) checkAccess(ctx context.Context) (bool, error) {
	_, err := auth.CheckGRPC(ctx, auth.WithRole(role.Operator))
	if err == nil {
		return true, nil
	}

	// insecure access mode should only be possible for the operator role users
	if backend.clusterID == "" {
		return false, err
	}

	// at least read access is required
	if _, err = auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
		return false, err
	}

	return false, nil
}

func (backend *TalosBackend) setRoleHeaders(ctx context.Context, md metadata.MD, fullMethodName string, nodes []dns.Info, hasModifyAccess bool) {
	if !hasModifyAccess {
		setHeaderData(ctx, md, constants.APIAuthzRoleMetadataKey, talosrole.MakeSet(talosrole.Reader).Strings()...)

		return
	}

	minTalosVersion := backend.minTalosVersion(nodes)

	// methods that should have admin access
	if _, ok := adminMethodSet[fullMethodName]; ok {
		setHeaderData(ctx, md, constants.APIAuthzRoleMetadataKey, talosrole.MakeSet(talosrole.Admin).Strings()...)

		return
	}

	// methods that should have admin access for Talos >= 1.12.0
	if _, ok := adminMethodSet1_12[fullMethodName]; ok {
		if minTalosVersion != nil && minTalosVersion.GTE(semver.MustParse("1.12.0")) {
			setHeaderData(ctx, md, constants.APIAuthzRoleMetadataKey, talosrole.MakeSet(talosrole.Admin).Strings()...)

			return
		}
	}

	// min Talos version is >= 1.4.0, we can use Operator role
	if minTalosVersion != nil && minTalosVersion.GTE(semver.MustParse("1.4.0")) {
		setHeaderData(ctx, md, constants.APIAuthzRoleMetadataKey, talosrole.MakeSet(talosrole.Operator).Strings()...)

		return
	}

	// min Talos version is unknown or < 1.4.0, fallback to backwards-compatibility logic
	if _, ok := operatorMethodSet[fullMethodName]; ok {
		setHeaderData(ctx, md, constants.APIAuthzRoleMetadataKey, talosrole.MakeSet(talosrole.Admin).Strings()...)
	} else {
		setHeaderData(ctx, md, constants.APIAuthzRoleMetadataKey, talosrole.MakeSet(talosrole.Reader).Strings()...)
	}
}

func (backend *TalosBackend) minTalosVersion(nodes []dns.Info) *semver.Version {
	var ver *semver.Version

	for _, node := range nodes {
		nodeVer := takePtr(semver.ParseTolerant(node.TalosVersion))
		if nodeVer != nil && (ver == nil || nodeVer.LT(*ver)) {
			ver = nodeVer
		}
	}

	return ver
}

func takePtr[T any](v T, err error) *T {
	if err != nil {
		return nil
	}

	return &v
}

// AppendInfo is called to enhance response from the backend with additional data.
func (backend *TalosBackend) AppendInfo(_ bool, resp []byte) ([]byte, error) {
	return resp, nil
}

// BuildError is called to convert error from upstream into response field.
func (backend *TalosBackend) BuildError(bool, error) ([]byte, error) {
	return nil, nil
}

func setHeaderData(ctx context.Context, md metadata.MD, k string, v ...string) {
	if len(v) == 0 {
		return
	}

	md.Set(k, v...)

	if len(v) == 1 {
		grpcutil.AddLogPair(ctx, k, v[0])
	} else {
		grpcutil.AddLogPair(ctx, k, v)
	}
}
