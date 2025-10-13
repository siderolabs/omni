// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	xmaps "github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// MachineSetEtcdAuditController runs etcd audit for control plane machine sets.
// It tracks and removes the orphaned etcd members.
type MachineSetEtcdAuditController = qtransform.QController[*omni.MachineSet, *omni.EtcdAuditResult]

var errSkipNode = errors.New("skip node audit")

// NewMachineSetEtcdAuditController initializes MachineSetEtcdAuditController.
//
// memberRemoveTimeout defines the interval between two checks: member is removed if two consequent checks mark it as orphaned.
func NewMachineSetEtcdAuditController(talosClientFactory *talos.ClientFactory, memberRemoveTimeout time.Duration) *MachineSetEtcdAuditController {
	var (
		requeueAfterDuration = memberRemoveTimeout + time.Second
		auditor              = etcdAuditor{
			talosClientFactory:       talosClientFactory,
			memberRemoveTimeout:      memberRemoveTimeout,
			requeueAfterDuration:     requeueAfterDuration,
			clusterToOrphanedMembers: map[string]map[uint64]time.Time{},
		}
	)

	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineSet, *omni.EtcdAuditResult]{
			Name: "MachineSetEtcdAuditController",
			MapMetadataOptionalFunc: func(machineSet *omni.MachineSet) optional.Optional[*omni.EtcdAuditResult] {
				cluster, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					return optional.None[*omni.EtcdAuditResult]()
				}

				if _, isCP := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole); !isCP {
					return optional.None[*omni.EtcdAuditResult]()
				}

				return optional.Some(omni.NewEtcdAuditResult(resources.DefaultNamespace, cluster))
			},
			UnmapMetadataFunc: func(etcdAuditResult *omni.EtcdAuditResult) *omni.MachineSet {
				return omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(etcdAuditResult.Metadata().ID()))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineSet *omni.MachineSet, etcdAuditResult *omni.EtcdAuditResult) error {
				logger.Debug("etcd audit: running for machine set", zap.String("machine_set", machineSet.Metadata().ID()))

				cluster, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine set %q has no cluster label", machineSet.Metadata().ID())
				}

				logger = logger.With(zap.String("cluster", cluster))

				clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](ctx, r, omni.NewClusterStatus(resources.DefaultNamespace, cluster).Metadata())
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("no ClusterStatus found for cluster %q: %w", cluster, err)
					}

					return fmt.Errorf("error getting cluster status for cluster %q: %w", cluster, err)
				}

				if !clusterStatus.TypedSpec().Value.HasConnectedControlPlanes {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine set %q has no connected control planes", machineSet.Metadata().ID())
				}

				talosCli, err := auditor.getClient(ctx, r, cluster)
				if err != nil {
					return err
				}

				orphanMemberSet, err := auditor.auditEtcd(ctx, r, talosCli, cluster, machineSet, logger)
				if err != nil {
					return err
				}

				// update the orphan member set with the found orphans
				membersToRemove := auditor.updateOrphanMembers(cluster, orphanMemberSet, logger)

				// there are no orphans, no need to requeue
				if len(orphanMemberSet) == 0 {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("no orphaned etcd members")
				}

				// there are orphans, but none of them are ready to be removed yet, requeue
				if len(membersToRemove) == 0 {
					// return an error here instead of a simple Requeue request, so that the etcdAuditResult stays unchanged
					return controller.NewRequeueErrorf(requeueAfterDuration, "no orphaned etcd members ready to be removed, requeue")
				}

				removedMembers := make([]uint64, 0, len(membersToRemove))

				for _, member := range membersToRemove {
					if err = auditor.removeMember(ctx, talosCli, cluster, member); err != nil {
						logger.Error("etcd audit: failed to remove orphan", zap.Error(err), zap.String("id", etcd.FormatMemberID(member)))

						continue
					}

					logger.Info("etcd audit: removed orphan", zap.String("id", etcd.FormatMemberID(member)))

					removedMembers = append(removedMembers, member)
				}

				if len(removedMembers) == 0 {
					// no members were removed in this run - skip updating the etcdAuditResult and requeue with an error
					return controller.NewRequeueErrorf(requeueAfterDuration, "failed to remove any of the orphaned etcd members")
				}

				// there was at least one removed member

				slices.Sort(removedMembers)

				etcdAuditResult.TypedSpec().Value.EtcdMemberIds = removedMembers

				if len(removedMembers) < len(membersToRemove) {
					// not all members were removed - requeue the audit without an explicit error, so that etcdAuditResult will still be updated with the last removed members
					return controller.NewRequeueInterval(requeueAfterDuration)
				}

				// all orphans were removed successfully
				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterStatus](
			// Map the cluster ID to the control planes machine set resource ID:
			// Example: my-cluster -> my-cluster-control-planes
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, cs controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				id := omni.ControlPlanesResourceID(cs.ID())

				return []resource.Pointer{omni.NewMachineSet(cs.Namespace(), id).Metadata()}, nil
			},
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineIdentity](
			mappers.MapByMachineSetLabelOnlyControlplane[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedDestroyReadyInput[*omni.ClusterMachine](
			mappers.MapByMachineSetLabel[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.TalosConfig](
			// Map the cluster ID to the control planes machine set resource ID:
			// Example: my-cluster -> my-cluster-control-planes
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, tc controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				id := omni.ControlPlanesResourceID(tc.ID())

				return []resource.Pointer{omni.NewMachineSet(tc.Namespace(), id).Metadata()}, nil
			},
		),
		qtransform.WithConcurrency(4),
	)
}

