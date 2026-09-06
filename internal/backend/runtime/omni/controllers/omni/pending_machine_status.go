// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/client"
	talosruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// PendingMachineStatusController is a controller that writes the meta key to the pending machine.
type PendingMachineStatusController = qtransform.QController[*siderolink.PendingMachine, *siderolink.PendingMachineStatus]

// NewPendingMachineStatusController initializes PendingMachineStatusController.
func NewPendingMachineStatusController() *PendingMachineStatusController {
	handler := &pendingMachineStatusHandler{}

	return qtransform.NewQController(
		qtransform.Settings[*siderolink.PendingMachine, *siderolink.PendingMachineStatus]{
			Name: "PendingMachineStatusController",
			MapMetadataFunc: func(pendingMachine *siderolink.PendingMachine) *siderolink.PendingMachineStatus {
				return siderolink.NewPendingMachineStatus(pendingMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(pendingMachineStatus *siderolink.PendingMachineStatus) *siderolink.PendingMachine {
				return siderolink.NewPendingMachine(pendingMachineStatus.Metadata().ID(), nil)
			},
			TransformFunc: handler.reconcileRunning,
		},
		qtransform.WithExtraMappedInput[*siderolink.LinkStatus](
			qtransform.MapperFuncFromTyped(
				func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, res *siderolink.LinkStatus) ([]resource.Pointer, error) {
					return []resource.Pointer{
						siderolink.NewPendingMachine(res.TypedSpec().Value.LinkId, nil).Metadata(),
					}, nil
				},
			),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](qtransform.MapperNone()),
		qtransform.WithExtraMappedInput[*omni.TalosConfig](qtransform.MapperNone()),
		qtransform.WithExtraMappedInput[*omni.MachineStatusSnapshot](qtransform.MapperNone()),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](qtransform.MapperNone()),
		qtransform.WithExtraMappedInput[*omni.Cluster](qtransform.MapperNone()),
		qtransform.WithConcurrency(32),
	)
}

type pendingMachineStatusHandler struct {
	tokenWrites singleflight.Group
}

func (handler *pendingMachineStatusHandler) reconcileRunning(
	ctx context.Context,
	r controller.Reader,
	logger *zap.Logger,
	pendingMachine *siderolink.PendingMachine,
	pendingMachineStatus *siderolink.PendingMachineStatus,
) error {
	_, err := safe.ReaderGet[*siderolink.LinkStatus](ctx, r, siderolink.NewLinkStatus(pendingMachine).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	c, err := handler.getClient(ctx, r, pendingMachine)
	if err != nil {
		return err
	}

	defer c.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	if err = handler.detectTalosInstallation(ctx, c, pendingMachineStatus); err != nil {
		return err
	}

	if err = handler.handleUUIDConflicts(ctx, c, logger, pendingMachine, pendingMachineStatus); err != nil {
		return err
	}

	return handler.generateUniqueNodeToken(ctx, c, logger, pendingMachine.TypedSpec().Value.NodeSubnet, pendingMachineStatus)
}

func (handler *pendingMachineStatusHandler) detectTalosInstallation(
	ctx context.Context,
	c *client.Client,
	pendingMachineStatus *siderolink.PendingMachineStatus,
) error {
	disks, err := c.Disks(ctx)
	if err != nil {
		return err
	}

	for _, m := range disks.Messages {
		for _, disk := range m.Disks {
			if disk.SystemDisk {
				pendingMachineStatus.TypedSpec().Value.TalosInstalled = true

				return nil
			}
		}
	}

	return nil
}

func (handler *pendingMachineStatusHandler) generateUniqueNodeToken(
	ctx context.Context,
	c *client.Client,
	logger *zap.Logger,
	nodeSubnet string,
	pendingMachineStatus *siderolink.PendingMachineStatus,
) error {
	// skip any other attempts to write the token if it was already written: every write makes the machine re-join
	if pendingMachineStatus.TypedSpec().Value.Token != "" {
		return nil
	}

	// The legacy token migration and a reboot with a new WireGuard key can create two pending machines for the same physical machine.
	// They reconcile concurrently, so the token operations are shared per machine address to end up with a single token.
	result, err, _ := handler.tokenWrites.Do(nodeSubnet, func() (any, error) {
		return handler.storeUniqueNodeToken(ctx, c, logger)
	})
	if err != nil {
		return err
	}

	pendingMachineStatus.TypedSpec().Value.Token = result.(string) //nolint:forcetypeassert,errcheck // storeUniqueNodeToken returns a string.

	return nil
}

func (handler *pendingMachineStatusHandler) storeUniqueNodeToken(ctx context.Context, c *client.Client, logger *zap.Logger) (string, error) {
	// The pending machine status is ephemeral, so it does not always know that a token was already written to the machine:
	// the previous write might have failed after Talos applied it, or another pending machine might have written it.
	// Reuse the token from the machine in that case. Generating a new one would make the machine present
	// a token which differs from the one Omni has already accepted, and the machine would be rejected from then on.
	token, err := readMetaKey(ctx, c, meta.UniqueMachineToken)
	if err != nil {
		return "", err
	}

	reused := token != ""

	if !reused {
		var fingerprint string

		fingerprint, err = jointoken.GetMachineFingerprint(ctx, c)
		if err != nil {
			return "", err
		}

		token, err = jointoken.NewNodeUniqueToken(fingerprint, uuid.NewString()).Encode()
		if err != nil {
			return "", err
		}
	}

	// Write the token even when it is reused: the previous write might have failed to flush META to the disk,
	// and the write makes the machine re-join with the token.
	if err = c.MetaWrite(ctx, meta.UniqueMachineToken, []byte(token)); err != nil {
		return "", err
	}

	logger.Info("stored node unique secret token", zap.Bool("reused", reused))

	return token, nil
}

func (handler *pendingMachineStatusHandler) handleUUIDConflicts(
	ctx context.Context,
	c *client.Client,
	logger *zap.Logger,
	pendingMachine *siderolink.PendingMachine,
	pendingMachineStatus *siderolink.PendingMachineStatus,
) error {
	machineUUID, ok := pendingMachine.Metadata().Labels().Get(omni.MachineUUID)
	if !ok {
		return fmt.Errorf("machine UUID is not set on the pending machine")
	}

	_, conflict := pendingMachine.Metadata().Annotations().Get(siderolink.PendingMachineUUIDConflict)
	if !conflict {
		pendingMachineStatus.Metadata().Annotations().Set(omni.MachineUUID, machineUUID)

		return nil
	}

	// The pending machine keeps the conflict annotation until it is cleaned up, so this handler reconciles several times for the same machine,
	// and the pending machine status is not saved when a reconcile fails. Reuse the override UUID already written to the machine:
	// generating a new one on each attempt makes the machine re-join under multiple UUIDs, which creates duplicate links for a single physical machine.
	// An override equal to the conflicting UUID is from an earlier conflict and does not resolve this one.
	// Unlike the token, an existing override is not written again: a lost flush only causes another conflict resolution after a reboot.
	id, err := readMetaKey(ctx, c, meta.UUIDOverride)
	if err != nil {
		return err
	}

	if id == "" || id == machineUUID {
		id = uuid.NewString()

		if err = c.MetaWrite(ctx, meta.UUIDOverride, []byte(id)); err != nil {
			return err
		}

		logger.Info("generated a random ID for the node", zap.String("machine", machineUUID), zap.String("new_uuid", id))
	}

	pendingMachineStatus.Metadata().Annotations().Set(omni.MachineUUID, id)

	return nil
}

// readMetaKey reads a META key from the machine, returning an empty value when the key is not set.
func readMetaKey(ctx context.Context, c *client.Client, key uint8) (string, error) {
	metaKey, err := safe.ReaderGetByID[*talosruntime.MetaKey](ctx, c.COSI, talosruntime.MetaKeyTagToID(key))
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil
		}

		return "", err
	}

	return metaKey.TypedSpec().Value, nil
}

