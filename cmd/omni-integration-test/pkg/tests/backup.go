// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// AssertEtcdManualBackupIsCreated creates a manual etcd backup and asserts the status.
func AssertEtcdManualBackupIsCreated(testCtx context.Context, st state.State, clusterName string) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
		defer cancel()

		// drop the previous EtcdManualBackup resource if it exists
		rtestutils.Destroy[*omni.EtcdManualBackup](ctx, t, st, []string{clusterName})

		// Take existing backups into account when asserting the length after creating a new backup.
		existingBackups, err := safe.StateListAll[*omni.EtcdBackup](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NoError(t, err)

		backup := omni.NewEtcdManualBackup(clusterName)
		backup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now())

		start := time.Now()

		require.NoError(t, st.Create(ctx, backup))

		assertLength[*omni.EtcdBackup](
			ctx,
			t,
			st,
			existingBackups.Len()+1,
			state.WatchWithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		var backupTime time.Time

		rtestutils.AssertResources(
			ctx,
			t,
			st,
			[]resource.ID{clusterName},
			func(es *omni.EtcdBackupStatus, assertion *assert.Assertions) {
				l := es.TypedSpec().Value.GetLastBackupTime().AsTime()

				assertion.Equal(clusterName, es.Metadata().ID())
				assertion.Equal(specs.EtcdBackupStatusSpec_Ok, es.TypedSpec().Value.Status)
				assertion.Zero(es.TypedSpec().Value.Error)
				assertion.Truef(l.After(start), "last backup time %q is not after start time %q", l, start)
				assertion.WithinDuration(time.Now(), l, 300*time.Second)

				backupTime = l
			},
		)

		rtestutils.AssertResources(
			ctx,
			t,
			st,
			[]resource.ID{omni.EtcdBackupOverallStatusID},
			func(res *omni.EtcdBackupOverallStatus, assertion *assert.Assertions) {
				assertion.Zero(res.TypedSpec().Value.GetConfigurationError())
				assertion.Zero(res.TypedSpec().Value.GetLastBackupStatus().GetError())
				assertion.EqualValues(1, res.TypedSpec().Value.GetLastBackupStatus().GetStatus())
				assertion.Equal(backupTime, res.TypedSpec().Value.GetLastBackupStatus().GetLastBackupAttempt().AsTime())
				assertion.Equal(backupTime, res.TypedSpec().Value.GetLastBackupStatus().GetLastBackupTime().AsTime())
			},
		)
	}
}

// AssertEtcdAutomaticBackupIsCreated asserts that automatic etcd backups are created.
func AssertEtcdAutomaticBackupIsCreated(testCtx context.Context, st state.State, clusterName string) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
		defer cancel()

		// drop the previous EtcdBackupS3Conf resource if it exists
		rtestutils.Destroy[*omni.EtcdBackupS3Conf](ctx, t, st, []string{omni.EtcdBackupS3ConfID})

		conf := createS3Conf()
		start := time.Now()

		require.NoError(t, st.Create(ctx, conf))

		t.Logf("waiting for backup for cluster %q to be created", clusterName)

		assertLength[*omni.EtcdBackup](
			ctx,
			t,
			st,
			1,
			state.WatchWithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		var backupTime time.Time

		rtestutils.AssertResources(
			testCtx,
			t,
			st,
			[]resource.ID{clusterName},
			func(es *omni.EtcdBackupStatus, assertion *assert.Assertions) {
				l := es.TypedSpec().Value.GetLastBackupTime().AsTime()

				assertion.Equal(specs.EtcdBackupStatusSpec_Ok, es.TypedSpec().Value.Status)
				assertion.Zero(es.TypedSpec().Value.Error)
				assertion.Truef(l.After(start), "last backup time %q is not after start time %q", l, start)
				assertion.WithinDuration(time.Now(), l, 120*time.Second)

				backupTime = l
			},
		)

		rtestutils.AssertResources(
			ctx,
			t,
			st,
			[]resource.ID{omni.EtcdBackupOverallStatusID},
			func(res *omni.EtcdBackupOverallStatus, assertion *assert.Assertions) {
				assertion.Zero(res.TypedSpec().Value.GetConfigurationError())
				assertion.Zero(res.TypedSpec().Value.GetLastBackupStatus().GetError())
				assertion.EqualValues(1, res.TypedSpec().Value.GetLastBackupStatus().GetStatus())
				assertion.Equal(backupTime, res.TypedSpec().Value.GetLastBackupStatus().GetLastBackupAttempt().AsTime())
				assertion.Equal(backupTime, res.TypedSpec().Value.GetLastBackupStatus().GetLastBackupTime().AsTime())
			},
		)
	}
}

