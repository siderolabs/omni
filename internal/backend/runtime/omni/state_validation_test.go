// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"crypto/rand"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"iter"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
)

//go:embed testdata/infra.json
var schema []byte

func GenerateRandomString(t *testing.T, n int) string {
	t.Helper()

	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"

	ret := make([]byte, n)

	for i := range n {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		require.NoError(t, err)

		ret[i] = letters[num.Int64()]
	}

	return string(ret)
}

func TestClusterValidation(t *testing.T) { //nolint:gocognit,maintidx
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	etcdBackupConfig := config.EtcdBackup{
		TickInterval: pointer.To(time.Minute),
		MinInterval:  pointer.To(time.Hour),
		MaxInterval:  pointer.To(24 * time.Hour),
	}

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.ClusterValidationOptions(state.WrapCore(innerSt), etcdBackupConfig, config.EmbeddedDiscoveryService{})...)

	// prepare talos versions
	for _, prep := range []struct {
		version       string
		compatibleK8s []string
		deprecated    bool
	}{
		{"1.3.0", []string{"1.26.0", "1.27.1"}, true},
		{"1.3.4", []string{"1.26.0", "1.27.1"}, true},
		{"1.4.0", []string{"1.27.0", "1.27.1"}, false},
		{"1.5.0", []string{"1.28.0", "1.28.1", "1.29.0", "1.30.0", "1.30.1", "1.30.2"}, false},
	} {
		talosVersion := omnires.NewTalosVersion(prep.version)
		talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = prep.compatibleK8s
		talosVersion.TypedSpec().Value.Deprecated = prep.deprecated

		require.NoError(t, st.Create(ctx, talosVersion))
	}

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		createTests := []struct { //nolint:govet
			name              string
			talosVersion      string
			kubernetesVersion string
			features          *specs.ClusterSpec_Features

			shouldFail    bool
			errorContains string
			errorIs       func(error) bool
		}{
			{
				name:          "no talos version set",
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "invalid talos version",
			},
			{
				name:          "unsupported talos version",
				talosVersion:  "1.3.0",
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "is no longer supported",
			},
			{
				name:          "unsupported kubernetes version",
				talosVersion:  "1.4.0",
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "invalid kubernetes version",
			},
			{
				name:              "encryption on unsupported talos",
				talosVersion:      "1.4.0",
				kubernetesVersion: "1.27.1",
				features: &specs.ClusterSpec_Features{
					DiskEncryption: true,
				},
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "disk encryption is supported only for Talos version",
			},
			{
				name:              "success",
				talosVersion:      "1.4.0",
				kubernetesVersion: "1.27.0",
			},
			{
				name:              "encryption success",
				talosVersion:      "1.5.0",
				kubernetesVersion: "1.28.0",
				features: &specs.ClusterSpec_Features{
					DiskEncryption: true,
				},
			},
		}

		for _, tc := range createTests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				// generate random cluster name
				clusterName := "test-cluster-" + GenerateRandomString(t, 6)

				cluster := omnires.NewCluster(clusterName)

				t.Cleanup(func() {
					_ = innerSt.Destroy(ctx, cluster.Metadata()) //nolint:errcheck // ignore error on cleanup
				})

				if tc.talosVersion != "" {
					cluster.TypedSpec().Value.TalosVersion = tc.talosVersion
				}

				if tc.kubernetesVersion != "" {
					cluster.TypedSpec().Value.KubernetesVersion = tc.kubernetesVersion
				}

				if tc.features != nil {
					cluster.TypedSpec().Value.Features = tc.features
				}

				err := st.Create(ctx, cluster)

				if tc.shouldFail {
					require.Error(t, err, "create expected to fail")

					if tc.errorIs != nil {
						require.True(t, tc.errorIs(err), "expected error to match the target")
					}

					if tc.errorContains != "" {
						assert.ErrorContains(t, err, tc.errorContains)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		type clusterVersions struct {
			features            *specs.ClusterSpec_Features
			initialTalosVersion string
			talosVersion        string
			kubernetesVersion   string
		}

		type ongoingUpgrade struct {
			fromVersion string
			toVersion   string
		}

		defaultVersions := clusterVersions{
			talosVersion:      "1.4.0",
			kubernetesVersion: "1.27.0",
		}

		updateTests := []struct { //nolint:govet
			name string

			from      clusterVersions
			to        clusterVersions
			upgrading *ongoingUpgrade

			shouldFail    bool
			errorIs       func(error) bool
			errorContains string
		}{
			{
				name: "incompatible update",
				from: defaultVersions,
				to: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.27.1",
				},
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "invalid kubernetes version",
			},
			{
				name: "incompatible update no k8s update",
				from: defaultVersions,
				to: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: defaultVersions.kubernetesVersion,
				},
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "invalid kubernetes version",
			},
			{
				name: "invalid talos kubernetes upgrade",
				from: clusterVersions{
					talosVersion:      "invalid",
					kubernetesVersion: "1.27.0",
				},
				to: clusterVersions{
					talosVersion:      "invalid",
					kubernetesVersion: "1.28.0",
				},
			},
			{
				name: "enable encryption on update",
				from: defaultVersions,
				to: clusterVersions{
					talosVersion:      defaultVersions.talosVersion,
					kubernetesVersion: defaultVersions.kubernetesVersion,
					features: &specs.ClusterSpec_Features{
						DiskEncryption: true,
					},
				},
				shouldFail: true,
				errorIs:    validated.IsValidationError,
			},
			{
				name: "deprecated upgrade",
				from: clusterVersions{
					talosVersion:      "1.3.0",
					kubernetesVersion: "1.26.0",
				},
				to: clusterVersions{
					talosVersion:      "1.3.4",
					kubernetesVersion: "1.26.0",
				},
			},
			{
				name: "over 1 minor jump upgrade",
				from: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.28.0",
				},
				to: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.30.0",
				},
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "kubernetes version is not supported for upgrade",
			},
			{
				name: "minor downgrade",
				from: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.30.0",
				},
				to: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.29.0",
				},
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "kubernetes version is not supported for upgrade",
			},
			{
				name: "minor downgrade during ongoing upgrade",
				from: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.30.0",
				},
				to: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.29.0",
				},
				upgrading: &ongoingUpgrade{
					fromVersion: "1.29.0",
					toVersion:   "1.30.0",
				},
			},
			{
				name: "minor downgrade to different version during ongoing upgrade",
				from: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.30.0",
				},
				to: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.29.1",
				},
				upgrading: &ongoingUpgrade{
					fromVersion: "1.29.0",
					toVersion:   "1.30.0",
				},
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "kubernetes version is not supported for upgrade",
			},
			{
				name: "patch upgrade during ongoing upgrade",
				from: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.30.0",
				},
				to: clusterVersions{
					talosVersion:      "1.5.0",
					kubernetesVersion: "1.30.2",
				},
				upgrading: &ongoingUpgrade{
					fromVersion: "1.30.0",
					toVersion:   "1.30.1",
				},
			},
			{
				name: "minor downgrade to lower version than initial version",
				from: clusterVersions{
					talosVersion:        "1.5.0",
					initialTalosVersion: "1.5.0",
					kubernetesVersion:   "1.30.0",
				},
				to: clusterVersions{
					talosVersion:      "1.4.0",
					kubernetesVersion: "1.30.0",
				},
				shouldFail:    true,
				errorIs:       validated.IsValidationError,
				errorContains: "downgrading from version \"1.5.0\" to \"1.4.0\" is not supported",
			},
		}

		// update
		for _, tc := range updateTests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				// generate random cluster name
				clusterName := "test-cluster-" + GenerateRandomString(t, 6)

				cluster := omnires.NewCluster(clusterName)
				clusterConfigVersion := omnires.NewClusterConfigVersion(clusterName)

				t.Cleanup(func() {
					_ = innerSt.Destroy(ctx, cluster.Metadata())              //nolint:errcheck // ignore error on cleanup
					_ = innerSt.Destroy(ctx, clusterConfigVersion.Metadata()) //nolint:errcheck // ignore error on cleanup
				})

				cluster.TypedSpec().Value.TalosVersion = tc.from.talosVersion
				cluster.TypedSpec().Value.KubernetesVersion = tc.from.kubernetesVersion
				cluster.TypedSpec().Value.Features = tc.from.features

				initialTalosVersion := tc.from.initialTalosVersion
				if initialTalosVersion == "" {
					initialTalosVersion = tc.from.talosVersion
				}

				clusterConfigVersion.TypedSpec().Value.Version = fmt.Sprintf("v%s", initialTalosVersion)

				require.NoError(t, innerSt.Create(ctx, cluster))
				require.NoError(t, innerSt.Create(ctx, clusterConfigVersion))

				if tc.upgrading != nil {
					kubernetesUpgrade := omnires.NewKubernetesUpgradeStatus(clusterName)

					t.Cleanup(func() {
						_ = innerSt.Destroy(ctx, kubernetesUpgrade.Metadata()) //nolint:errcheck // ignore error on cleanup
					})

					kubernetesUpgrade.TypedSpec().Value.LastUpgradeVersion = tc.upgrading.fromVersion
					kubernetesUpgrade.TypedSpec().Value.CurrentUpgradeVersion = tc.upgrading.toVersion
					require.NoError(t, innerSt.Create(ctx, kubernetesUpgrade))
				}

				if tc.to.talosVersion != "" {
					cluster.TypedSpec().Value.TalosVersion = tc.to.talosVersion
				}

				if tc.to.kubernetesVersion != "" {
					cluster.TypedSpec().Value.KubernetesVersion = tc.to.kubernetesVersion
				}

				if tc.to.features != nil {
					cluster.TypedSpec().Value.Features = tc.to.features
				}

				err := st.Update(ctx, cluster)

				if tc.shouldFail {
					require.Error(t, err, "update expected to fail")

					if tc.errorIs != nil {
						require.True(t, tc.errorIs(err), "expected error to be")
					}

					if tc.errorContains != "" {
						assert.ErrorContains(t, err, tc.errorContains)
					}
				} else {
					assert.NoError(t, err, "update expected to succeed")
				}
			})
		}
	})
}

