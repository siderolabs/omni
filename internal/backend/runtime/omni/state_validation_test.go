// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestClusterValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	talos15 := "1.5.0"

	etcdBackupConfig := config.EtcdBackupParams{
		TickInterval: time.Minute,
		MinInterval:  time.Hour,
		MaxInterval:  24 * time.Hour,
	}

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.ClusterValidationOptions(state.WrapCore(innerSt), etcdBackupConfig, config.EmbeddedDiscoveryServiceParams{})...)

	talosVersion1 := omnires.NewTalosVersion(resources.DefaultNamespace, "1.4.0")
	talosVersion1.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.27.0", "1.27.1"}

	talosVersion2 := omnires.NewTalosVersion(resources.DefaultNamespace, talos15)
	talosVersion2.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.28.0", "1.28.1"}

	require.NoError(t, st.Create(ctx, talosVersion1))
	require.NoError(t, st.Create(ctx, talosVersion2))

	cluster := omnires.NewCluster(resources.DefaultNamespace, "test")

	// create
	err := st.Create(ctx, cluster)

	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "invalid talos version")

	cluster.TypedSpec().Value.TalosVersion = "1.4.0"
	cluster.TypedSpec().Value.KubernetesVersion = "1.26.0"

	err = st.Create(ctx, cluster)
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "invalid kubernetes version")

	cluster.TypedSpec().Value.KubernetesVersion = "1.27.0"

	require.NoError(t, st.Create(ctx, cluster))

	// update
	cluster.TypedSpec().Value.TalosVersion = talos15
	cluster.TypedSpec().Value.KubernetesVersion = "1.27.1"

	// incompatible update
	err = st.Update(ctx, cluster)
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "invalid kubernetes version")

	cluster.TypedSpec().Value.TalosVersion = talos15
	cluster.TypedSpec().Value.KubernetesVersion = "1.27.0"

	// incompatible update, but because the kubernetes version did not change, it's allowed
	require.NoError(t, st.Update(ctx, cluster))

	cluster.TypedSpec().Value.TalosVersion = "invalid"

	require.NoError(t, innerSt.Update(ctx, cluster))

	// invalid Talos version, but because it did not change, it's allowed

	cluster.TypedSpec().Value.KubernetesVersion = "1.28.0"

	require.NoError(t, st.Update(ctx, cluster))

	// try to enable encryption
	cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
		DiskEncryption: true,
	}

	err = st.Update(ctx, cluster)
	require.True(t, validated.IsValidationError(err), "expected validation error")

	cluster = omnires.NewCluster(resources.DefaultNamespace, "encryption")
	cluster.TypedSpec().Value.TalosVersion = "1.4.7"
	cluster.TypedSpec().Value.KubernetesVersion = "1.27.1"
	cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
		DiskEncryption: true,
	}

	err = st.Create(ctx, cluster)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	cluster.TypedSpec().Value.TalosVersion = talos15
	cluster.TypedSpec().Value.KubernetesVersion = "1.28.1"

	require.NoError(t, st.Create(ctx, cluster))
}

