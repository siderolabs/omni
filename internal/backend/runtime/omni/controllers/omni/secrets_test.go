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

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

type ClusterSecretsSuite struct {
	OmniSuite
}

func (suite *ClusterSecretsSuite) TestNewSecrets() {
	require := suite.Require()

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))

	cluster := omni.NewCluster("clusterID")
	cluster.TypedSpec().Value.TalosVersion = "1.2.3"
	require.NoError(suite.state.Create(suite.ctx, cluster))

	machineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(cluster.Metadata().ID()))
	require.NoError(suite.state.Create(suite.ctx, machineSet))

	var foundClusterSecrets *omni.ClusterSecrets

	// The only test I could think at this time, was simply to test bundle existence.
	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterSecrets(cluster.Metadata().ID()).Metadata(),
		func(res *omni.ClusterSecrets, _ *assert.Assertions) {
			foundClusterSecrets = res
			clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
			suite.Require().NotEmpty(clusterSecretsSpec.GetData())

			var bundle secrets.Bundle

			err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
			suite.Require().NoError(err)
			suite.Require().NotEmpty(bundle)
			suite.Require().Equal(clusterSecretsSpec.Imported, false)
		})

	// Check that we can get cluster secrets by metadata.
	assertResource(
		&suite.OmniSuite,
		*foundClusterSecrets.Metadata(),
		func(*omni.ClusterSecrets, *assert.Assertions) {},
	)

	// Check that cluster secrets will be removed when cluster is removed.
	rtestutils.Destroy[*omni.Cluster](suite.ctx, suite.T(), suite.state, []resource.ID{cluster.Metadata().ID()})
	assertNoResource(&suite.OmniSuite, foundClusterSecrets)
}

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

func (suite *ClusterSecretsSuite) TestSecretsFromBackup() {
	require := suite.Require()

	suite.startRuntime()
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(&mockBackupStoreFactory{})))

	cluster := omni.NewCluster("clusterID")
	cluster.TypedSpec().Value.TalosVersion = "1.2.3"

	require.NoError(suite.state.Create(suite.ctx, cluster))

	// create ClusterUUID, as it will be looked up by SecretsController to find the source cluster ID
	clusterUUID := omni.NewClusterUUID(cluster.Metadata().ID())
	clusterUUID.TypedSpec().Value.Uuid = "test-uuid"

	clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, "test-uuid")

	require.NoError(suite.state.Create(suite.ctx, clusterUUID))

	// create BackupData, as it will be looked up by SecretsController to get the encryption key
	backupData := omni.NewBackupData(cluster.Metadata().ID())

	require.NoError(suite.state.Create(suite.ctx, backupData))

	machineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(cluster.Metadata().ID()))
	machineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
		ClusterUuid: "test-uuid",
		Snapshot:    "test-snapshot",
	}

	machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	require.NoError(suite.state.Create(suite.ctx, machineSet))

	var foundClusterSecrets *omni.ClusterSecrets

	// The only test I could think at this time, was simply to test bundle existence.
	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterSecrets(cluster.Metadata().ID()).Metadata(),
		func(res *omni.ClusterSecrets, _ *assert.Assertions) {
			foundClusterSecrets = res
			clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
			suite.Require().NotEmpty(clusterSecretsSpec.GetData())

			var bundle secrets.Bundle

			err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
			suite.Require().NoError(err)
			suite.Require().NotEmpty(bundle)

			// assert that the AES-CBC and Secretbox encryption secrets are set to the values from the backup data
			suite.Require().Equal("aes-cbc-test", bundle.Secrets.AESCBCEncryptionSecret)
			suite.Require().Equal("secretbox-test", bundle.Secrets.SecretboxEncryptionSecret)
		})
}

//go:embed testdata/secrets-valid.yaml
var validSecretsBundle string

func (suite *ClusterSecretsSuite) TestImportedSecrets() {
	require := suite.Require()

	suite.startRuntime()
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(&mockBackupStoreFactory{})))

	clusterID := "clusterID"

	// create ImportedClusterSecret, as it will be looked up by SecretsController to attempt importing secrets bundle
	importedClusterSecrets := omni.NewImportedClusterSecrets(clusterID)
	importedClusterSecrets.TypedSpec().Value.Data = validSecretsBundle

	require.NoError(suite.state.Create(suite.ctx, importedClusterSecrets))

	cluster := omni.NewCluster(clusterID)
	cluster.TypedSpec().Value.TalosVersion = "1.10.5"

	require.NoError(suite.state.Create(suite.ctx, cluster))

	// create ClusterUUID, as it will be looked up by SecretsController to find the source cluster ID
	clusterUUID := omni.NewClusterUUID(cluster.Metadata().ID())
	clusterUUID.TypedSpec().Value.Uuid = "test-uuid"

	clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, "test-uuid")

	require.NoError(suite.state.Create(suite.ctx, clusterUUID))

	machineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(cluster.Metadata().ID()))
	machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	require.NoError(suite.state.Create(suite.ctx, machineSet))

	var foundClusterSecrets *omni.ClusterSecrets

	assertResource(
		&suite.OmniSuite,
		*omni.NewClusterSecrets(cluster.Metadata().ID()).Metadata(),
		func(res *omni.ClusterSecrets, _ *assert.Assertions) {
			foundClusterSecrets = res
			clusterSecretsSpec := foundClusterSecrets.TypedSpec().Value
			suite.Require().NotEmpty(clusterSecretsSpec.GetData())

			var bundle secrets.Bundle

			err := json.Unmarshal(clusterSecretsSpec.Data, &bundle)
			suite.Require().NoError(err)
			suite.Require().NotEmpty(bundle)
			suite.Require().Equal(clusterSecretsSpec.Imported, true)
		})
}

func TestClusterSecretsSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterSecretsSuite))
}
