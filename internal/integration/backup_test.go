// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// AssertEtcdManualBackupIsCreated creates a manual etcd backup and asserts the status.
func AssertEtcdManualBackupIsCreated(testCtx context.Context, st state.State, clusterName string) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
		defer cancel()

		// drop the previous EtcdManualBackup resource if it exists
		rtestutils.Destroy[*omni.EtcdManualBackup](ctx, t, st, []string{clusterName})

		start := time.Now()

		// If we have another backup lets use it's time to determine that we have a new one.
		// The reason for that is that backup names are unix timestamps with second resolution,
		// but backup status has a nanosecond resolution. So if we create a new backup, status will
		// always contain ts which is newer that the one from the state.
		//
		// We can't use number of backups here because two backups can happen at the same second, where
		// the newer will overwrite the older.
		bs, err := safe.ReaderGetByID[*omni.EtcdBackupStatus](testCtx, st, clusterName)

		if err == nil {
			start = bs.TypedSpec().Value.LastBackupTime.AsTime()
		} else if !state.IsNotFoundError(err) {
			require.NoError(t, err)
		}

		backup := omni.NewEtcdManualBackup(clusterName)
		backup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now())

		require.NoError(t, st.Create(ctx, backup))

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
		cpMachineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(clusterName))

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
