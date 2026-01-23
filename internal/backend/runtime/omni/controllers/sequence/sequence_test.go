// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sequence_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/sequence"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

type fakeSequence struct{}

func (f fakeSequence) MapFunc(input *omni.Cluster) *omni.ClusterStatus {
	return omni.NewClusterStatus(input.Metadata().ID())
}

func (f fakeSequence) UnmapFunc(output *omni.ClusterStatus) *omni.Cluster {
	return omni.NewCluster(output.Metadata().ID())
}

func (f fakeSequence) Options() []qtransform.ControllerOption {
	return []qtransform.ControllerOption{
		qtransform.WithExtraMappedInput[*omni.EtcdBackupStatus](
			qtransform.MapperSameID[*omni.Cluster](),
		),
	}
}

func (f fakeSequence) Stages() []sequence.Stage[*omni.Cluster, *omni.ClusterStatus] {
	getEtcdBackupStatus := func(ctx context.Context, r controller.Reader, input *omni.Cluster) (*omni.EtcdBackupStatus, error) {
		backupStatus, err := safe.ReaderGetByID[*omni.EtcdBackupStatus](ctx, r, input.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, xerrors.NewTagged[qtransform.SkipReconcileTag](err)
			}

			return nil, err
		}

		return backupStatus, nil
	}

	return []sequence.Stage[*omni.Cluster, *omni.ClusterStatus]{
		sequence.NewStage("idle", func(ctx context.Context, logger *zap.Logger, sequenceContext sequence.Context[*omni.Cluster, *omni.ClusterStatus]) (completed bool, err error) {
			r := sequenceContext.Runtime
			input := sequenceContext.Input
			output := sequenceContext.Output

			backupStatus, err := getEtcdBackupStatus(ctx, r, input)
			if err != nil {
				return false, err
			}

			logger.Info("waiting for etcd backup to be triggered", zap.String("status", backupStatus.TypedSpec().Value.Status.String()))

			if backupStatus.TypedSpec().Value.Status == specs.EtcdBackupStatusSpec_Ok {
				output.Metadata().Labels().Set("etcd", "ok")

				return false, nil
			}

			logger.Info("etcd backup is triggered", zap.String("status", backupStatus.TypedSpec().Value.Status.String()))

			return true, controller.NewRequeueInterval(time.Millisecond)
		}),
		sequence.NewStage("backup", func(ctx context.Context, logger *zap.Logger, sequenceContext sequence.Context[*omni.Cluster, *omni.ClusterStatus]) (completed bool, err error) {
			r := sequenceContext.Runtime
			input := sequenceContext.Input
			output := sequenceContext.Output

			backupStatus, err := getEtcdBackupStatus(ctx, r, input)
			if err != nil {
				return false, err
			}

			logger.Info("etcd backup in progress", zap.String("status", backupStatus.TypedSpec().Value.Status.String()))

			if backupStatus.TypedSpec().Value.Status != specs.EtcdBackupStatusSpec_Running {
				output.Metadata().Labels().Set("etcd", "no-backup")

				logger.Info("etcd backup failed", zap.String("status", backupStatus.TypedSpec().Value.Status.String()))

				return false, nil
			}

			logger.Info("etcd backup finalizing", zap.String("status", backupStatus.TypedSpec().Value.Status.String()))

			output.Metadata().Labels().Set("etcd", "running")

			return true, controller.NewRequeueInterval(time.Millisecond)
		}),
		sequence.NewStage("done", func(ctx context.Context, logger *zap.Logger, sequenceContext sequence.Context[*omni.Cluster, *omni.ClusterStatus]) (completed bool, err error) {
			r := sequenceContext.Runtime
			input := sequenceContext.Input
			output := sequenceContext.Output

			backupStatus, err := getEtcdBackupStatus(ctx, r, input)
			if err != nil {
				return false, err
			}

			if backupStatus.TypedSpec().Value.Status != specs.EtcdBackupStatusSpec_Ok {
				logger.Info("etcd backup has problems", zap.String("status", backupStatus.TypedSpec().Value.Status.String()))

				output.Metadata().Labels().Set("etcd", "error")

				return false, nil
			}

			logger.Info("etcd backup completed", zap.String("status", backupStatus.TypedSpec().Value.Status.String()))

			output.Metadata().Labels().Set("etcd", "done")

			return true, controller.NewRequeueInterval(time.Millisecond)
		}),
	}
}

func FakeSequenceController() *sequence.Controller[*omni.Cluster, *omni.ClusterStatus] {
	return sequence.NewController[*omni.Cluster, *omni.ClusterStatus]("FakeSequenceController", &fakeSequence{})
}

func TestSequenceController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx,
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(FakeSequenceController()))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			clusterID := "test-cluster"
			rmock.Mock[*omni.Cluster](ctx, t, st, options.WithID(clusterID))
			rmock.Mock[*omni.EtcdBackupStatus](ctx, t, st, options.WithID(clusterID), options.Modify(func(res *omni.EtcdBackupStatus) error {
				res.TypedSpec().Value.Status = specs.EtcdBackupStatusSpec_Ok

				return nil
			}))

			rtestutils.AssertResource(ctx, t, st, clusterID, func(res *omni.ClusterStatus, asrt *assert.Assertions) {
				lbl, ok := res.Metadata().Labels().Get("etcd")
				asrt.True(ok)
				asrt.Equal("ok", lbl)
			})

			rmock.Mock[*omni.EtcdBackupStatus](ctx, t, st, options.WithID(clusterID), options.Modify(func(res *omni.EtcdBackupStatus) error {
				res.TypedSpec().Value.Status = specs.EtcdBackupStatusSpec_Error

				return nil
			}))

			rtestutils.AssertResource(ctx, t, st, clusterID, func(res *omni.ClusterStatus, asrt *assert.Assertions) {
				lbl, ok := res.Metadata().Labels().Get("etcd")
				asrt.True(ok)
				asrt.Equal("no-backup", lbl)
			})

			rmock.Mock[*omni.EtcdBackupStatus](ctx, t, st, options.WithID(clusterID), options.Modify(func(res *omni.EtcdBackupStatus) error {
				res.TypedSpec().Value.Status = specs.EtcdBackupStatusSpec_Running

				return nil
			}))

			rtestutils.AssertResource(ctx, t, st, clusterID, func(res *omni.ClusterStatus, asrt *assert.Assertions) {
				lbl, ok := res.Metadata().Labels().Get("etcd")
				asrt.True(ok)
				asrt.Equal("running", lbl)
			})

			rmock.Mock[*omni.EtcdBackupStatus](ctx, t, st, options.WithID(clusterID), options.Modify(func(res *omni.EtcdBackupStatus) error {
				res.TypedSpec().Value.Status = specs.EtcdBackupStatusSpec_Error

				return nil
			}))

			rtestutils.AssertResource(ctx, t, st, clusterID, func(res *omni.ClusterStatus, asrt *assert.Assertions) {
				lbl, ok := res.Metadata().Labels().Get("etcd")
				asrt.True(ok)
				asrt.Equal("error", lbl)
			})

			rmock.Mock[*omni.EtcdBackupStatus](ctx, t, st, options.WithID(clusterID), options.Modify(func(res *omni.EtcdBackupStatus) error {
				res.TypedSpec().Value.Status = specs.EtcdBackupStatusSpec_Ok

				return nil
			}))

			rtestutils.AssertResource(ctx, t, st, clusterID, func(res *omni.ClusterStatus, asrt *assert.Assertions) {
				lbl, ok := res.Metadata().Labels().Get("etcd")
				asrt.True(ok)
				asrt.Equal("ok", lbl)
			})
		},
	)
}