// etcdAuditor provides various functionality to audit etcd members.
type etcdAuditor struct {
	talosClientFactory *talos.ClientFactory

	clusterToOrphanedMembers     map[string]map[uint64]time.Time
	clusterToOrphanedMembersLock sync.Mutex

	memberRemoveTimeout  time.Duration
	requeueAfterDuration time.Duration
}

func (auditor *etcdAuditor) updateOrphanMembers(cluster string, currentOrphanedMembers map[uint64]struct{}, logger *zap.Logger) (membersToRemove []uint64) {
	auditor.clusterToOrphanedMembersLock.Lock()
	defer auditor.clusterToOrphanedMembersLock.Unlock()

	// log the previous orphans that are no longer orphans after the check is done
	noMoreOrphansSet := xslices.ToSetFunc(xmaps.Keys(auditor.clusterToOrphanedMembers[cluster]), etcd.FormatMemberID)

	defer func() {
		// remove all orphans that are still orphans
		for orphan := range auditor.clusterToOrphanedMembers[cluster] {
			delete(noMoreOrphansSet, etcd.FormatMemberID(orphan))
		}

		if len(noMoreOrphansSet) > 0 {
			noMoreOrphans := xmaps.Keys(noMoreOrphansSet)

			slices.Sort(noMoreOrphans)

			logger.Info("etcd audit: mark members as no longer orphaned", zap.Strings("members", noMoreOrphans))
		}
	}()

	// no orphans at the current check, cluster is ok, remove it from the map
	if len(currentOrphanedMembers) == 0 {
		delete(auditor.clusterToOrphanedMembers, cluster)

		return nil
	}

	// initialize the orphaned member set for the cluster if it does not exist
	if _, ok := auditor.clusterToOrphanedMembers[cluster]; !ok {
		auditor.clusterToOrphanedMembers[cluster] = map[uint64]time.Time{}
	}

	// remove the existing orphans which are not orphans anymore
	for existingOrphanedMember := range auditor.clusterToOrphanedMembers[cluster] {
		if _, ok := currentOrphanedMembers[existingOrphanedMember]; !ok {
			delete(auditor.clusterToOrphanedMembers[cluster], existingOrphanedMember)
		}
	}

	// add new orphans with orphanedAt timestamp of now
	for orphanedMember := range currentOrphanedMembers {
		if _, ok := auditor.clusterToOrphanedMembers[cluster][orphanedMember]; !ok {
			auditor.clusterToOrphanedMembers[cluster][orphanedMember] = time.Now()
		}
	}

	// filter the orphans which have stayed as orphans long enough, so they can be removed
	membersToRemove = make([]uint64, 0, len(auditor.clusterToOrphanedMembers[cluster]))

	for orphanedMember, orphanedAt := range auditor.clusterToOrphanedMembers[cluster] {
		if time.Since(orphanedAt) >= auditor.memberRemoveTimeout {
			membersToRemove = append(membersToRemove, orphanedMember)
		}
	}

	return membersToRemove
}

