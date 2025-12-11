// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets_test

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
	gensecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
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
			require.NoError(t, testContext.Runtime.RegisterQController(secrets.NewSecretsController(nil)))
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
				func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
					foundClusterSecrets = res
					clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
					assertions.NotEmpty(clusterSecretsSpec.GetData())

					var bundle gensecrets.Bundle

					err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
					assertions.NoError(err)
					assertions.NotEmpty(bundle)
					assertions.Equal(clusterSecretsSpec.Imported, false)
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
			require.NoError(t, testContext.Runtime.RegisterQController(secrets.NewSecretsController(&mockBackupStoreFactory{})))
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
				func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
					foundClusterSecrets = res
					clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
					assertions.NotEmpty(clusterSecretsSpec.GetData())

					var bundle gensecrets.Bundle

					err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
					assertions.NoError(err)
					assertions.NotEmpty(bundle)

					// assert that the AES-CBC and Secretbox encryption secrets are set to the values from the backup data
					assertions.Equal("aes-cbc-test", bundle.Secrets.AESCBCEncryptionSecret)
					assertions.Equal("secretbox-test", bundle.Secrets.SecretboxEncryptionSecret)
				})
		},
	)
}

func TestImportedSecrets(t *testing.T) {
	t.Parallel()

	clusterName := "clusterID"

	testutils.WithRuntime(
		t.Context(),
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
			require.NoError(t, testContext.Runtime.RegisterQController(secrets.NewSecretsController(nil)))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			// create ImportedClusterSecret, as it will be looked up by SecretsController to attempt importing secrets bundle
			importedClusterSecrets := omni.NewImportedClusterSecrets(resources.DefaultNamespace, clusterName)
			importedClusterSecrets.TypedSpec().Value.Data = validSecretsBundle

			require.NoError(t, testContext.State.Create(ctx, importedClusterSecrets))

			cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)
			cluster.TypedSpec().Value.TalosVersion = "1.10.5"

			require.NoError(t, st.Create(ctx, cluster))

			// create ClusterUUID, as it will be looked up by SecretsController to find the source cluster ID
			clusterUUID := omni.NewClusterUUID(cluster.Metadata().ID())
			clusterUUID.TypedSpec().Value.Uuid = "test-uuid"

			clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, "test-uuid")

			require.NoError(t, st.Create(ctx, clusterUUID))

			machineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(cluster.Metadata().ID()))
			machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
			machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
			require.NoError(t, st.Create(ctx, machineSet))

			var foundClusterSecrets *omni.ClusterSecrets

			rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
				func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
					foundClusterSecrets = res
					clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
					assertions.NotEmpty(clusterSecretsSpec.GetData())

					var bundle gensecrets.Bundle

					err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
					assertions.NoError(err)
					assertions.NotEmpty(bundle)
					assertions.Equal(clusterSecretsSpec.Imported, true)
				})
		},
	)
}

func TestSecretRotation(t *testing.T) {
	t.Parallel()

	addControllers := func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(secrets.NewSecretsController(nil)))
	}

	t.Run("trigger rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)

		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State

				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 1, 1)

				var data, rotateData []byte

				rmock.Mock[*omni.ClusterSecretsRotationStatus](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecretsRotationStatus) error {
					res.TypedSpec().Value.Component = specs.ClusterSecretsRotationStatusSpec_NONE
					res.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_OK

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						clusterSecretsSpec := res.TypedSpec().Value
						assertions.NotEmpty(clusterSecretsSpec.GetData())
						assertions.Equal(clusterSecretsSpec.Imported, false)
						assertions.Empty(clusterSecretsSpec.GetRotateData())

						version, versionOK := res.Metadata().Annotations().Get(omni.RotateTalosCAVersion)
						assertions.Empty(version)
						assertions.False(versionOK)
					})

				// Create a new RotateTalosCA resource to trigger secret rotation
				rotateTalosCA := omni.NewRotateTalosCA(cluster.Metadata().ID())
				require.NoError(t, st.Create(ctx, rotateTalosCA))

				rmock.Mock[*omni.ClusterSecretsRotationStatus](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecretsRotationStatus) error {
					res.TypedSpec().Value.Component = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
					res.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						clusterSecretsSpec := res.TypedSpec().Value
						assertions.NotEmpty(clusterSecretsSpec.GetData())
						assertions.NotEmpty(clusterSecretsSpec.GetRotateData())
						assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, clusterSecretsSpec.ComponentInRotation)
						assertions.Equal(specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE, clusterSecretsSpec.RotationPhase)
						data = clusterSecretsSpec.GetData()
						rotateData = clusterSecretsSpec.GetRotateData()
					},
				)

				rmock.Mock[*omni.ClusterSecretsRotationStatus](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecretsRotationStatus) error {
					res.TypedSpec().Value.Component = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
					res.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_ROTATE

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assert *assert.Assertions) {
						assert.Equal(specs.ClusterSecretsRotationStatusSpec_ROTATE, res.TypedSpec().Value.RotationPhase)
					},
				)

				rmock.Mock[*omni.ClusterSecretsRotationStatus](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecretsRotationStatus) error {
					res.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_POST_ROTATE

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						clusterSecretsSpec := res.TypedSpec().Value
						assertions.Equal(data, clusterSecretsSpec.GetRotateData())
						assertions.Equal(rotateData, clusterSecretsSpec.GetData())
						assertions.Equal(specs.ClusterSecretsRotationStatusSpec_POST_ROTATE, clusterSecretsSpec.RotationPhase)
					},
				)

				rmock.Mock[*omni.ClusterSecretsRotationStatus](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecretsRotationStatus) error {
					res.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_OK
					res.TypedSpec().Value.Component = specs.ClusterSecretsRotationStatusSpec_NONE

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						clusterSecretsSpec := res.TypedSpec().Value
						assertions.Empty(clusterSecretsSpec.GetRotateData())
						assertions.Equal(specs.ClusterSecretsRotationStatusSpec_OK, clusterSecretsSpec.RotationPhase)
						assertions.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, clusterSecretsSpec.ComponentInRotation)

						timestamp, timestampOK := res.Metadata().Annotations().Get(omni.RotateTalosCATimestamp)
						assertions.True(timestampOK)
						assertions.NotEmpty(timestamp)

						version, versionOK := res.Metadata().Annotations().Get(omni.RotateTalosCAVersion)
						assertions.True(versionOK)
						assertions.NotEmpty(version)
					},
				)
			},
		)
	})
}