func TestClusterUseEmbeddedDiscoveryServiceValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	buildState := func(conf config.EmbeddedDiscoveryService) (inner, outer state.State) {
		innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
		st := validated.NewState(innerSt, omni.ClusterValidationOptions(state.WrapCore(innerSt), config.EtcdBackup{}, conf)...)

		return innerSt, state.WrapCore(st)
	}

	t.Run("disabled instance-wide - create", func(t *testing.T) {
		t.Parallel()

		_, st := buildState(config.EmbeddedDiscoveryService{
			Enabled: pointer.To(false),
		})

		cluster := omnires.NewCluster("test")

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
		innerSt, st := buildState(config.EmbeddedDiscoveryService{
			Enabled: pointer.To(false),
		})

		talosVersion := omnires.NewTalosVersion("1.7.4")
		talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.30.1"}

		require.NoError(t, st.Create(ctx, talosVersion))

		cluster := omnires.NewCluster("test")
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

		_, st := buildState(config.EmbeddedDiscoveryService{
			Enabled: pointer.To(true),
		})

		talosVersion := omnires.NewTalosVersion("1.7.4")
		talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.30.1"}

		require.NoError(t, st.Create(ctx, talosVersion))

		cluster := omnires.NewCluster("test")
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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.RelationLabelsValidationOptions()...)

	clusterID := "test-cluster" //nolint:goconst
	differentClusterID := "different-cluster"
	machineSetID := "test-machine-set"
	differentMachineSetID := "different-machine-set"
	machineSetNodeId := "test-machine-set-node"

	// MachineSet

	machineSet := omnires.NewMachineSet(machineSetID)

	err := st.Create(ctx, machineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelCluster))

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, clusterID)

	assert.NoError(t, st.Create(ctx, machineSet))

	machineSet.Metadata().Labels().Delete(omnires.LabelCluster)

	err = st.Update(ctx, machineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelCluster))

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, clusterID)

	assert.NoError(t, st.Update(ctx, machineSet))

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, differentClusterID)

	err = st.Update(ctx, machineSet)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`changing value of label "%s" from "%s" to "%s"`, omnires.LabelCluster, clusterID, differentClusterID))

	// MachineSetNode

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, clusterID)

	machineSetNode := omnires.NewMachineSetNode(machineSetNodeId, machineSet)

	machineSetNode.Metadata().Labels().Delete(omnires.LabelCluster)
	machineSetNode.Metadata().Labels().Delete(omnires.LabelMachineSet)

	err = st.Create(ctx, machineSetNode)

	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelCluster))
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelMachineSet))

	machineSetNode.Metadata().Labels().Set(omnires.LabelCluster, clusterID)
	machineSetNode.Metadata().Labels().Set(omnires.LabelMachineSet, machineSetID)

	assert.NoError(t, st.Create(ctx, machineSetNode))

	machineSetNode.Metadata().Labels().Delete(omnires.LabelCluster)
	machineSetNode.Metadata().Labels().Delete(omnires.LabelMachineSet)

	err = st.Update(ctx, machineSetNode)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelCluster))
	assert.ErrorContains(t, err, fmt.Sprintf(`label "%s" does not exist`, omnires.LabelMachineSet))

	machineSetNode.Metadata().Labels().Set(omnires.LabelCluster, clusterID)
	machineSetNode.Metadata().Labels().Set(omnires.LabelMachineSet, machineSetID)

	assert.NoError(t, st.Update(ctx, machineSetNode))

	machineSetNode.Metadata().Labels().Set(omnires.LabelCluster, differentClusterID)
	machineSetNode.Metadata().Labels().Set(omnires.LabelMachineSet, differentMachineSetID)

	err = st.Update(ctx, machineSetNode)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, fmt.Sprintf(`changing value of label "%s" from "%s" to "%s"`, omnires.LabelCluster, clusterID, differentClusterID))
	assert.ErrorContains(t, err, fmt.Sprintf(`changing value of label "%s" from "%s" to "%s"`, omnires.LabelMachineSet, machineSetID, differentMachineSetID))
}

func TestMachineSetValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	etcdBackupStoreFactory, err := store.NewStoreFactory(config.EtcdBackup{})
	require.NoError(t, err)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.MachineSetValidationOptions(innerSt, etcdBackupStoreFactory)...)

	machineSet1 := omnires.NewMachineSet("test-cluster-wrong-suffix")

	// cluster doesn't exist

	err = st.Create(ctx, machineSet1)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "does not exist")

	require.NoError(t, st.Create(ctx, omnires.NewCluster("test-cluster")))

	machineSet1.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")

	// no role label

	err = st.Create(ctx, machineSet1)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "machine set must have either")

	require.NoError(t, st.Create(ctx, omnires.NewCluster("wrong-cluster")))
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

	status := omnires.NewClusterBootstrapStatus("test-cluster")

	status.TypedSpec().Value.Bootstrapped = true

	require.NoError(t, st.Create(ctx, status))

	machineSet2 := omnires.NewMachineSet("test-cluster-control-planes")

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

	machineSet3 := omnires.NewMachineSet("test-cluster-control-planes")

	machineSet3.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")
	machineSet3.Metadata().Labels().Set(omnires.LabelWorkerRole, "")

	err = st.Create(ctx, machineSet3)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "worker machine set must not have ID")

	// no cluster exists

	machineSet4 := omnires.NewMachineSet("test-cluster-tearing-down-control-planes")
	machineSet4.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster-tearing-down")
	machineSet4.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")

	// cluster exists and is tearing down

	require.NoError(t, st.Create(ctx, omnires.NewCluster("test-cluster-tearing-down")))
	_, err = state.WrapCore(st).Teardown(ctx, omnires.NewCluster("test-cluster-tearing-down").Metadata())
	require.NoError(t, err)

	err = st.Create(ctx, machineSet4)
	assert.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "tearing down")
}

func TestMachineSetBootstrapSpecValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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

	cluster := omnires.NewCluster(clusterID)

	require.NoError(t, st.Create(ctx, cluster))

	// worker machine set with bootstrap spec - not allowed

	workerMachineSet := omnires.NewMachineSet(omnires.WorkersResourceID(clusterID))

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

	controlPlaneMachineSet := omnires.NewMachineSet(omnires.ControlPlanesResourceID(clusterID))

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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.MachineSetNodeValidationOptions(state.WrapCore(innerSt))...)

	cluster := omnires.NewCluster("test-cluster")
	machineSet := omnires.NewMachineSet("test-machine-set")
	machineSet.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")
	machineSetNode := omnires.NewMachineSetNode("test-machine-set", machineSet)

	machineSetNode.Metadata().Annotations().Set(omnires.MachineLocked, "")

	assert := assert.New(t)

	require.NoError(t, st.Create(ctx, cluster))
	assert.NoError(st.Create(ctx, machineSet))
	assert.NoError(st.Create(ctx, machineSetNode))

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

	machineSetNode = omnires.NewMachineSetNode("test-machine-set-1", machineSet)
	assert.NoError(st.Create(ctx, machineSetNode))

	_, err = safe.StateUpdateWithConflicts(ctx, s, machineSetNode.Metadata(), func(n *omnires.MachineSetNode) error {
		n.Metadata().Annotations().Set(omnires.MachineLocked, "")

		return nil
	})

	assert.ErrorContains(err, "locking controlplanes is not allowed")

	machineSetNode = omnires.NewMachineSetNode("test-machine-set-2", machineSet)
	machineSetNode.Metadata().Annotations().Set(omnires.MachineLocked, "")
	assert.ErrorContains(st.Create(ctx, machineSetNode), "locking controlplanes is not allowed")
}

