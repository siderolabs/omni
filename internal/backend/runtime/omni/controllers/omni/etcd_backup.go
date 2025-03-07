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
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/containers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
)

// EtcdBackupController manages etcd backups.
type EtcdBackupController struct {
	settings EtcdBackupControllerSettings

	manualBackups map[resource.ID]time.Time
	lastBackup    containers.ConcurrentMap[string, time.Time]
	parallel      int
	maxBackupTime time.Duration
}

// EtcdBackupControllerSettings is a set of parameters for EtcdBackupController.
type EtcdBackupControllerSettings struct {
	ClientMaker  ClientMaker
	StoreFactory store.Factory
	TickInterval time.Duration // can be 0 in which case StoreFactory should be [store.DisabledStoreFactory]
}

// ClientMaker is a function that creates Talos client.
type ClientMaker func(ctx context.Context, clusterName string) (TalosClient, error)

// NewEtcdBackupController creates new EtcdBackupController with defined uploader.
func NewEtcdBackupController(s EtcdBackupControllerSettings) (*EtcdBackupController, error) {
	if s.TickInterval < time.Minute && s.StoreFactory != store.DisabledStoreFactory {
		return nil, errors.New("tick interval must be at least 1 minute")
	}

	return &EtcdBackupController{
		settings:      s,
		manualBackups: map[resource.ID]time.Time{},
		parallel:      5,
		maxBackupTime: 30 * time.Minute,
	}, nil
}

// Name implements controller.Controller interface.
func (*EtcdBackupController) Name() string {
	return "EtcdBackupController"
}

// Inputs implements controller.Controller interface.
func (*EtcdBackupController) Inputs() []controller.Input {
	return []controller.Input{
		safe.Input[*omni.BackupData](controller.InputWeak),
		safe.Input[*omni.EtcdManualBackup](controller.InputWeak),
	}
}

// Outputs implements controller.Controller interface.
func (*EtcdBackupController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.EtcdBackupStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *EtcdBackupController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.settings.StoreFactory == store.DisabledStoreFactory {
		logger.Info("etcd backups are disabled")

		return nil
	}

	logger.Info(
		"etcd backups are enabled",
		zap.String("uploader", ctrl.settings.StoreFactory.Description()),
		zap.Duration("tick_interval", ctrl.settings.TickInterval),
	)

	for {
		_, state := channel.TryRecv(r.EventCh())
		if state != channel.StateRecv {
			// If we didn't receive any event, wait for tick interval.
			// We don't drop here if we are getting stream of events, so we don't create unnecessary timers.
			timer := time.NewTimer(ctrl.settings.TickInterval)

			select {
			case <-ctx.Done():
				return nil
			case <-r.EventCh():
			case <-timer.C:
			}

			timer.Stop()
		}

		err := ctrl.run(ctx, r, logger)
		if err != nil {
			if errors.Is(err, store.ErrS3NotInitialized) {
				continue
			}

			return fmt.Errorf("failed to run etcd backup controller: %w", err)
		}
	}
}