func (handler *pendingMachineStatusHandler) getClient(
	ctx context.Context,
	r controller.Reader,
	pendingMachine *siderolink.PendingMachine,
) (*client.Client, error) {
	if strings.HasPrefix(pendingMachine.TypedSpec().Value.NodeSubnet, "unix://") {
		return helpers.GetTalosClient[*omni.ClusterMachine](ctx, r, pendingMachine.TypedSpec().Value.NodeSubnet, nil)
	}

	ipPrefix, err := netip.ParsePrefix(pendingMachine.TypedSpec().Value.NodeSubnet)
	if err != nil {
		return nil, err
	}

	address := ipPrefix.Addr().String()

	machineUUID, ok := pendingMachine.Metadata().Labels().Get(omni.MachineUUID)
	if !ok {
		return nil, fmt.Errorf("machine UUID is not set on the pending machine")
	}

	// A machine in UUID conflict is by definition not the machine that owns this UUID: the cluster
	// credentials registered under it belong to a different, healthy node and must never be used to
	// talk to this one. Treat it as an unallocated machine instead.
	if _, conflict := pendingMachine.Metadata().Annotations().Get(siderolink.PendingMachineUUIDConflict); conflict {
		return helpers.GetTalosClient[*omni.ClusterMachine](ctx, r, address, nil)
	}

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machineUUID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if clusterMachine != nil {
		return helpers.GetTalosClient(ctx, r, address, clusterMachine)
	}

	return handler.handleClusterImport(ctx, r, address, machineUUID)
}

// This method handles the case when the pending machine belongs to a cluster import process, therefore, needs a secure talos client.
func (handler *pendingMachineStatusHandler) handleClusterImport(ctx context.Context, r controller.Reader, address string, machineUUID string) (*client.Client, error) {
	machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, r, machineUUID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return helpers.GetTalosClient[*omni.ClusterMachine](ctx, r, address, nil)
		}

		return nil, err
	}

	clusterName, ok := machineSetNode.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return helpers.GetTalosClient[*omni.ClusterMachine](ctx, r, address, nil)
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return helpers.GetTalosClient[*omni.ClusterMachine](ctx, r, address, nil)
		}

		return nil, err
	}

	_, importing := cluster.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)
	if !importing {
		return helpers.GetTalosClient[*omni.ClusterMachine](ctx, r, address, nil)
	}

	importedClusterMachine := omni.NewClusterMachine(machineUUID)
	helpers.CopyLabels(machineSetNode, importedClusterMachine, omni.LabelCluster, omni.LabelMachineSet, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

	return helpers.GetTalosClient(ctx, r, address, importedClusterMachine)
}