// auditEtcd returns the set of orphaned members.
func (auditor *etcdAuditor) auditEtcd(ctx context.Context, r controller.Reader, cli *talos.Client, cluster string, machineSet *omni.MachineSet, logger *zap.Logger) (map[uint64]struct{}, error) {
	members := map[uint64]struct{}{}

	listCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	response, err := cli.EtcdMemberList(listCtx, &machine.EtcdMemberListRequest{})
	if err != nil {
		return nil, fmt.Errorf("error listing etcd members for cluster %q: %w", cluster, err)
	}

	for _, memberList := range response.Messages {
		for _, member := range memberList.Members {
			members[member.Id] = struct{}{}
		}
	}

	list, err := safe.ReaderListAll[*omni.ClusterMachine](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())),
	)
	if err != nil {
		return nil, fmt.Errorf("error listing cluster machines for cluster %q: %w", cluster, err)
	}

	// skip etcd audit if no cluster machines were found
	if list.Len() == 0 {
		return nil, nil //nolint:nilnil
	}

	if err = auditor.ensureSupportedVersion(ctx, cli, cluster); err != nil {
		return nil, err
	}

	filteredMembers := maps.Clone(members)
	machineIDToMemberID := make(map[string]string, list.Len())

	for cm := range list.All() {
		machineID := cm.Metadata().ID()

		memberID, err := auditor.auditMember(ctx, r, machineID, cluster)
		if err != nil {
			return nil, err
		}

		if memberID != 0 {
			delete(filteredMembers, memberID)

			machineIDToMemberID[machineID] = etcd.FormatMemberID(memberID)
		} else {
			machineIDToMemberID[machineID] = "(none)"
		}
	}

	if len(filteredMembers) > 0 {
		allMembers := xmaps.KeysFunc(members, etcd.FormatMemberID)
		orphanedMembers := xmaps.KeysFunc(filteredMembers, etcd.FormatMemberID)

		slices.Sort(allMembers)
		slices.Sort(orphanedMembers)

		logger.Info(
			"etcd audit: found orphans",
			zap.Strings("all_members", allMembers),
			zap.Strings("orphaned_members", orphanedMembers),
			zap.Any("machine_id_to_member_id", machineIDToMemberID),
		)
	}

	return filteredMembers, nil
}

// auditMember audits the etcd member in the given machine. It returns the member ID if it is ok (not an orphan).
func (auditor *etcdAuditor) auditMember(ctx context.Context, r controller.Reader, machine, clusterName string) (uint64, error) {
	clusterMachineIdentity, err := safe.ReaderGetByID[*omni.ClusterMachineIdentity](ctx, r, machine)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, err
	}

	if clusterMachineIdentity != nil && clusterMachineIdentity.TypedSpec().Value.EtcdMemberId != 0 {
		return clusterMachineIdentity.TypedSpec().Value.EtcdMemberId, nil
	}

	cli, err := auditor.getNodeClient(ctx, r, clusterName, machine)
	if err != nil {
		if errors.Is(err, errSkipNode) {
			return 0, nil
		}

		return 0, err
	}

	defer cli.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)

	defer cancel()

	var (
		hasEtcdDirectory bool
		ephemeralMounted bool
		etcdMember       *etcd.Member
	)

	if ephemeralMounted, err = auditor.checkEphemeralMount(ctx, cli); err != nil {
		return 0, err
	}

	if !ephemeralMounted {
		requeueErr := fmt.Errorf("etcd audit skipped: machine %q from cluster %q doesn't have ephemeral partition mounted", machine, clusterName)

		return 0, controller.NewRequeueError(requeueErr, auditor.requeueAfterDuration)
	}

	if hasEtcdDirectory, err = auditor.checkEtcdDirectory(ctx, cli); err != nil {
		return 0, err
	}

	if etcdMember, err = auditor.getEtcdMember(ctx, cli); err != nil {
		return 0, err
	}

	// skip audit for the member that doesn't have etcd running
	if !hasEtcdDirectory && etcdMember == nil {
		return 0, nil
	}

	if hasEtcdDirectory && etcdMember == nil {
		requeueErr := fmt.Errorf("etcd audit skipped: machine %q from cluster %q still joining the cluster", machine, clusterName)

		return 0, controller.NewRequeueError(requeueErr, auditor.requeueAfterDuration)
	}

	id, err := etcd.ParseMemberID(etcdMember.TypedSpec().MemberID)
	if err != nil {
		return 0, fmt.Errorf("error parsing member ID: %w", err)
	}

	return id, nil
}

func (auditor *etcdAuditor) removeMember(ctx context.Context, cli *talos.Client, cluster string, memberID uint64) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	if err := cli.EtcdRemoveMemberByID(ctx, &machine.EtcdRemoveMemberByIDRequest{
		MemberId: memberID,
	}); err != nil {
		return err
	}

	auditor.clusterToOrphanedMembersLock.Lock()
	defer auditor.clusterToOrphanedMembersLock.Unlock()

	delete(auditor.clusterToOrphanedMembers[cluster], memberID)

	if len(auditor.clusterToOrphanedMembers[cluster]) == 0 {
		delete(auditor.clusterToOrphanedMembers, cluster)
	}

	return nil
}

