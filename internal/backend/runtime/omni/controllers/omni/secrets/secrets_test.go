// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/talos/pkg/machinery/config"
	gensecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
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
			cluster := omni.NewCluster("clusterID")
			cluster.TypedSpec().Value.TalosVersion = "1.2.3"
			require.NoError(t, st.Create(ctx, cluster))

			machineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(cluster.Metadata().ID()))
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

			cluster := omni.NewCluster("clusterID")
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

			machineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(cluster.Metadata().ID()))
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
			importedClusterSecrets := omni.NewImportedClusterSecrets(clusterName)
			importedClusterSecrets.TypedSpec().Value.Data = validSecretsBundle

			require.NoError(t, testContext.State.Create(ctx, importedClusterSecrets))

			cluster := omni.NewCluster(clusterName)
			cluster.TypedSpec().Value.TalosVersion = "1.10.5"

			require.NoError(t, st.Create(ctx, cluster))

			// create ClusterUUID, as it will be looked up by SecretsController to find the source cluster ID
			clusterUUID := omni.NewClusterUUID(cluster.Metadata().ID())
			clusterUUID.TypedSpec().Value.Uuid = "test-uuid"

			clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, "test-uuid")

			require.NoError(t, st.Create(ctx, clusterUUID))

			machineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(cluster.Metadata().ID()))
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