func TestClusterLockedAnnotation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	etcdBackupStoreFactory, err := store.NewStoreFactory(config.EtcdBackup{})
	etcdBackupConfig := config.EtcdBackup{
		TickInterval: pointer.To(time.Minute),
		MinInterval:  pointer.To(time.Hour),
		MaxInterval:  pointer.To(24 * time.Hour),
	}

	require.NoError(t, err)

	//nolint:prealloc
	var validationOptions []validated.StateOption

	validationOptions = append(validationOptions, omni.ClusterValidationOptions(state.WrapCore(innerSt), etcdBackupConfig, config.EmbeddedDiscoveryService{})...)
	validationOptions = append(validationOptions, omni.MachineSetNodeValidationOptions(state.WrapCore(innerSt))...)
	validationOptions = append(validationOptions, omni.MachineSetValidationOptions(state.WrapCore(innerSt), etcdBackupStoreFactory)...)

	st := validated.NewState(innerSt, validationOptions...)

	clusterID := "test-cluster"
	machineSetID := "test-cluster-control-planes"
	newMachineSetID := fmt.Sprintf("test-cluster-workers-%s", uuid.New())
	machineSetNodeID := fmt.Sprintf("machine-%s", uuid.New())
	newMachineSetNodeID := fmt.Sprintf("machine-%s", uuid.New())

	cluster := omnires.NewCluster(clusterID)
	cluster.TypedSpec().Value.KubernetesVersion = "1.32.7"
	cluster.TypedSpec().Value.TalosVersion = "1.10.6"
	clusterStatus := omnires.NewClusterStatus(clusterID)
	talosVersion := omnires.NewTalosVersion("1.10.6")
	talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.32.7", "1.32.8"}
	machineSet := omnires.NewMachineSet(machineSetID)
	machineSet.Metadata().Labels().Set(omnires.LabelCluster, clusterID)
	machineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")
	machineSetNode := omnires.NewMachineSetNode(machineSetNodeID, machineSet)

	assert := assert.New(t)

	assert.NoError(st.Create(ctx, talosVersion))
	assert.NoError(st.Create(ctx, cluster))
	assert.NoError(st.Create(ctx, machineSet))
	assert.NoError(st.Create(ctx, machineSetNode))
	assert.NoError(st.Create(ctx, clusterStatus))

	cluster.Metadata().Annotations().Set(omnires.ClusterLocked, "")
	assert.NoError(st.Update(ctx, cluster))

	newMachineSet := omnires.NewMachineSet(newMachineSetID)
	newMachineSet.Metadata().Labels().Set(omnires.LabelCluster, clusterID)
	newMachineSet.Metadata().Labels().Set(omnires.LabelWorkerRole, "")

	err = st.Create(ctx, newMachineSet)
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf(`adding machine set "%s" to the cluster "%s" is not allowed: the cluster is locked`, newMachineSetID, clusterID))

	err = st.Update(ctx, machineSet)
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf(`updating machine set "%s" on the cluster "%s" is not allowed: the cluster is locked`, machineSetID, clusterID))

	err = st.Destroy(ctx, machineSet.Metadata())
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf(`removing machine set "%s" from the cluster "%s" is not allowed: the cluster is locked`, machineSetID, clusterID))

	newMachineSetNode := omnires.NewMachineSetNode(newMachineSetNodeID, machineSet)
	newMachineSetNode.Metadata().Labels().Set(omnires.LabelCluster, clusterID)
	newMachineSetNode.Metadata().Labels().Set(omnires.LabelMachineSet, machineSetID)

	err = st.Create(ctx, newMachineSetNode)
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf(`adding machine set node to the machine set "%s" is not allowed: the cluster "%s" is locked`, machineSetID, clusterID))

	err = st.Update(ctx, machineSetNode)
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf(`updating machine set node on the machine set "%s" is not allowed: the cluster "%s" is locked`, machineSetID, clusterID))

	err = st.Destroy(ctx, machineSetNode.Metadata())
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf("removing machine set node from the machine set \"%s\" is not allowed: the cluster \"%s\" is locked", machineSetID, clusterID))

	err = st.Update(ctx, cluster)
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf(`updating cluster configuration is not allowed: the cluster "%s" is locked`, clusterID))

	err = st.Destroy(ctx, cluster.Metadata())
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf(`deletion is not allowed: the cluster "%s" is locked`, clusterID))

	cluster.Metadata().Annotations().Delete(omnires.ClusterLocked)
	assert.NoError(st.Update(ctx, cluster))

	cluster.TypedSpec().Value.KubernetesVersion = "1.32.8"

	assert.NoError(st.Update(ctx, cluster))
	assert.NoError(st.Create(ctx, newMachineSet))
	assert.NoError(st.Create(ctx, newMachineSetNode))
	assert.NoError(st.Update(ctx, machineSet))
	assert.NoError(st.Update(ctx, machineSetNode))
	assert.NoError(st.Destroy(ctx, machineSetNode.Metadata()))
	assert.NoError(st.Destroy(ctx, machineSet.Metadata()))
	assert.NoError(st.Destroy(ctx, cluster.Metadata()))
}

func TestClusterImport(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	etcdBackupStoreFactory, err := store.NewStoreFactory(config.EtcdBackup{})
	etcdBackupConfig := config.EtcdBackup{
		TickInterval: pointer.To(time.Minute),
		MinInterval:  pointer.To(time.Hour),
		MaxInterval:  pointer.To(24 * time.Hour),
	}

	require.NoError(t, err)

	//nolint:prealloc
	var validationOptions []validated.StateOption

	validationOptions = append(validationOptions, omni.ClusterValidationOptions(state.WrapCore(innerSt), etcdBackupConfig, config.EmbeddedDiscoveryService{})...)
	validationOptions = append(validationOptions, omni.MachineSetNodeValidationOptions(state.WrapCore(innerSt))...)
	validationOptions = append(validationOptions, omni.MachineSetValidationOptions(state.WrapCore(innerSt), etcdBackupStoreFactory)...)

	st := validated.NewState(innerSt, validationOptions...)

	clusterID := "test-cluster"
	machineSetID := "test-cluster-control-planes"
	machineSetNodeID := fmt.Sprintf("machine-%s", uuid.New())

	cluster := omnires.NewCluster(clusterID)
	cluster.Metadata().Annotations().Set(omnires.ClusterLocked, "")
	cluster.Metadata().Annotations().Set(omnires.ClusterImportIsInProgress, "")
	cluster.TypedSpec().Value.KubernetesVersion = "1.32.7"
	cluster.TypedSpec().Value.TalosVersion = "1.10.6"
	clusterStatus := omnires.NewClusterStatus(clusterID)
	clusterStatus.Metadata().Labels().Set(omnires.LabelClusterTaintedByImporting, "")

	talosVersion := omnires.NewTalosVersion("1.10.6")
	talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.32.7", "1.32.8"}
	machineSet := omnires.NewMachineSet(machineSetID)
	machineSet.Metadata().Labels().Set(omnires.LabelCluster, clusterID)
	machineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")
	machineSetNode := omnires.NewMachineSetNode(machineSetNodeID, machineSet)

	assert := assert.New(t)

	assert.NoError(st.Create(ctx, talosVersion))
	assert.NoError(st.Create(ctx, cluster))
	assert.NoError(st.Create(ctx, machineSet))
	assert.NoError(st.Create(ctx, machineSetNode))
	assert.NoError(st.Create(ctx, clusterStatus))

	assert.Error(st.Destroy(ctx, machineSetNode.Metadata()))

	err = st.Destroy(ctx, machineSet.Metadata())
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf(`removing machine set "%s" from the cluster "%s" is not allowed: the cluster is locked`, machineSetID, clusterID))

	err = st.Destroy(ctx, machineSetNode.Metadata())
	assert.Error(err)
	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, fmt.Sprintf("removing machine set node from the machine set \"%s\" is not allowed: the cluster \"%s\" is locked", machineSetID, clusterID))

	assert.NoError(st.Destroy(ctx, cluster.Metadata()))
}

func TestIdentitySAMLValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.IdentityValidationOptions(config.SAML{
		Enabled: pointer.To(true),
	})...)

	user := auth.NewIdentity("aaa@example.org")

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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.IdentityValidationOptions(config.SAML{})...)

	assert := assert.New(t)

	err := st.Create(ctx, auth.NewIdentity("aaA"))

	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, "must be lowercase")
	assert.ErrorContains(err, "not a valid email address")

	err = st.Create(ctx, auth.NewIdentity("aaa"))

	assert.True(validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(err, "not a valid email address")

	assert.NoError(st.Create(ctx, auth.NewIdentity("aaa@example.org")))
}

func TestExposedServiceAliasValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.ExposedServiceValidationOptions()...)

	exposedService := omnires.NewExposedService("test-exposed-service")

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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.ConfigPatchValidationOptions(innerSt)...)

	configPatch := omnires.NewConfigPatch("test-config-patch")

	patchDataNotAllowed := strings.TrimSpace(`
cluster:
  clusterName: test-cluster-name
`)

	err := configPatch.TypedSpec().Value.SetUncompressedData([]byte(patchDataNotAllowed))
	require.NoError(t, err)

	err = st.Create(ctx, configPatch)
	require.ErrorContains(t, err, "is not allowed in the config patch")

	patchDataAllowed := strings.TrimSpace(`
machine:
  env:
    bla: bla
`)

	err = configPatch.TypedSpec().Value.SetUncompressedData([]byte(patchDataAllowed))
	require.NoError(t, err)

	err = st.Create(ctx, configPatch)
	require.NoError(t, err)

	err = configPatch.TypedSpec().Value.SetUncompressedData([]byte(patchDataNotAllowed))
	require.NoError(t, err)

	err = st.Update(ctx, configPatch)
	require.ErrorContains(t, err, "is not allowed in the config patch")

	clusterID := "tearing-one"

	cluster := omnires.NewCluster(clusterID)

	require.NoError(t, st.Create(ctx, cluster))
	_, err = innerSt.Teardown(ctx, cluster.Metadata())
	require.NoError(t, err)

	configPatch = omnires.NewConfigPatch("another")
	configPatch.Metadata().Labels().Set(omnires.LabelCluster, clusterID)

	err = st.Create(ctx, configPatch)
	require.ErrorContains(t, err, "tearing down")

	msID := "machineset"

	ms := omnires.NewMachineSet(msID)

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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.SAMLLabelRuleValidationOptions()...)

	labelRule := auth.NewSAMLLabelRule("test-label-rule")
	labelRule.TypedSpec().Value.AssignRole = "invalid"
	labelRule.TypedSpec().Value.MatchLabels = []string{"--invalid--- ===== 5"}

	err := st.Create(ctx, labelRule)
	assert.ErrorContains(t, err, "unknown role")
	assert.ErrorContains(t, err, "invalid match labels")

	labelRule.TypedSpec().Value.AssignRole = string(role.Operator)

	err = st.Create(ctx, labelRule)
	assert.ErrorContains(t, err, "invalid match labels")

	labelRule.TypedSpec().Value.MatchLabels = []string{"a", "b"}

	err = st.Create(ctx, labelRule)
	assert.NoError(t, err)
}

func TestMachineSetClassesValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	etcdBackupStoreFactory, err := store.NewStoreFactory(config.EtcdBackup{})
	require.NoError(t, err)

	st := validated.NewState(innerSt,
		append(
			omni.MachineSetNodeValidationOptions(state.WrapCore(innerSt)),
			omni.MachineSetValidationOptions(state.WrapCore(innerSt), etcdBackupStoreFactory)...,
		)...,
	)

	require.NoError(t, st.Create(ctx, omnires.NewCluster("test-cluster")))

	machineSet := omnires.NewMachineSet("test-cluster-control-planes")

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")
	machineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")

	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{}

	err = st.Create(ctx, machineSet)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name: "none",
	}

	err = st.Create(ctx, machineSet)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	machineClass := omnires.NewMachineClass("test-class")

	require.NoError(t, st.Create(ctx, machineClass))

	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name: machineClass.Metadata().ID(),
	}

	require.NoError(t, st.Create(ctx, machineSet))

	machineSetNode := omnires.NewMachineSetNode("machine", machineSet)

	err = st.Create(ctx, machineSetNode)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	machineSet.TypedSpec().Value.MachineAllocation.Name = "none"

	err = st.Update(ctx, machineSet)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// expect no error, as the machine mgmt mode change is allowed if there are no nodes in the machine set
	machineSet.TypedSpec().Value.MachineAllocation = nil

	require.NoError(t, st.Update(ctx, machineSet))

	// add a node
	require.NoError(t, st.Create(ctx, machineSetNode))

	// machine set mgmt mode change is not allowed anymore
	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name: machineClass.Metadata().ID(),
	}

	err = st.Update(ctx, machineSet)
	require.True(t, validated.IsValidationError(err), "expected validation error")
	require.ErrorContains(t, err, "machine set is not empty")
}

func TestMachineClassValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	st := validated.NewState(innerSt,
		omni.MachineClassValidationOptions(state.WrapCore(innerSt))...,
	)

	require.NoError(t, st.Create(ctx, omnires.NewCluster("test-cluster")))

	machineSet := omnires.NewMachineSet("test-cluster-control-planes")

	machineSet.Metadata().Labels().Set(omnires.LabelCluster, "test-cluster")
	machineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")

	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name: "test-class",
	}

	machineClass := omnires.NewMachineClass("test-class")

	err := st.Create(ctx, machineClass)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	machineClass.TypedSpec().Value.MatchLabels = []string{"a"}

	require.NoError(t, st.Create(ctx, machineClass))
	require.NoError(t, st.Create(ctx, machineSet))

	err = st.Destroy(ctx, machineClass.Metadata())

	require.True(t, validated.IsValidationError(err), "expected validation error")

	require.NoError(t, st.Destroy(ctx, machineSet.Metadata()))
	require.NoError(t, st.Destroy(ctx, machineClass.Metadata()))

	// invalid selectors

	machineClass.TypedSpec().Value.MatchLabels = []string{"abcd + a"}

	err = st.Create(ctx, machineClass)

	require.Error(t, err)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// both modes set

	machineClass.TypedSpec().Value.MatchLabels = []string{"abcd"}
	machineClass.TypedSpec().Value.AutoProvision = &specs.MachineClassSpec_Provision{}

	err = st.Create(ctx, machineClass)

	require.Error(t, err)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// auto provision

	talosVersion := omnires.NewTalosVersion("1.8.0")
	talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.30.0", "1.30.1"}

	providerStatus := infra.NewProviderStatus("exists")
	providerStatus.TypedSpec().Value.Schema = string(schema)

	staticProvider := infra.NewProviderStatus("static")
	staticProvider.Metadata().Labels().Set(omnires.LabelIsStaticInfraProvider, "")

	require.NoError(t, st.Create(ctx, talosVersion))
	require.NoError(t, st.Create(ctx, providerStatus))
	require.NoError(t, st.Create(ctx, staticProvider))

	// no provider id

	machineClass.TypedSpec().Value.MatchLabels = nil
	machineClass.TypedSpec().Value.AutoProvision = &specs.MachineClassSpec_Provision{
		ProviderId: "",
	}

	err = st.Create(ctx, machineClass)

	require.Error(t, err)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// no provider registered

	machineClass.TypedSpec().Value.AutoProvision.ProviderId = "none"

	err = st.Create(ctx, machineClass)

	require.Error(t, err)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// no talos version

	machineClass.TypedSpec().Value.AutoProvision.ProviderId = providerStatus.Metadata().ID()

	err = st.Create(ctx, machineClass)

	require.Error(t, err)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// invalid provider data

	machineClass.TypedSpec().Value.AutoProvision.ProviderData = `mem: .nan`

	err = st.Create(ctx, machineClass)

	require.Error(t, err)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	machineClass.TypedSpec().Value.AutoProvision.ProviderData = `
disk: 1TB
`

	err = st.Create(ctx, machineClass)

	require.Error(t, err)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// static infra provider usage is not allowed

	machineClass.TypedSpec().Value.AutoProvision.ProviderId = staticProvider.Metadata().ID()

	err = st.Create(ctx, machineClass)

	require.Error(t, err)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	// valid

	machineClass.TypedSpec().Value.AutoProvision.ProviderId = providerStatus.Metadata().ID()

	machineClass.TypedSpec().Value.AutoProvision.ProviderData = `
size: t2.small
`

	err = st.Create(ctx, machineClass)

	require.NoError(t, err)
}

func TestS3ConfigValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
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

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	st := validated.NewState(innerSt, omni.SchematicConfigurationValidationOptions()...)

	res := omnires.NewSchematicConfiguration("test")

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

func TestMachineRequestSetValidation(t *testing.T) {
	t.Parallel()

	talosVersion1 := omnires.NewTalosVersion("1.2.0")
	talosVersion1.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.27.0", "1.27.1"}
	talosVersion1.TypedSpec().Value.Deprecated = true

	talosVersion2 := omnires.NewTalosVersion("1.7.5")
	talosVersion2.TypedSpec().Value.CompatibleKubernetesVersions = []string{"1.30.0"}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, innerSt.Create(ctx, talosVersion1))
	require.NoError(t, innerSt.Create(ctx, talosVersion2))

	providerStatus := infra.NewProviderStatus("talemu")
	providerStatus.TypedSpec().Value.Schema = string(schema)

	require.NoError(t, innerSt.Create(ctx, providerStatus))

	st := validated.NewState(innerSt, omni.MachineRequestSetValidationOptions(innerSt)...)

	res := omnires.NewMachineRequestSet("test")

	err := st.Create(ctx, res)

	require.True(t, validated.IsValidationError(err), "expected validation error")

	res.TypedSpec().Value.ProviderId = "talemu"
	res.TypedSpec().Value.TalosVersion = "1234"
	res.TypedSpec().Value.ProviderData = `
size: t2.small
`

	err = st.Create(ctx, res)

	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "invalid talos version")

	res.TypedSpec().Value.TalosVersion = "1.2.0"

	err = st.Create(ctx, res)

	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "is no longer supported")

	res.TypedSpec().Value.TalosVersion = "1.7.5"

	err = st.Create(ctx, res)

	require.NoError(t, err)
}

func TestInfraMachineConfigValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.InfraMachineConfigValidationOptions(innerSt)...)
	wrappedState := state.WrapCore(st)

	conf := omnires.NewInfraMachineConfig("test")
	link := siderolink.NewLink("test", nil)

	require.NoError(t, st.Create(ctx, conf))
	require.NoError(t, st.Create(ctx, link))

	// can delete the conf, as it is not accepted
	require.NoError(t, st.Destroy(ctx, conf.Metadata()))

	// recreate the conf, as accepted this time
	conf = omnires.NewInfraMachineConfig("test")
	conf.TypedSpec().Value.AcceptanceStatus = specs.InfraMachineConfigSpec_ACCEPTED

	require.NoError(t, st.Create(ctx, conf))

	// try to "unaccept" it

	_, err := safe.StateUpdateWithConflicts(ctx, wrappedState, conf.Metadata(), func(res *omnires.InfraMachineConfig) error {
		res.TypedSpec().Value.AcceptanceStatus = specs.InfraMachineConfigSpec_REJECTED

		return nil
	})
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "an accepted machine cannot be rejected")

	// try to destroy it

	err = st.Destroy(ctx, conf.Metadata())
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "cannot delete the config for an already accepted machine config while it is linked to a machine")

	// destroy the link
	require.NoError(t, st.Destroy(ctx, link.Metadata()))

	// now it can be destroyed
	require.NoError(t, st.Destroy(ctx, conf.Metadata()))
}

func TestNodeForceDestroyRequestValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	st := validated.NewState(innerSt, omni.NodeForceDestroyRequestValidationOptions(innerSt)...)

	req := omnires.NewNodeForceDestroyRequest("test")

	err := st.Create(ctx, req)
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, `cannot create/update a NodeForceDestroyRequest for node "test", as there is no matching cluster machine`)

	require.NoError(t, st.Create(ctx, omnires.NewClusterMachine("test"))) // create the matching cluster machine
	require.NoError(t, st.Create(ctx, req))                               // assert that we can create the destroy request now
}

func TestJoinTokenValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))

	st := validated.NewState(innerSt, omni.JoinTokenValidationOptions(innerSt)...)
	wrappedState := state.WrapCore(st)

	normalJoinToken := siderolink.NewJoinToken("1234567812345678")
	normalJoinToken.TypedSpec().Value.Name = "hello"

	assert.NoError(t, wrappedState.Create(ctx, normalJoinToken))

	_, err := safe.StateUpdateWithConflicts(ctx, wrappedState, normalJoinToken.Metadata(), func(token *siderolink.JoinToken) error {
		token.TypedSpec().Value.Name = ""

		return nil
	})

	assert.ErrorContains(t, err, "empty")

	_, err = safe.StateUpdateWithConflicts(ctx, wrappedState, normalJoinToken.Metadata(), func(token *siderolink.JoinToken) error {
		token.TypedSpec().Value.Name = "mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm"

		return nil
	})

	assert.ErrorContains(t, err, "long")

	_, err = safe.StateUpdateWithConflicts(ctx, wrappedState, normalJoinToken.Metadata(), func(token *siderolink.JoinToken) error {
		token.TypedSpec().Value.Name = "hi"

		return nil
	})

	assert.NoError(t, err)

	defaultToken := siderolink.NewDefaultJoinToken()

	defaultToken.TypedSpec().Value.TokenId = normalJoinToken.Metadata().ID()

	require.NoError(t, wrappedState.Create(ctx, defaultToken))

	assert.ErrorContains(t, wrappedState.Destroy(ctx, normalJoinToken.Metadata()), "not possible")

	require.NoError(t, wrappedState.Destroy(ctx, defaultToken.Metadata()))

	assert.NoError(t, wrappedState.Destroy(ctx, normalJoinToken.Metadata()))
}

func TestDefaultJoinTokenValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.DefaultJoinTokenValidationOptions(innerSt)...)
	wrappedState := state.WrapCore(st)

	defaultToken := siderolink.NewDefaultJoinToken()

	joinToken := siderolink.NewJoinToken("mm")

	require.NoError(t, st.Create(ctx, joinToken))

	joinToken = siderolink.NewJoinToken("mmmm")

	require.NoError(t, st.Create(ctx, joinToken))

	defaultToken.TypedSpec().Value.TokenId = "mm"

	require.NoError(t, wrappedState.Create(ctx, defaultToken))

	_, err := safe.StateUpdateWithConflicts(ctx, wrappedState, defaultToken.Metadata(), func(token *siderolink.DefaultJoinToken) error {
		token.TypedSpec().Value.TokenId = "mmmm"

		return nil
	})

	assert.NoError(t, err)

	_, err = safe.StateUpdateWithConflicts(ctx, wrappedState, defaultToken.Metadata(), func(token *siderolink.DefaultJoinToken) error {
		token.TypedSpec().Value.TokenId = "mmmmmm"

		return nil
	})

	assert.Error(t, err)

	_, err = wrappedState.Teardown(ctx, defaultToken.Metadata())

	assert.ErrorContains(t, err, "destroying")

	err = wrappedState.Destroy(ctx, defaultToken.Metadata())

	assert.ErrorContains(t, err, "destroying")
}

var (
	//go:embed controllers/omni/secrets/testdata/secrets-valid.yaml
	validSecrets string

	//go:embed controllers/omni/secrets/testdata/secrets-broken.yaml
	brokenSecrets string

	//go:embed controllers/omni/secrets/testdata/secrets-invalid.yaml
	invalidSecrets string
)

func TestImportedClusterSecretValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.ImportedClusterSecretValidationOptions(innerSt, true)...)
	res := omnires.NewImportedClusterSecrets("test")

	res.TypedSpec().Value.Data = brokenSecrets
	require.True(t, validated.IsValidationError(st.Create(ctx, res)), "expected validation error")

	res.TypedSpec().Value.Data = validSecrets
	require.NoError(t, st.Create(ctx, res))
	require.NoError(t, st.Update(ctx, res))

	res.TypedSpec().Value.Data = brokenSecrets
	require.True(t, validated.IsValidationError(st.Update(ctx, res)), "expected validation error")
	require.NoError(t, st.Destroy(ctx, res.Metadata()))

	res.TypedSpec().Value.Data = invalidSecrets
	err := st.Create(ctx, res)
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "cluster.secret is required")
	assert.ErrorContains(t, err, "one of [secrets.secretboxencryptionsecret, secrets.aescbcencryptionsecret] is required")
	assert.ErrorContains(t, err, "trustdinfo is required")
	assert.ErrorContains(t, err, "certs.etcd is invalid")
	assert.ErrorContains(t, err, "certs.k8saggregator is required")
	assert.ErrorContains(t, err, "certs.os is invalid")

	cluster := omnires.NewCluster("test")
	require.NoError(t, st.Create(ctx, cluster))

	res.TypedSpec().Value.Data = validSecrets
	err = st.Create(ctx, res)
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "cannot create/update an ImportedClusterSecrets, as there is already an existing cluster with name")
}

func TestInfraProviderValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.InfraProviderValidationOptions(innerSt)...)

	provider := infra.NewProvider("test")
	require.NoError(t, st.Create(ctx, provider))

	machine := omnires.NewMachine("machine")
	machine.Metadata().Labels().Set(omnires.LabelIsManagedByStaticInfraProvider, "")
	machine.Metadata().Labels().Set(omnires.LabelInfraProviderID, provider.Metadata().ID())
	require.NoError(t, st.Create(ctx, machine))

	machine2 := omnires.NewMachine("machine2")
	machine2.Metadata().Labels().Set(omnires.LabelIsManagedByStaticInfraProvider, "")
	machine2.Metadata().Labels().Set(omnires.LabelInfraProviderID, "different-provider")
	require.NoError(t, st.Create(ctx, machine2))

	machine3 := omnires.NewMachine("machine3")
	machine3.Metadata().Labels().Set(omnires.LabelInfraProviderID, "non-static-provider")
	require.NoError(t, st.Create(ctx, machine3))

	machine4 := omnires.NewMachine("machine4")
	require.NoError(t, st.Create(ctx, machine4))

	err := st.Destroy(ctx, provider.Metadata())
	require.True(t, validated.IsValidationError(err), "expected validation error")
	require.ErrorContains(t, err, "cannot delete the infra provider")

	require.NoError(t, st.Destroy(ctx, machine.Metadata()))
	require.NoError(t, st.Destroy(ctx, provider.Metadata()))
}

func TestRotateSecretsValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	innerSt := state.WrapCore(namespaced.NewState(inmem.Build))
	st := validated.NewState(innerSt, omni.RotateSecretsValidationOptions(innerSt)...)

	updateTalosCATimestamp := func(rotateTalosCA *omnires.RotateTalosCA) {
		rotateTalosCA.Metadata().Annotations().Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	}

	rotationStatus := omnires.NewClusterSecretsRotationStatus("test")
	require.NoError(t, st.Create(ctx, rotationStatus))

	rotateTalosCA := omnires.NewRotateTalosCA("test")
	require.NoError(t, st.Create(ctx, rotateTalosCA))

	updateTalosCATimestamp(rotateTalosCA)
	require.NoError(t, st.Update(ctx, rotateTalosCA))

	rotationStatus.TypedSpec().Value.Phase = specs.SecretRotationSpec_PRE_ROTATE
	rotationStatus.TypedSpec().Value.Component = specs.SecretRotationSpec_TALOS_CA
	require.NoError(t, st.Update(ctx, rotationStatus))

	updateTalosCATimestamp(rotateTalosCA)
	err := st.Update(ctx, rotateTalosCA)
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "cannot modify the RotateTalosCAs.omni.sidero.dev \"test\" while a secret rotation is in progress")

	err = st.Destroy(ctx, rotateTalosCA.Metadata())
	require.True(t, validated.IsValidationError(err), "expected validation error")
	assert.ErrorContains(t, err, "cannot delete the RotateTalosCAs.omni.sidero.dev \"test\" while a secret rotation is in progress")

	rotationStatus.TypedSpec().Value.Phase = specs.SecretRotationSpec_OK
	rotationStatus.TypedSpec().Value.Component = specs.SecretRotationSpec_NONE
	require.NoError(t, st.Update(ctx, rotationStatus))

	require.NoError(t, st.Destroy(ctx, rotateTalosCA.Metadata()))
}

func TestInstallationMediaConfigValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	st := validated.NewState(state.WrapCore(namespaced.NewState(inmem.Build)), omni.InstallationMediaConfigValidationOptions()...)

	installationMediaConfig := omnires.NewInstallationMediaConfig("test")

	err := st.Create(ctx, installationMediaConfig)
	require.Contains(t, err.Error(), "invalid installation media config: talos version is required")

	installationMediaConfig.TypedSpec().Value.TalosVersion = constants.DefaultTalosVersion
	err = st.Create(ctx, installationMediaConfig)
	require.Contains(t, err.Error(), "invalid installation media config: architecture is required")

	installationMediaConfig.TypedSpec().Value.Architecture = specs.PlatformConfigSpec_AMD64
	err = st.Create(ctx, installationMediaConfig)
	require.Contains(t, err.Error(), "invalid installation media config: join token is required")

	installationMediaConfig.TypedSpec().Value.JoinToken = "test-token"
	installationMediaConfig.TypedSpec().Value.Cloud = &specs.InstallationMediaConfigSpec_Cloud{}
	installationMediaConfig.TypedSpec().Value.Sbc = &specs.InstallationMediaConfigSpec_SBC{}

	err = st.Create(ctx, installationMediaConfig)
	require.Contains(t, err.Error(), "invalid installation media config: both sbc and cloud fields are set")

	installationMediaConfig.TypedSpec().Value.Sbc = nil
	err = st.Create(ctx, installationMediaConfig)
	require.NoError(t, err)

	modifiedInstallationMediaConfig, ok := installationMediaConfig.DeepCopy().(*omnires.InstallationMediaConfig)
	require.True(t, ok)

	modifiedInstallationMediaConfig.TypedSpec().Value.TalosVersion = ""
	err = st.Update(ctx, modifiedInstallationMediaConfig)
	require.Contains(t, err.Error(), "invalid installation media config: talos version is required")

	modifiedInstallationMediaConfig, ok = installationMediaConfig.DeepCopy().(*omnires.InstallationMediaConfig)
	require.True(t, ok)

	modifiedInstallationMediaConfig.TypedSpec().Value.Architecture = specs.PlatformConfigSpec_UNKNOWN_ARCH
	err = st.Update(ctx, modifiedInstallationMediaConfig)
	require.Contains(t, err.Error(), "invalid installation media config: architecture is required")

	modifiedInstallationMediaConfig, ok = installationMediaConfig.DeepCopy().(*omnires.InstallationMediaConfig)
	require.True(t, ok)

	modifiedInstallationMediaConfig.TypedSpec().Value.JoinToken = ""
	err = st.Update(ctx, modifiedInstallationMediaConfig)
	require.Contains(t, err.Error(), "invalid installation media config: join token is required")

	modifiedInstallationMediaConfig, ok = installationMediaConfig.DeepCopy().(*omnires.InstallationMediaConfig)
	require.True(t, ok)

	modifiedInstallationMediaConfig.TypedSpec().Value.Sbc = &specs.InstallationMediaConfigSpec_SBC{}
	err = st.Update(ctx, modifiedInstallationMediaConfig)
	require.Contains(t, err.Error(), "invalid installation media config: both sbc and cloud fields are set")

	installationMediaConfig.TypedSpec().Value.Cloud.Platform = "AWS"
	require.NoError(t, st.Update(ctx, installationMediaConfig))

	installationMediaConfig.Metadata().SetPhase(resource.PhaseTearingDown)
	installationMediaConfig.TypedSpec().Value.TalosVersion = ""
	installationMediaConfig.TypedSpec().Value.Architecture = specs.PlatformConfigSpec_UNKNOWN_ARCH
	installationMediaConfig.TypedSpec().Value.JoinToken = ""
	installationMediaConfig.TypedSpec().Value.Cloud = &specs.InstallationMediaConfigSpec_Cloud{}
	installationMediaConfig.TypedSpec().Value.Sbc = &specs.InstallationMediaConfigSpec_SBC{}
	require.NoError(t, st.Update(ctx, installationMediaConfig))

	err = st.Destroy(ctx, installationMediaConfig.Metadata())
	require.NoError(t, err)
}

type mockEtcdBackupStoreFactory struct {
	store etcdbackup.Store
}

func (m *mockEtcdBackupStoreFactory) SetThroughputs(uint64, uint64) {}

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

func (m *mockEtcdBackupStore) ListBackups(context.Context, string) (iter.Seq2[etcdbackup.Info, error], error) {
	return xiter.Empty2, nil
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