func (auditor *etcdAuditor) getNodeClient(ctx context.Context, r controller.Reader, cluster, id string) (*client.Client, error) {
	clusterMachineStatus, err := safe.ReaderGet[*omni.ClusterMachineStatus](ctx, r, omni.NewClusterMachineStatus(resources.DefaultNamespace, id).Metadata())
	if err != nil {
		return nil, err
	}

	if clusterMachineStatus.TypedSpec().Value.IsRemoved {
		return nil, errSkipNode
	}

	talosConfig, err := safe.ReaderGet[*omni.TalosConfig](ctx, r, omni.NewTalosConfig(resources.DefaultNamespace, cluster).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("talosconfig for cluster %q is not found: %w", cluster, err)
		}

		return nil, fmt.Errorf("cluster %q failed to get talosconfig: %w", cluster, err)
	}

	if clusterMachineStatus.TypedSpec().Value.ManagementAddress == "" {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %q doesn't have management address", id)
	}

	endpoints := []string{clusterMachineStatus.TypedSpec().Value.ManagementAddress}

	opts := talos.GetSocketOptions(clusterMachineStatus.TypedSpec().Value.ManagementAddress)

	if opts == nil {
		opts = append(opts, client.WithEndpoints(clusterMachineStatus.TypedSpec().Value.ManagementAddress))
	}

	opts = append(opts, client.WithConfig(omni.NewTalosClientConfig(talosConfig, endpoints...)))

	result, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client to machine %q: %w", id, err)
	}

	return result, nil
}

func (auditor *etcdAuditor) getClient(ctx context.Context, r controller.Reader, cluster string) (*talos.Client, error) {
	c, err := auditor.talosClientFactory.Get(ctx, cluster)
	if err != nil {
		if talos.IsClientNotReadyError(err) {
			return nil, xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return nil, fmt.Errorf("error getting talos client for cluster %q: %w", cluster, err)
	}

	connected, err := c.Connected(ctx, r)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return nil, fmt.Errorf("error checking if client is connected: %w", err)
	}

	if !connected {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("the cluster is not available")
	}

	return c, nil
}

func (auditor *etcdAuditor) ensureSupportedVersion(ctx context.Context, c *talos.Client, clusterName string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	resp, err := c.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to get versions, cluster %q: %w", clusterName, err)
	}

	for _, m := range resp.Messages {
		v := strings.TrimLeft(m.Version.Tag, "v")

		var version semver.Version

		hostname := "*"

		if m.Metadata != nil {
			hostname = m.Metadata.Hostname
		}

		version, err = semver.Parse(v)
		if err != nil {
			return fmt.Errorf("failed to get versions, machine %q, cluster %q: %w", hostname, clusterName, err)
		}

		if !version.GE(semver.MustParse("1.3.0")) {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("etcd audit is not supported, machine %q, cluster %q: Talos version %s < 1.3.0", hostname, clusterName, v)
		}
	}

	return nil
}

func (auditor *etcdAuditor) checkEtcdDirectory(ctx context.Context, client *client.Client) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	checkNotExists := func(err error) error {
		if strings.Contains(err.Error(), "no such file or directory") {
			return nil
		}

		return fmt.Errorf("error listing etcd directory: %w", err)
	}

	list, err := client.LS(ctx, &machine.ListRequest{Root: "/var/lib/etcd/member"})
	if err != nil {
		return false, checkNotExists(err)
	}

	info, err := list.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}

		return false, checkNotExists(err)
	}

	if info.Metadata != nil && info.Metadata.Error != "" {
		return false, fmt.Errorf("error checking etcd directory %q", info.Metadata.Error)
	}

	return true, nil
}

func (auditor *etcdAuditor) checkEphemeralMount(ctx context.Context, client *client.Client) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	_, err := safe.StateGet[*runtime.MountStatus](ctx, client.COSI, runtime.NewMountStatus(runtime.NamespaceName, constants.EphemeralPartitionLabel).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("error checking ephemeral mount: %w", err)
	}

	return true, nil
}

func (auditor *etcdAuditor) getEtcdMember(ctx context.Context, client *client.Client) (*etcd.Member, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	member, err := safe.StateGet[*etcd.Member](ctx, client.COSI, etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil //nolint:nilnil
		}

		return nil, fmt.Errorf("error getting etcd member: %w", err)
	}

	return member, nil
}