func TestClusterUseEmbeddedDiscoveryServiceValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	buildState := func(conf config.EmbeddedDiscoveryServiceParams) (inner, outer state.State) {
		innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
		st := validated.NewState(innerSt, omni.ClusterValidationOptions(state.WrapCore(innerSt), config.EtcdBackupParams{}, conf)...)

		return innerSt, state.WrapCore(st)
	}

	t.Run("disabled instance-wide - create", func(t *testing.T) {
		t.Parallel()

		_, st := buildState(config.EmbeddedDiscoveryServiceParams{
			Enabled: false,
		})

		cluster := omnires.NewCluster(resources.DefaultNamespace, "test")

		cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
			UseEmbeddedDiscoveryService: true,
		}

		err := st.Create(ctx, cluster)

		require.True(t, validated.IsValidationError(err), "expected validation error")
		assert.ErrorContains(t, err, "embedded discovery service is not enabled")
	})

	t.Run("disabled instance-wide - update", func(t *testing.T) {
		t.Parallel()

		// prepare a cluster which has the feature enabled, while it is disabled instance-wide
		innerSt, st := buildState(config.EmbeddedDiscoveryServiceParams{
			Enabled: false,
		})

		talosVersion := omnires.NewTalosVersion(resources.DefaultNamespace, "1.7.4")
		talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.30.1"}

		require.NoError(t, st.Create(ctx, talosVersion))

		cluster := omnires.NewCluster(resources.DefaultNamespace, "test")
		cluster.TypedSpec().Value.TalosVersion = "1.7.4"
		cluster.TypedSpec().Value.KubernetesVersion = "1.30.1"

		cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
			UseEmbeddedDiscoveryService: true,
		}

		require.NoError(t, innerSt.Create(ctx, cluster)) // use innerSt to skip the validation

		// update the cluster - it must pass through despite the feature being disabled instance wide, as the feature flag value is not changed
		cluster.TypedSpec().Value.Features.UseEmbeddedDiscoveryService = true
		cluster.TypedSpec().Value.Features.EnableWorkloadProxy = true

		assert.NoError(t, st.Update(ctx, cluster))
	})

	t.Run("enabled instance-wide", func(t *testing.T) {
		t.Parallel()

		_, st := buildState(config.EmbeddedDiscoveryServiceParams{
			Enabled: true,
		})

		talosVersion := omnires.NewTalosVersion(resources.DefaultNamespace, "1.7.4")
		talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.30.1"}

		require.NoError(t, st.Create(ctx, talosVersion))

		cluster := omnires.NewCluster(resources.DefaultNamespace, "test")
		cluster.TypedSpec().Value.TalosVersion = "1.7.4"
		cluster.TypedSpec().Value.KubernetesVersion = "1.30.1"
		cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{
			UseEmbeddedDiscoveryService: true,
		}

		require.NoError(t, st.Create(ctx, cluster))

		cluster.TypedSpec().Value.Features.UseEmbeddedDiscoveryService = false
		require.NoError(t, st.Update(ctx, cluster))
	})
}

func TestRelationLabelsValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.RelationLabelsValidationOptions()...)

	// MachineSet -> Cluster

	machineSet := omnires.NewMachineSet(resources.DefaultNamespace, "test-machine-set")

	err := st.Create(ctx, machineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelCluster))

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")

	assert.NoError(t, st.Create(ctx, machineSet))

	machineSet.Metadata().Labels().Delete(omnires.LabelCluster)

	err = st.Update(ctx, machineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelCluster))

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")

	// MachineSetNode -> MachineSet

	machineSetNode := omnires.NewMachineSetNode(resources.DefaultNamespace, "test-machine-set", machineSet)

	machineSetNode.Metadata().Labels().Delete(omnires.LabelMachineSet)

	err = st.Create(ctx, machineSetNode)

	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelMachineSet))

	machineSetNode.Metadata().Labels().Set(omnires.LabelMachineSet, "test-machine-set")

	assert.NoError(t, st.Create(ctx, machineSetNode))

	machineSetNode.Metadata().Labels().Delete(omnires.LabelMachineSet)

	err = st.Update(ctx, machineSetNode)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelMachineSet))
}

func TestMachineSetValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	etcdBackupStoreFactory, err := store.NewStoreFactory()
	require.NoError(t, err)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.MachineSetValidationOptions(innerSt, etcdBackupStoreFactory)...)

	machineSet1 := omnires.NewMachineSet(resources.DefaultNamespace, "test-cluster-wrong-suffix")

	require.NoError(t, st.Create(ctx, omnires.NewCluster(resources.DefaultNamespace, "test-cluster")))

	// no cluster label

	err = st.Create(ctx, machineSet1)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "cluster label is missing")

	machineSet1.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")

	// no role label

	err = st.Create(ctx, machineSet1)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "machine set must have either")

	machineSet1.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")
	machineSet1.Metadata().Labels().Set(omnires.LabelCluster, "wrong-cluster")

	// machine set with a wrong prefix

	err = st.Create(ctx, machineSet1)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, `machine set of cluster "wrong-cluster" ID must have "wrong-cluster-" as prefix`)

	machineSet1.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")

	// control plane machine set with wrong id

	err = st.Create(ctx, machineSet1)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "control plane machine set must have ID")

	// control plane machine set with correct id on already bootstrapped cluster

	status := omnires.NewClusterBootstrapStatus(resources.DefaultNamespace, "test-cluster")

	status.TypedSpec().Value.Bootstrapped = true

	require.NoError(t, st.Create(ctx, status))

	machineSet2 := omnires.NewMachineSet(resources.DefaultNamespace, "test-cluster-control-planes")

	machineSet2.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")
	machineSet2.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")

	err = st.Create(ctx, machineSet2)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "adding control plane machine set to an already bootstrapped cluster is not allowed")

	require.NoError(t, st.Destroy(ctx, status.Metadata()))

	// control plane machine set with correct id

	err = st.Create(ctx, machineSet2)
	assert.NoError(t, err)

	// worker machine set with id reserved for control plane

	machineSet3 := omnires.NewMachineSet(resources.DefaultNamespace, "test-cluster-control-planes")

	machineSet3.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")
	machineSet3.Metadata().Labels().Set(omnires.LabelWorkerRole, "")

	err = st.Create(ctx, machineSet3)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "worker machine set must not have ID")

	// no cluster exists

	machineSet4 := omnires.NewMachineSet(resources.DefaultNamespace, "test-cluster-tearing-down-control-planes")
	machineSet4.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster-tearing-down")
	machineSet4.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")

	// cluster exists and is tearing down

	require.NoError(t, st.Create(ctx, omnires.NewCluster(resources.DefaultNamespace, "test-cluster-tearing-down")))
	_, err = state.WrapCore(st).Teardown(ctx, omnires.NewCluster(resources.DefaultNamespace, "test-cluster-tearing-down").Metadata())
	require.NoError(t, err)

	err = st.Create(ctx, machineSet4)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "tearing down")
}

func TestMachineSetBootstrapSpecValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	clusterID := "test-cluster"
	aesCBCSecret := "aes-cbc-secret"
	secretboxEncryptionSecret := "secretbox-secret"
	clusterUUID := "70fcf307-f9be-40dc-81de-6a427adfc1d4"
	snapshotName := "test-snapshot"

	etcdBackupStoreFactory := mockEtcdBackupStoreFactory{
		store: &mockEtcdBackupStore{
			clusterUUID:  clusterUUID,
			snapshotName: snapshotName,
			backupData: etcdbackup.BackupData{
				AESCBCEncryptionSecret:    aesCBCSecret,
				SecretboxEncryptionSecret: secretboxEncryptionSecret,
			},
		},
	}

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.MachineSetValidationOptions(innerSt, &etcdBackupStoreFactory)...)

	cluster := omnires.NewCluster(resources.DefaultNamespace, clusterID)

	require.NoError(t, st.Create(ctx, cluster))

	// worker machine set with bootstrap spec - not allowed

	workerMachineSet := omnires.NewMachineSet(resources.DefaultNamespace, omnires.WorkersResourceID(clusterID))

	workerMachineSet.Metadata().Labels().Set(omnires.LabelCluster, clusterID)
	workerMachineSet.Metadata().Labels().Set(omnires.LabelWorkerRole, "")

	workerMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
		ClusterUuid: clusterUUID,
		Snapshot:    snapshotName,
	}

	err := st.Create(ctx, workerMachineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "bootstrap spec is not allowed for worker machine sets")

	// cluster UUID mismatch

	clusterUUIDRes := omnires.NewClusterUUID(clusterID)
	clusterUUIDRes.TypedSpec().Value.Uuid = clusterUUID

	clusterUUIDRes.Metadata().Labels().Set(omnires.LabelClusterUUID, clusterUUID)

	require.NoError(t, st.Create(ctx, clusterUUIDRes))

	backupData := omnires.NewBackupData(clusterID)
	backupData.TypedSpec().Value.AesCbcEncryptionSecret = "wrong-aes-cbc-secret"
	backupData.TypedSpec().Value.SecretboxEncryptionSecret = "wrong-secretbox-secret"
	backupData.TypedSpec().Value.EncryptionKey = []byte("wvd90ml3i7z9td0g2skir9mym6ax4zoz")

	require.NoError(t, st.Create(ctx, backupData))

	controlPlaneMachineSet := omnires.NewMachineSet(resources.DefaultNamespace, omnires.ControlPlanesResourceID(clusterID))

	controlPlaneMachineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")
	controlPlaneMachineSet.Metadata().Labels().Set(omnires.LabelCluster, clusterID)

	// non-existent snapshot

	controlPlaneMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
		ClusterUuid: clusterUUID,
		Snapshot:    "wrong-snapshot-name",
	}

	err = st.Create(ctx, controlPlaneMachineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "not found")

	// aes cbc encryption secret mismatch

	controlPlaneMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
		ClusterUuid: clusterUUID,
		Snapshot:    snapshotName,
	}

	err = st.Create(ctx, controlPlaneMachineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "aes cbc encryption secret mismatch")

	// secretbox encryption secret mismatch

	backupData.TypedSpec().Value.AesCbcEncryptionSecret = aesCBCSecret

	require.NoError(t, st.Update(ctx, backupData))

	err = st.Create(ctx, controlPlaneMachineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "secretbox encryption secret mismatch")

	// success

	backupData.TypedSpec().Value.SecretboxEncryptionSecret = secretboxEncryptionSecret

	require.NoError(t, st.Update(ctx, backupData))

	assert.NoError(t, st.Create(ctx, controlPlaneMachineSet))

	// update bootstrap spec, which is immutable

	controlPlaneMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
		ClusterUuid: clusterUUID,
		Snapshot:    "different",
	}

	err = st.Update(ctx, controlPlaneMachineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "bootstrap spec is immutable after creation")
}

