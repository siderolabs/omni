// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"iter"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

type mockBackupStore struct{}

func (m *mockBackupStore) ListBackups(context.Context, string) (iter.Seq2[etcdbackup.Info, error], error) {
	return xiter.Empty2, nil
}

func (m *mockBackupStore) Upload(context.Context, etcdbackup.Description, io.Reader) error {
	return nil
}

func (m *mockBackupStore) Download(context.Context, []byte, string, string) (etcdbackup.BackupData, io.ReadCloser, error) {
	return etcdbackup.BackupData{
		AESCBCEncryptionSecret:    "aes-cbc-test",
		SecretboxEncryptionSecret: "secretbox-test",
	}, io.NopCloser(bytes.NewReader(nil)), nil
}

type mockBackupStoreFactory struct{}

func (m *mockBackupStoreFactory) SetThroughputs(uint64, uint64) {}

func (m *mockBackupStoreFactory) GetStore() (etcdbackup.Store, error) { return &mockBackupStore{}, nil }

func (m *mockBackupStoreFactory) Start(context.Context, state.State, *zap.Logger) error { return nil }

func (m *mockBackupStoreFactory) Description() string { return "" }

//go:embed testdata/secrets-valid.yaml
var validSecretsBundle string

func TestNewSecrets(t *testing.T) {
	t.Parallel()

	testutils.WithRuntime(
		t.Context(),
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			cluster := omni.NewCluster(resources.DefaultNamespace, "clusterID")
			cluster.TypedSpec().Value.TalosVersion = "1.2.3"
			require.NoError(t, st.Create(ctx, cluster))

			machineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(cluster.Metadata().ID()))
			require.NoError(t, st.Create(ctx, machineSet))

			var foundClusterSecrets *omni.ClusterSecrets

			// The only test I could think at this time, was simply to test bundle existence.
			rtestutils.AssertResource(
				ctx,
				t,
				st,
				cluster.Metadata().ID(),
				func(res *omni.ClusterSecrets, assert *assert.Assertions) {
					foundClusterSecrets = res
					clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
					assert.NotEmpty(clusterSecretsSpec.GetData())

					var bundle secrets.Bundle

					err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
					assert.NoError(err)
					assert.NotEmpty(bundle)
					assert.Equal(clusterSecretsSpec.Imported, false)
				})

			// Check that we can get cluster secrets by metadata.
			rtestutils.AssertResource(
				ctx,
				t,
				st,
				foundClusterSecrets.Metadata().ID(),
				func(res *omni.ClusterSecrets, _ *assert.Assertions) {
				})

			// Check that cluster secrets will be removed when the cluster is removed.
			rtestutils.Destroy[*omni.Cluster](ctx, t, st, []resource.ID{cluster.Metadata().ID()})
			rtestutils.AssertNoResource[*omni.ClusterSecrets](ctx, t, st, foundClusterSecrets.Metadata().ID())
		},
	)
}

func TestSecretsFromBackup(t *testing.T) {
	t.Parallel()

	testutils.WithRuntime(
		t.Context(),
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewSecretsController(&mockBackupStoreFactory{})))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			cluster := omni.NewCluster(resources.DefaultNamespace, "clusterID")
			cluster.TypedSpec().Value.TalosVersion = "1.2.3"

			require.NoError(t, st.Create(ctx, cluster))

			// create ClusterUUID, as it will be looked up by SecretsController to find the source cluster ID
			clusterUUID := omni.NewClusterUUID(cluster.Metadata().ID())
			clusterUUID.TypedSpec().Value.Uuid = "test-uuid"

			clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, "test-uuid")

			require.NoError(t, st.Create(ctx, clusterUUID))

			// create BackupData, as it will be looked up by SecretsController to get the encryption key
			backupData := omni.NewBackupData(cluster.Metadata().ID())

			require.NoError(t, st.Create(ctx, backupData))

			machineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(cluster.Metadata().ID()))
			machineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
				ClusterUuid: "test-uuid",
				Snapshot:    "test-snapshot",
			}

			machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
			machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

			require.NoError(t, st.Create(ctx, machineSet))

			var foundClusterSecrets *omni.ClusterSecrets

			// The only test I could think at this time, was simply to test bundle existence.
			rtestutils.AssertResource(ctx, t, st,
				cluster.Metadata().ID(),
				func(res *omni.ClusterSecrets, assert *assert.Assertions) {
					foundClusterSecrets = res
					clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
					assert.NotEmpty(clusterSecretsSpec.GetData())

					var bundle secrets.Bundle

					err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
					assert.NoError(err)
					assert.NotEmpty(bundle)

					// assert that the AES-CBC and Secretbox encryption secrets are set to the values from the backup data
					assert.Equal("aes-cbc-test", bundle.Secrets.AESCBCEncryptionSecret)
					assert.Equal("secretbox-test", bundle.Secrets.SecretboxEncryptionSecret)
				})
		},
	)
}