//nolint:maintidx
func TestSecretRotation(t *testing.T) {
	t.Parallel()

	addControllers := func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(secrets.NewSecretsController(nil)))
	}

	t.Run("trigger Talos CA rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)

		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State

				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 1, 1)

				clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, st, cluster.Metadata().ID())
				require.NoError(t, err)

				secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
				require.NoError(t, err)

				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Component = specs.SecretRotationSpec_NONE
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_OK
					res.TypedSpec().Value.Certs = &specs.ClusterSecretsSpec_Certs{
						Os: &specs.ClusterSecretsSpec_Certs_CA{
							Crt: secretsBundle.Certs.OS.Crt,
							Key: secretsBundle.Certs.OS.Key,
						},
					}
					res.TypedSpec().Value.ExtraCerts = nil

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						assertions.Nil(res.TypedSpec().Value.ExtraCerts)

						version, versionOK := res.Metadata().Annotations().Get(omni.RotateTalosCAVersion)
						assertions.Empty(version)
						assertions.False(versionOK)
					})

				talosCA, err := gensecrets.NewTalosCA(gensecrets.NewFixedClock(time.Now()).Now())
				require.NoError(t, err)

				// SecretRotation resource is updated to indicate that secret rotation is in progress
				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Component = specs.SecretRotationSpec_TALOS_CA
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_PRE_ROTATE
					res.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{
						Os: &specs.ClusterSecretsSpec_Certs_CA{
							Crt: talosCA.CrtPEM,
							Key: talosCA.KeyPEM,
						},
					}

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						assertions.NotNil(res.TypedSpec().Value.GetExtraCerts())
						assertions.NotNil(res.TypedSpec().Value.ExtraCerts.GetOs())
					},
				)

				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_ROTATE

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assert *assert.Assertions) {
						bundle, innerErr := omni.ToSecretsBundle(res.TypedSpec().Value.Data)
						assert.NoError(innerErr)

						assert.Equal(talosCA.CrtPEM, bundle.Certs.OS.Crt)
						assert.Equal(talosCA.KeyPEM, bundle.Certs.OS.Key)

						assert.Equal(secretsBundle.Certs.OS.Crt, res.TypedSpec().Value.ExtraCerts.Os.Crt)
						assert.Equal(secretsBundle.Certs.OS.Key, res.TypedSpec().Value.ExtraCerts.Os.Key)
					},
				)

				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_POST_ROTATE

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						timestamp, timestampOK := res.Metadata().Annotations().Get(omni.RotateTalosCATimestamp)
						assertions.True(timestampOK)
						assertions.NotEmpty(timestamp)
					},
				)

				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_OK
					res.TypedSpec().Value.Component = specs.SecretRotationSpec_TALOS_CA

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						assertions.Nil(res.TypedSpec().Value.ExtraCerts)
						timestamp, timestampOK := res.Metadata().Annotations().Get(omni.RotateTalosCATimestamp)
						assertions.True(timestampOK)
						assertions.NotEmpty(timestamp)
					},
				)
			},
		)
	})

	t.Run("trigger Kubernetes CA rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)

		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State

				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-k8s-ca", 1, 1)

				clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, st, cluster.Metadata().ID())
				require.NoError(t, err)

				secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
				require.NoError(t, err)

				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Component = specs.SecretRotationSpec_NONE
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_OK
					res.TypedSpec().Value.Certs = &specs.ClusterSecretsSpec_Certs{
						K8S: &specs.ClusterSecretsSpec_Certs_CA{
							Crt: secretsBundle.Certs.K8s.Crt,
							Key: secretsBundle.Certs.K8s.Key,
						},
					}
					res.TypedSpec().Value.ExtraCerts = nil

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						assertions.Nil(res.TypedSpec().Value.ExtraCerts)

						version, versionOK := res.Metadata().Annotations().Get(omni.RotateKubernetesCAVersion)
						assertions.Empty(version)
						assertions.False(versionOK)
					})

				versionContract, err := config.ParseContractFromVersion(cluster.TypedSpec().Value.TalosVersion)
				require.NoError(t, err)

				kubernetesCA, err := gensecrets.NewKubernetesCA(gensecrets.NewFixedClock(time.Now()).Now(), versionContract)
				require.NoError(t, err)

				// SecretRotation resource is updated to indicate that secret rotation is in progress
				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Component = specs.SecretRotationSpec_KUBERNETES_CA
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_PRE_ROTATE
					res.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{
						K8S: &specs.ClusterSecretsSpec_Certs_CA{
							Crt: kubernetesCA.CrtPEM,
							Key: kubernetesCA.KeyPEM,
						},
					}

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						assertions.NotNil(res.TypedSpec().Value.GetExtraCerts())
						assertions.NotNil(res.TypedSpec().Value.ExtraCerts.GetK8S())
					},
				)

				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_ROTATE

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assert *assert.Assertions) {
						bundle, innerErr := omni.ToSecretsBundle(res.TypedSpec().Value.Data)
						assert.NoError(innerErr)

						assert.Equal(kubernetesCA.CrtPEM, bundle.Certs.K8s.Crt)
						assert.Equal(kubernetesCA.KeyPEM, bundle.Certs.K8s.Key)

						assert.Equal(secretsBundle.Certs.K8s.Crt, res.TypedSpec().Value.ExtraCerts.K8S.Crt)
						assert.Equal(secretsBundle.Certs.K8s.Key, res.TypedSpec().Value.ExtraCerts.K8S.Key)
					},
				)

				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_POST_ROTATE

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						timestamp, timestampOK := res.Metadata().Annotations().Get(omni.RotateKubernetesCATimestamp)
						assertions.True(timestampOK)
						assertions.NotEmpty(timestamp)
					},
				)

				rmock.Mock[*omni.SecretRotation](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.SecretRotation) error {
					res.TypedSpec().Value.Phase = specs.SecretRotationSpec_OK
					res.TypedSpec().Value.Component = specs.SecretRotationSpec_KUBERNETES_CA

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, cluster.Metadata().ID(),
					func(res *omni.ClusterSecrets, assertions *assert.Assertions) {
						assertions.Nil(res.TypedSpec().Value.ExtraCerts)
						timestamp, timestampOK := res.Metadata().Annotations().Get(omni.RotateKubernetesCATimestamp)
						assertions.True(timestampOK)
						assertions.NotEmpty(timestamp)
					},
				)
			},
		)
	})
}