func TestMachineSetLockedAnnotation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.MachineSetNodeValidationOptions(state.WrapCore(innerSt))...)

	machineSet := omnires.NewMachineSet(resources.DefaultNamespace, "test-machine-set")
	machineSetNode := omnires.NewMachineSetNode(resources.DefaultNamespace, "test-machine-set", machineSet)

	machineSetNode.Metadata().Annotations().Set(omnires.MachineLocked, "")

	assert := assert.New(t)

	assert.NoError(st.Create(ctx, machineSetNode))
	assert.NoError(st.Create(ctx, machineSet))

	err := st.Destroy(ctx, machineSetNode.Metadata())
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, "machine set node is locked")

	s := state.WrapCore(st)

	_, err = s.Teardown(ctx, machineSetNode.Metadata())
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, "machine set node is locked")

	machineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")
	assert.NoError(st.Update(ctx, machineSet))

	machineSetNode = omnires.NewMachineSetNode(resources.DefaultNamespace, "test-machine-set-1", machineSet)
	assert.NoError(st.Create(ctx, machineSetNode))

	_, err = safe.StateUpdateWithConflicts(ctx, s, machineSetNode.Metadata(), func(n *omnires.MachineSetNode) error {
		n.Metadata().Annotations().Set(omnires.MachineLocked, "")

		return nil
	})

	assert.ErrorContains(err, "locking controlplanes is not allowed")

	machineSetNode = omnires.NewMachineSetNode(resources.DefaultNamespace, "test-machine-set-2", machineSet)
	machineSetNode.Metadata().Annotations().Set(omnires.MachineLocked, "")
	assert.ErrorContains(st.Create(ctx, machineSetNode), "locking controlplanes is not allowed")
}

func TestIdentitySAMLValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.IdentityValidationOptions(config.SAMLParams{
		Enabled: true,
	})...)

	user := auth.NewIdentity(resources.DefaultNamespace, "aaa@example.org")

	assert := assert.New(t)

	assert.NoError(innerSt.Create(ctx, user))

	s := state.WrapCore(st)

	checkError := func(err error) {
		assert.Error(err)
		assert.True(validated.IsValidationError(err), "expected validation error")
		assert.ErrorContains(err, "updating identity is not allowed in SAML mode")
	}

	_, err := safe.StateUpdateWithConflicts(ctx, s, user.Metadata(), func(n *auth.Identity) error {
		n.TypedSpec().Value.UserId = "bacd"

		return nil
	})

	checkError(err)

	_, err = safe.StateUpdateWithConflicts(ctx, s, user.Metadata(), func(n *auth.Identity) error {
		n.Metadata().Labels().Set("label", "a")

		return nil
	})

	checkError(err)

	_, err = safe.StateUpdateWithConflicts(ctx, s, user.Metadata(), func(n *auth.Identity) error {
		n.Metadata().Annotations().Set("key", "value")

		return nil
	})

	checkError(err)

	_, err = s.Teardown(ctx, user.Metadata())
	assert.NoError(err)
}

func TestCreateIdentityValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.IdentityValidationOptions(config.SAMLParams{})...)

	assert := assert.New(t)

	err := st.Create(ctx, auth.NewIdentity(resources.DefaultNamespace, "aaA"))

	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, "must be lowercase")
	assert.ErrorContains(err, "not a valid email address")

	err = st.Create(ctx, auth.NewIdentity(resources.DefaultNamespace, "aaa"))

	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, "not a valid email address")

	assert.NoError(st.Create(ctx, auth.NewIdentity(resources.DefaultNamespace, "aaa@example.org")))
}

func TestExposedServiceAliasValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.ExposedServiceValidationOptions()...)

	exposedService := omnires.NewExposedService(resources.DefaultNamespace, "test-exposed-service")

	err := st.Create(ctx, exposedService)

	require.True(t, validated.IsValidationError(err), "expected validation error")
	require.ErrorContains(t, err, "alias must be set")

	exposedService.Metadata().Labels().Set(omnires.LabelExposedServiceAlias, "test-alias")

	require.NoError(t, st.Create(ctx, exposedService))

	exposedService.Metadata().Labels().Set(omnires.LabelExposedServiceAlias, "test-alias-updated")

	err = st.Update(ctx, exposedService)

	require.True(t, validated.IsValidationError(err), "expected validation error")
	require.ErrorContains(t, err, "alias cannot be changed")

	exposedService.Metadata().Labels().Set(omnires.LabelExposedServiceAlias, "test-alias")

	exposedService.TypedSpec().Value.Port = 12345

	require.NoError(t, st.Update(ctx, exposedService))
}

func TestConfigPatchValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.ConfigPatchValidationOptions(innerSt)...)

	configPatch := omnires.NewConfigPatch(resources.DefaultNamespace, "test-config-patch")

	patchDataNotAllowed := strings.TrimSpace(`
cluster:
  clusterName: test-cluster-name
`)

	configPatch.TypedSpec().Value.Data = patchDataNotAllowed

	err := st.Create(ctx, configPatch)
	require.ErrorContains(t, err, "is not allowed in the config patch")

	patchDataAllowed := strings.TrimSpace(`
machine:
  env:
    bla: bla
`)

	configPatch.TypedSpec().Value.Data = patchDataAllowed

	err = st.Create(ctx, configPatch)
	require.NoError(t, err)

	configPatch.TypedSpec().Value.Data = patchDataNotAllowed

	err = st.Update(ctx, configPatch)
	require.ErrorContains(t, err, "is not allowed in the config patch")

	clusterID := "tearing-one"

	cluster := omnires.NewCluster(resources.DefaultNamespace, clusterID)

	require.NoError(t, st.Create(ctx, cluster))
	_, err = innerSt.Teardown(ctx, cluster.Metadata())
	require.NoError(t, err)

	configPatch = omnires.NewConfigPatch(resources.DefaultNamespace, "another")
	configPatch.Metadata().Labels().Set(omnires.LabelCluster, clusterID)

	err = st.Create(ctx, configPatch)
	require.ErrorContains(t, err, "tearing down")

	msID := "machineset"

	ms := omnires.NewMachineSet(resources.DefaultNamespace, msID)

	require.NoError(t, st.Create(ctx, ms))
	_, err = innerSt.Teardown(ctx, ms.Metadata())
	require.NoError(t, err)

	configPatch.Metadata().Labels().Set(omnires.LabelCluster, "some")
	configPatch.Metadata().Labels().Set(omnires.LabelMachineSet, msID)

	err = st.Create(ctx, configPatch)
	require.ErrorContains(t, err, "tearing down")
}

func TestEtcdBackupValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	st := validated.NewState(innerSt, omni.EtcdManualBackupValidationOptions()...)

	backup := omnires.NewEtcdManualBackup("test-etcd-manual-backup")
	backup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(-time.Hour))

	err := st.Create(ctx, backup)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	backup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(-30 * time.Second))

	require.NoError(t, st.Create(ctx, backup))
}

func TestSAMLLabelRuleValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.SAMLLabelRuleValidationOptions()...)

	labelRule := auth.NewSAMLLabelRule(resources.DefaultNamespace, "test-label-rule")
	labelRule.TypedSpec().Value.AssignRoleOnRegistration = "invalid"
	labelRule.TypedSpec().Value.MatchLabels = []string{"--invalid--- ===== 5"}

	err := st.Create(ctx, labelRule)
	assert.ErrorContains(t, err, "unknown role")
	assert.ErrorContains(t, err, "invalid match labels")

	labelRule.TypedSpec().Value.AssignRoleOnRegistration = string(role.Operator)

	err = st.Create(ctx, labelRule)
	assert.ErrorContains(t, err, "invalid match labels")

	labelRule.TypedSpec().Value.MatchLabels = []string{"a", "b"}

	err = st.Create(ctx, labelRule)
	assert.NoError(t, err)
}

func TestMachineSetClassesValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	etcdBackupStoreFactory, err := store.NewStoreFactory()
	require.NoError(t, err)

	st := validated.NewState(innerSt,
		append(
			omni.MachineSetNodeValidationOptions(state.WrapCore(innerSt)),
			omni.MachineSetValidationOptions(state.WrapCore(innerSt), etcdBackupStoreFactory)...,
		)...,
	)

	require.NoError(t, st.Create(ctx, omnires.NewCluster(resources.DefaultNamespace, "test-cluster")))

	machineSet := omnires.NewMachineSet(resources.DefaultNamespace, "test-cluster-control-planes")

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")
	machineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")

	machineSet.TypedSpec().Value.MachineClass = &specs.MachineSetSpec_MachineClass{}

	err = st.Create(ctx, machineSet)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	machineSet.TypedSpec().Value.MachineClass = &specs.MachineSetSpec_MachineClass{
		Name: "none",
	}

	err = st.Create(ctx, machineSet)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	machineClass := omnires.NewMachineClass(resources.DefaultNamespace, "test-class")

	require.NoError(t, st.Create(ctx, machineClass))

	machineSet.TypedSpec().Value.MachineClass = &specs.MachineSetSpec_MachineClass{
		Name: machineClass.Metadata().ID(),
	}

	require.NoError(t, st.Create(ctx, machineSet))

	machineSetNode := omnires.NewMachineSetNode(resources.DefaultNamespace, "machine", machineSet)

	err = st.Create(ctx, machineSetNode)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	machineSet.TypedSpec().Value.MachineClass.Name = "none"

	err = st.Update(ctx, machineSet)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// expect no error, as the machine mgmt mode change is allowed if there are no nodes in the machine set
	machineSet.TypedSpec().Value.MachineClass = nil

	require.NoError(t, st.Update(ctx, machineSet))

	// add a node
	require.NoError(t, st.Create(ctx, machineSetNode))

	// machine set mgmt mode change is not allowed anymore
	machineSet.TypedSpec().Value.MachineClass = &specs.MachineSetSpec_MachineClass{
		Name: machineClass.Metadata().ID(),
	}

	err = st.Update(ctx, machineSet)
	require.True(t, validated.IsValidationError(err), "expected validation error")
	require.ErrorContains(t, err, "machine set is not empty")
}

func TestMachineClassValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	st := validated.NewState(innerSt,
		omni.MachineClassValidationOptions(state.WrapCore(innerSt))...,
	)

	require.NoError(t, st.Create(ctx, omnires.NewCluster(resources.DefaultNamespace, "test-cluster")))

	machineSet := omnires.NewMachineSet(resources.DefaultNamespace, "test-cluster-control-planes")

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")
	machineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")

	machineSet.TypedSpec().Value.MachineClass = &specs.MachineSetSpec_MachineClass{
		Name: "test-class",
	}

	machineClass := omnires.NewMachineClass(resources.DefaultNamespace, "test-class")

	require.NoError(t, st.Create(ctx, machineClass))
	require.NoError(t, st.Create(ctx, machineSet))

	err := st.Destroy(ctx, machineClass.Metadata())

	require.True(t, validated.IsValidationError(err), "expected validation error")

	require.NoError(t, st.Destroy(ctx, machineSet.Metadata()))
	require.NoError(t, st.Destroy(ctx, machineClass.Metadata()))
}

func TestS3ConfigValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	st := validated.NewState(innerSt, omni.S3ConfigValidationOptions()...)

	res := omnires.NewEtcdBackupS3Conf()
	res.TypedSpec().Value = &specs.EtcdBackupS3ConfSpec{}

	require.NoError(t, st.Create(ctx, res))

	res.TypedSpec().Value = &specs.EtcdBackupS3ConfSpec{
		Bucket:          "",
		Region:          "us-east-1",
		Endpoint:        "http://localhost:27777",
		AccessKeyId:     "access",
		SecretAccessKey: "secret123",
		SessionToken:    "",
	}

	require.True(t, validated.IsValidationError(st.Update(ctx, res)), "expected validation error")

	res.TypedSpec().Value = &specs.EtcdBackupS3ConfSpec{
		Bucket:          "temp-buclet",
		Region:          "us-east-1",
		Endpoint:        "http://localhost:27777",
		AccessKeyId:     "",
		SecretAccessKey: "secret123",
		SessionToken:    "",
	}

	require.True(t, validated.IsValidationError(st.Update(ctx, res)), "expected validation error")

	res.TypedSpec().Value = &specs.EtcdBackupS3ConfSpec{}

	require.NoError(t, st.Update(ctx, res))
}

func TestSchematicConfigurationValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	st := validated.NewState(innerSt, omni.SchematicConfigurationValidationOptions()...)

	res := omnires.NewSchematicConfiguration(resources.DefaultNamespace, "test")

	require.True(t, validated.IsValidationError(st.Create(ctx, res)), "expected validation error")

	res.Metadata().Labels().Set(omnires.LabelClusterMachine, "test")

	require.True(t, validated.IsValidationError(st.Create(ctx, res)), "expected validation error")

	res.TypedSpec().Value.SchematicId = "abab"

	require.NoError(t, st.Create(ctx, res))

	res.Metadata().Labels().Delete(omnires.LabelClusterMachine)

	require.True(t, validated.IsValidationError(st.Update(ctx, res)), "expected validation error")

	res.Metadata().Labels().Set(omnires.LabelMachineSet, "test")

	require.NoError(t, st.Update(ctx, res))
}

type mockEtcdBackupStoreFactory struct {
	store etcdbackup.Store
}

func (m *mockEtcdBackupStoreFactory) GetStore() (etcdbackup.Store, error) {
	return m.store, nil
}

func (m *mockEtcdBackupStoreFactory) Start(context.Context, state.State, *zap.Logger) error {
	return nil
}

func (m *mockEtcdBackupStoreFactory) Description() string { return "mock" }

type mockEtcdBackupStore struct {
	backupData   etcdbackup.BackupData
	clusterUUID  string
	snapshotName string
}

func (m *mockEtcdBackupStore) ListBackups(context.Context, string) (etcdbackup.InfoIterator, error) {
	return nil, nil //nolint:nilnil
}

func (m *mockEtcdBackupStore) Upload(context.Context, etcdbackup.Description, io.Reader) error {
	return nil
}

func (m *mockEtcdBackupStore) Download(_ context.Context, _ []byte, clusterUUID, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	if m.clusterUUID == clusterUUID && m.snapshotName == snapshotName {
		return m.backupData, io.NopCloser(strings.NewReader("test")), nil
	}

	return etcdbackup.BackupData{}, nil, errors.New("not found")
}