func (ctrl *EtcdBackupController) run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	bdl, err := safe.ReaderListAll[*omni.BackupData](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	err = ctrl.cleanup(ctx, r, bdl, logger)
	if err != nil {
		return err
	}

	backupDataList, err := ctrl.findClustersToBackup(ctx, r, bdl, logger)
	if err != nil {
		return fmt.Errorf("error while finding cluster to backup: %w", err)
	}

	if len(backupDataList) == 0 {
		logger.Debug("no cluster to backup")

		return nil
	}

	eg := panichandler.NewErrGroup()

	eg.SetLimit(ctrl.parallel)

	type result struct {
		time      time.Time
		err       error
		clusterID string
	}

	backupResultCh := make(chan result, len(backupDataList))

	for _, backupData := range backupDataList {
		clusterLogger := logger.With(zap.String("cluster", backupData.Metadata().ID()))

		eg.Go(func() error {
			localCtx, cancel := context.WithTimeout(ctx, ctrl.maxBackupTime)
			defer cancel()

			backupErr := ctrl.doBackup(localCtx, backupData, clusterLogger)

			backupResultCh <- result{
				clusterID: backupData.Metadata().ID(),
				time:      time.Now(),
				err:       backupErr,
			}

			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("failed to backup cluster: %w", err)
	}

	close(backupResultCh)

	for backupResultData := range backupResultCh {
		ctrl.updateBackupStatus(ctx, r, backupResultData.clusterID, backupResultData.time, backupResultData.err, logger)
	}

	return nil
}

func (ctrl *EtcdBackupController) cleanup(ctx context.Context, r controller.Runtime, bdl safe.List[*omni.BackupData], logger *zap.Logger) error {
	ctrl.lastBackup.FilterInPlace(func(uuid string, _ time.Time) bool {
		idx := bdl.Index(func(backupData *omni.BackupData) bool {
			return backupData.TypedSpec().Value.ClusterUuid == uuid
		})
		if idx >= 0 {
			return true
		}

		logger.Info(
			"cluster backup no longer exists, removing from last backup list",
			zap.String("uuid", uuid),
		)

		return false
	})

	err := cleanupOutputs(ctx, r, func(es *omni.EtcdBackupStatus) bool {
		return bdl.Index(func(bd *omni.BackupData) bool {
			return bd.Metadata().ID() == es.Metadata().ID()
		}) >= 0
	})
	if err != nil {
		return fmt.Errorf("failed to cleanup EtcdBackupStatus outputs: %w", err)
	}

	return nil
}

func (ctrl *EtcdBackupController) findClustersToBackup(ctx context.Context, r controller.Runtime, bdl safe.List[*omni.BackupData], logger *zap.Logger) ([]*omni.BackupData, error) {
	if bdl.Len() == 0 {
		return nil, nil
	}

	result := make([]*omni.BackupData, 0, bdl.Len())

	for backupData := range bdl.All() {
		clusterID := backupData.Metadata().ID()
		value := backupData.TypedSpec().Value

		emb, err := safe.ReaderGetByID[*omni.EtcdManualBackup](ctx, r, clusterID)
		if err != nil && !state.IsNotFoundError(err) {
			return nil, fmt.Errorf("failed to get manual backup for cluster %q: %w", clusterID, err)
		}

		if ctrl.shouldManualBackup(emb) {
			asTime := emb.TypedSpec().Value.GetBackupAt().AsTime()

			logger.Info("doing manual backup", zap.Time("next_backup", asTime), zap.String("cluster", clusterID))

			result = append(result, backupData)

			ctrl.manualBackups[clusterID] = asTime

			continue
		}

		if backupData.TypedSpec().Value.GetInterval().AsDuration() == 0 {
			continue
		}

		latestBackupTime, err := ctrl.latestBackupTime(ctx, backupData.TypedSpec().Value.ClusterUuid)
		if err != nil {
			ctrl.updateBackupStatus(ctx, r, clusterID, time.Now(), err, logger)

			return nil, fmt.Errorf("failed to get latest backup time for cluster %q: %w", clusterID, err)
		}

		if latestBackupTime.IsZero() {
			logger.Info("cluster backup is missing, creating new one", zap.String("cluster", clusterID))

			result = append(result, backupData)

			continue
		}

		if elapsed := time.Since(latestBackupTime); elapsed >= value.Interval.AsDuration() {
			result = append(result, backupData)
		}
	}

	return result, nil
}

func (ctrl *EtcdBackupController) shouldManualBackup(emb *omni.EtcdManualBackup) bool {
	if emb == nil {
		return false
	}

	clusterID := emb.Metadata().ID()
	value := emb.TypedSpec().Value

	if value.GetBackupAt().GetSeconds() == 0 {
		return false
	}

	manualBackupAt := value.GetBackupAt().AsTime()

	if lastManualBackup, ok := ctrl.manualBackups[clusterID]; ok && lastManualBackup.Equal(manualBackupAt) {
		return false
	}

	timeDiff := time.Since(manualBackupAt)
	if timeDiff >= 10*time.Minute || timeDiff <= -10*time.Minute {
		return false
	}

	return true
}

func (ctrl *EtcdBackupController) latestBackupTime(ctx context.Context, clusterUUID string) (time.Time, error) {
	if latestBackupTime, ok := ctrl.lastBackup.Get(clusterUUID); ok {
		return latestBackupTime, nil
	}

	st, err := ctrl.settings.StoreFactory.GetStore()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get store: %w", err)
	}

	it, err := st.ListBackups(ctx, clusterUUID)
	if err != nil {
		return time.Time{}, err
	}

	var lastBackup etcdbackup.Info

	for v, iterErr := range it {
		if iterErr != nil {
			return time.Time{}, fmt.Errorf("failed to get last backup: %w", err)
		}

		lastBackup = v
	}

	if lastBackup.Timestamp.IsZero() {
		return time.Time{}, nil
	}

	ctrl.lastBackup.Set(clusterUUID, lastBackup.Timestamp)

	return lastBackup.Timestamp, nil
}

func (ctrl *EtcdBackupController) doBackup(
	ctx context.Context,
	backupData *omni.BackupData,
	logger *zap.Logger,
) error {
	clusterName := backupData.Metadata().ID()

	client, err := ctrl.settings.ClientMaker(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to create talos client for cluster, skipping cluster backup: %w", err)
	}

	rdr, err := client.EtcdSnapshot(ctx, &machineapi.EtcdSnapshotRequest{})
	if err != nil {
		return fmt.Errorf("failed to start etcd snapshot stream for cluster: %w", err)
	}

	defer func() {
		if rdrErr := rdr.Close(); rdrErr != nil {
			logger.Warn("failed to close etcd snapshot reader", zap.Error(rdrErr))
		}
	}()

	now := time.Now()

	st, err := ctrl.settings.StoreFactory.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get store: %w", err)
	}

	if err := st.Upload(
		ctx,
		etcdbackup.Description{
			Timestamp:   now,
			ClusterUUID: backupData.TypedSpec().Value.ClusterUuid,
			ClusterName: clusterName,
			EncryptionData: etcdbackup.EncryptionData{
				AESCBCEncryptionSecret:    backupData.TypedSpec().Value.AesCbcEncryptionSecret,
				SecretboxEncryptionSecret: backupData.TypedSpec().Value.SecretboxEncryptionSecret,
				EncryptionKey:             backupData.TypedSpec().Value.EncryptionKey,
			},
		},
		rdr,
	); err != nil {
		return fmt.Errorf("failed to upload etcd snapshot for cluster: %w", err)
	}

	ctrl.lastBackup.Set(backupData.TypedSpec().Value.ClusterUuid, now)

	logger.Info(
		"uploaded etcd snapshot",
		zap.String("cluster_uuid", backupData.TypedSpec().Value.ClusterUuid),
		zap.String("snapshot_name", etcdbackup.CreateSnapshotName(now)),
		zap.Time("ts", now),
	)

	return nil
}