// AssertControlPlaneCanBeRestoredFromBackup asserts that the control plane
// is recreated with the bootstrap spec pointing to the most recent etcd backup.
// It does not add any machines to the newly created control plane.
func AssertControlPlaneCanBeRestoredFromBackup(testCtx context.Context, st state.State, clusterName string) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
		defer cancel()

		clusterUUID, err := safe.StateGetByID[*omni.ClusterUUID](ctx, st, clusterName)
		require.NoError(t, err)

		backupList, err := safe.StateListAll[*omni.EtcdBackup](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NoError(t, err)

		require.NotEmpty(t, backupList.Len())

		snapshotName := backupList.Get(0).TypedSpec().Value.GetSnapshot() // the first backup is the most recent one
		cpMachineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(clusterName))

		cpMachineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
		cpMachineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

		t.Logf("snapshot name: %q", snapshotName)

		cpMachineSet.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling
		cpMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
			ClusterUuid: clusterUUID.TypedSpec().Value.GetUuid(),
			Snapshot:    snapshotName,
		}

		require.NoError(t, st.Create(ctx, cpMachineSet))
	}
}

// assertLength asserts on the length of a resource list. It's a specialized version of [rtestutils.AssertLength] which
// restarts WatchKind on each ticker tick. It here because [rtestutils.AssertLength] does not support [state.WatchKindOption]
// and because WatchKind on external resource does not send new events.
func assertLength[R meta.ResourceWithRD](ctx context.Context, t *testing.T, st state.State, expectedLength int, watchOpts ...state.WatchKindOption) {
	var r R

	rds := r.ResourceDefinition()

	watchOpts = append(
		watchOpts[:len(watchOpts):len(watchOpts)],
		state.WithBootstrapContents(true),
	)

	reportTicker := time.NewTicker(5 * time.Second)
	defer reportTicker.Stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		req := require.New(t)

		innerCtx, cancel := context.WithCancel(ctx)

		watchCh := make(chan state.Event)

		req.NoError(st.WatchKind(
			innerCtx,
			resource.NewMetadata(rds.DefaultNamespace, rds.Type, "", resource.VersionUndefined),
			watchCh,
			watchOpts...))

		length := 0
		bootstrapped := false

		for {
			shouldBreak := false

			select {
			case event := <-watchCh:
				switch event.Type { //nolint:exhaustive
				case state.Created:
					length++
				case state.Destroyed:
					length--
				case state.Bootstrapped:
					bootstrapped = true
				case state.Errored:
					req.NoError(event.Error)
				}

				if bootstrapped && length == expectedLength {
					cancel()

					return
				}
			case <-reportTicker.C:
				t.Logf("length: expected %d, actual %d", expectedLength, length)

				shouldBreak = true
			case <-innerCtx.Done():
				t.Fatalf("timeout: expected %d, actual %d", expectedLength, length)
			}

			if shouldBreak {
				cancel()

				break
			}
		}
	}
}

func createS3Conf() *omni.EtcdBackupS3Conf {
	conf := omni.NewEtcdBackupS3Conf()

	conf.TypedSpec().Value = &specs.EtcdBackupS3ConfSpec{
		Bucket:          "mybucket",
		Region:          "us-east-1",
		Endpoint:        "http://127.0.0.1:9000",
		AccessKeyId:     "access",
		SecretAccessKey: "secret123",
	}

	return conf
}