func TestImportedSecrets(t *testing.T) {
	t.Parallel()

	testutils.WithRuntime(
		t.Context(),
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			cluster := omni.NewCluster(resources.DefaultNamespace, "clusterID")
			cluster.TypedSpec().Value.TalosVersion = "1.10.5"

			require.NoError(t, st.Create(ctx, cluster))

			// create ClusterUUID, as it will be looked up by SecretsController to find the source cluster ID
			clusterUUID := omni.NewClusterUUID(cluster.Metadata().ID())
			clusterUUID.TypedSpec().Value.Uuid = "test-uuid"

			clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, "test-uuid")

			require.NoError(t, st.Create(ctx, clusterUUID))

			// create ImportedClusterSecret, as it will be looked up by SecretsController to attempt importing secrets bundle
			importedClusterSecrets := omni.NewImportedClusterSecrets(resources.DefaultNamespace, cluster.Metadata().ID())
			importedClusterSecrets.TypedSpec().Value.Data = validSecretsBundle

			require.NoError(t, st.Create(ctx, importedClusterSecrets))

			machineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(cluster.Metadata().ID()))
			require.NoError(t, st.Create(ctx, machineSet))

			var foundClusterSecrets *omni.ClusterSecrets

			rtestutils.AssertResource(ctx, t, st,
				cluster.Metadata().ID(),
				func(res *omni.ClusterSecrets, assert *assert.Assertions) {
					foundClusterSecrets = res
					clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
					assert.NotEmpty(clusterSecretsSpec.GetData())

					var bundle secrets.Bundle

					err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
					assert.NoError(err)
					assert.NotEmpty(bundle)
					assert.Equal(clusterSecretsSpec.Imported, true)
				})
		},
	)
}

func TestTriggerSecretRotation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)

	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx,
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			cluster := omni.NewCluster(resources.DefaultNamespace, "clusterID")
			cluster.TypedSpec().Value.TalosVersion = "1.11.4"
			require.NoError(t, st.Create(ctx, cluster))

			machineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(cluster.Metadata().ID()))
			require.NoError(t, st.Create(ctx, machineSet))

			secretRotationStatus := omni.NewClusterSecretsRotationStatus(cluster.Metadata().ID())
			require.NoError(t, st.Create(ctx, secretRotationStatus))

			rtestutils.AssertResource(
				ctx,
				t,
				st,
				cluster.Metadata().ID(),
				func(res *omni.ClusterSecrets, assert *assert.Assertions) {
					clusterSecretsSpec := res.TypedSpec().Value
					assert.NotEmpty(clusterSecretsSpec.GetData())
					assert.Equal(clusterSecretsSpec.Imported, false)
					assert.Empty(clusterSecretsSpec.GetRotateData())
					assert.Empty(clusterSecretsSpec.RotateTalosCaVersion)
				})

			// Create a new RotateTalosCA resource to trigger secret rotation
			rotateTalosCA := omni.NewRotateTalosCA(cluster.Metadata().ID())
			require.NoError(t, st.Create(ctx, rotateTalosCA))

			rtestutils.AssertResource(
				ctx,
				t,
				st,
				cluster.Metadata().ID(),
				func(res *omni.ClusterSecrets, assert *assert.Assertions) {
					clusterSecretsSpec := res.TypedSpec().Value
					assert.NotEmpty(clusterSecretsSpec.GetData())
					assert.NotEmpty(clusterSecretsSpec.GetRotateData())
				},
			)
		},
	)
}