func (ctrl *EtcdBackupController) updateBackupStatus(
	ctx context.Context,
	r controller.Runtime,
	clusterID string,
	backupTime time.Time,
	backupErr error,
	logger *zap.Logger,
) {
	if backupErr != nil {
		logger.Warn(
			"failed to backup cluster",
			zap.String("cluster", clusterID),
			zap.Error(backupErr),
		)
	}

	err := safe.WriterModify(
		ctx,
		r,
		omni.NewEtcdBackupStatus(clusterID),
		func(status *omni.EtcdBackupStatus) error {
			status.TypedSpec().Value.LastBackupAttempt = timestamppb.New(backupTime)

			if backupErr != nil {
				status.TypedSpec().Value.Error = backupErr.Error()
				status.TypedSpec().Value.Status = specs.EtcdBackupStatusSpec_Error
			} else {
				status.TypedSpec().Value.Error = ""
				status.TypedSpec().Value.Status = specs.EtcdBackupStatusSpec_Ok
				status.TypedSpec().Value.LastBackupTime = timestamppb.New(backupTime)
			}

			return nil
		},
	)
	if err != nil {
		logger.Warn("failed to update etcd backup status", zap.Error(backupErr))
	}
}

// TalosClient is a subset of Talos client.
type TalosClient interface {
	EtcdSnapshot(ctx context.Context, req *machineapi.EtcdSnapshotRequest, callOptions ...grpc.CallOption) (io.ReadCloser, error)
}
