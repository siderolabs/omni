// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/migration"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/auth/scope"
	"github.com/siderolabs/omni/internal/pkg/config"
)

type MigrationSuite struct {
	suite.Suite
	state   state.State
	manager *migration.Manager
	logger  *zap.Logger
}

type machine struct {
	labels map[string]string
	name   resource.ID
	patch  string
}

const (
	testConfigPatch = `machine:
  network:
    hostname: debug`
	testInstallDisk = "/dev/vda"
)

func (suite *MigrationSuite) assertLabel(res resource.Resource, key, value string) {
	label, ok := res.Metadata().Labels().Get(key)
	suite.Require().Truef(ok, "the label %s is not set on %s", key, res.Metadata())
	suite.Require().Equal(value, label)
}

func (suite *MigrationSuite) SetupTest() {
	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	suite.logger = zaptest.NewLogger(suite.T())

	suite.manager = migration.NewManager(suite.state, suite.logger)
}

func (suite *MigrationSuite) TestClusterInfo() {
	ctx := suite.T().Context()

	cluster, machine := suite.createCluster(ctx, "c1")

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("clusterInfo", "clusterInstallImageToTalosVersion"))))

	var err error

	cluster, err = safe.StateGet[*omni.Cluster](ctx, suite.state, cluster.Metadata())
	suite.Require().NoError(err)

	suite.Require().Equal(
		machine.TypedSpec().Value.InstallImage,
		constants.TalosRegistry+":v"+cluster.TypedSpec().Value.TalosVersion,
	)

	suite.Require().Equal(
		machine.TypedSpec().Value.KubernetesVersion,
		cluster.TypedSpec().Value.KubernetesVersion,
	)

	cluster, _ = suite.createCluster(ctx, "c2")

	// This shouldn't happen: create old version of the cluster again and see it not being updated
	// as the DB version is already up-to-date.
	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("clusterInfo", "clusterInstallImageToTalosVersion"))))

	cluster, err = safe.StateGet[*omni.Cluster](ctx, suite.state, cluster.Metadata())

	suite.Require().NoError(err)

	suite.Require().Equal(
		"",
		cluster.TypedSpec().Value.InstallImage, //nolint:staticcheck
	)

	suite.Require().Equal(
		"",
		cluster.TypedSpec().Value.KubernetesVersion,
	)
}

func (suite *MigrationSuite) TestConfigPatches() {
	ctx := suite.T().Context()

	_, machine := suite.createCluster(ctx, "c1")

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(15)))

	list, err := safe.StateListAll[*omni.ConfigPatch](
		ctx,
		suite.state,
		state.WithLabelQuery(resource.LabelEqual("cluster", "c1")),
	)

	suite.Require().Greater(list.Len(), 1)

	diskPatch := list.Get(0)
	userPatch := list.Get(1)

	suite.Require().NoError(err)
	suite.assertLabel(diskPatch, "cluster", "c1")
	suite.assertLabel(diskPatch, "cluster-machine", machine.Metadata().ID())

	config := v1alpha1.Config{}

	buffer, err := diskPatch.TypedSpec().Value.GetUncompressedData()
	suite.Require().NoError(err)

	defer buffer.Free()

	patchData := buffer.Data()

	suite.Require().NoError(yaml.Unmarshal(patchData, &config))

	suite.Require().Equal(testInstallDisk, config.MachineConfig.MachineInstall.InstallDisk)

	suite.Require().NoError(err)
	suite.assertLabel(userPatch, "cluster", "c1")
	suite.assertLabel(userPatch, "cluster-machine", machine.Metadata().ID())

	buffer, err = userPatch.TypedSpec().Value.GetUncompressedData()
	suite.Require().NoError(err)

	defer buffer.Free()

	patchData = buffer.Data()

	suite.Require().Equal(
		testConfigPatch,
		string(patchData),
	)
}

func (suite *MigrationSuite) Test_changePublicKeyOwner() {
	ctx, cancel := context.WithCancel(suite.T().Context())
	defer cancel()

	keys := []*authres.PublicKey{
		authres.NewPublicKey(resources.DefaultNamespace, "test1"),
		authres.NewPublicKey(resources.DefaultNamespace, "test2"),
		authres.NewPublicKey(resources.DefaultNamespace, "test3"),
	}

	for _, key := range keys[:2] {
		suite.Require().NoError(suite.state.Create(ctx, key))
	}

	for _, key := range keys[2:] {
		suite.Require().NoError(suite.state.Create(
			ctx,
			key,
			state.WithCreateOwner(pointer.To(omnictrl.KeyPrunerController{}).Name())),
		)
	}

	// test migration in isolation
	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("changePublicKeyOwner"))))

	keyVerifier := func(key *authres.PublicKey, expectedVersion int) {
		result, err := safe.StateGet[*authres.PublicKey](ctx, suite.state, key.Metadata())
		suite.Require().NoError(err)
		suite.Require().Equal(pointer.To(omnictrl.KeyPrunerController{}).Name(), result.Metadata().Owner())
		suite.Require().EqualValues(result.Metadata().Version().Value(), expectedVersion)
	}

	for _, key := range keys[:2] {
		keyVerifier(key, 2)
	}

	for _, key := range keys[2:] {
		keyVerifier(key, 1)
	}
}

func (suite *MigrationSuite) TestMachineSets() {
	ctx := suite.T().Context()

	cluster := suite.createClusterWithMachines(ctx, "c1", []machine{
		{
			name: "m1",
			labels: map[string]string{
				"role-controlplane": "",
			},
		},
		{
			name: "m2",
			labels: map[string]string{
				"role-controlplane": "",
			},
		},
		{
			name: "m3",
			labels: map[string]string{
				"role-controlplane": "",
			},
		},
		{
			name: "m4",
			labels: map[string]string{
				"role-worker": "",
			},
		},
	}, true)

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(15)))

	var err error

	controlPlane, err := safe.StateGet[*omni.MachineSet](ctx, suite.state, resource.NewMetadata(
		resources.DefaultNamespace,
		omni.MachineSetType,
		omni.ControlPlanesResourceID(cluster.Metadata().ID()),
		resource.VersionUndefined,
	))

	suite.Require().NoError(err)

	var machines []string

	{
		var list safe.List[*omni.MachineSetNode]

		list, err = safe.StateListAll[*omni.MachineSetNode](
			ctx,
			suite.state,
			state.WithLabelQuery(
				resource.LabelEqual(omni.LabelMachineSet, controlPlane.Metadata().ID()),
			),
		)

		suite.Require().NoError(err)

		for val := range list.All() {
			machines = append(machines, val.Metadata().ID())
		}
	}

	suite.Require().Equal(
		[]string{"c1.m1", "c1.m2", "c1.m3"},
		machines,
	)

	machineSet, err := safe.StateGet[*omni.MachineSet](ctx, suite.state, resource.NewMetadata(
		resources.DefaultNamespace,
		omni.MachineSetType,
		omni.WorkersResourceID(cluster.Metadata().ID()),
		resource.VersionUndefined,
	))

	suite.Require().NoError(err)

	{
		list, err := safe.StateListAll[*omni.MachineSetNode](
			ctx,
			suite.state,
			state.WithLabelQuery(
				resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()),
			),
		)

		suite.Require().NoError(err)

		machines = make([]string, 0, list.Len())

		for val := range list.All() {
			machines = append(machines, val.Metadata().ID())
		}
	}

	suite.Require().Equal(
		[]string{"c1.m4"},
		machines,
	)

	for i, m := range []string{"c1.m1", "c1.m2", "c1.m3", "c1.m4"} {
		machine, err := safe.StateGet[*omni.ClusterMachine](ctx, suite.state, omni.NewClusterMachine(resources.DefaultNamespace, m).Metadata())
		suite.Require().NoError(err)
		suite.assertLabel(machine, "cluster", "c1")

		if i < 3 {
			suite.assertLabel(machine, "role-controlplane", "")
			suite.assertLabel(machine, "machine-set", omni.ControlPlanesResourceID("c1"))
		} else {
			suite.assertLabel(machine, "role-worker", "")
			suite.assertLabel(machine, "machine-set", omni.WorkersResourceID("c1"))
		}
	}
}

func (suite *MigrationSuite) TestClusterInfoTearingDown() {
	ctx := suite.T().Context()

	cluster, _ := suite.createCluster(ctx, "c1", "finzlier")

	_, err := suite.state.Teardown(ctx, cluster.Metadata())
	suite.Require().NoError(err)

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(15)))
}

func (suite *MigrationSuite) createCluster(ctx context.Context, name string, finalizers ...string) (*omni.Cluster, *omni.ClusterMachineTemplate) {
	cluster := omni.NewCluster(resources.DefaultNamespace, name)
	machine := omni.NewClusterMachineTemplate(resources.DefaultNamespace, fmt.Sprintf("%s.uuid", name))
	machine.TypedSpec().Value.Patch = testConfigPatch
	machine.TypedSpec().Value.InstallDisk = testInstallDisk

	for _, finalizer := range finalizers {
		cluster.Metadata().Finalizers().Add(finalizer)
	}

	machine.TypedSpec().Value.InstallImage = "ghcr.io/siderolabs/installer:v1.1.3"
	machine.TypedSpec().Value.KubernetesVersion = "1.24.1"

	machine.Metadata().Labels().Set("cluster", cluster.Metadata().ID())
	machine.Metadata().Labels().Set("role-controlplane", "")

	machine.Metadata().Finalizers().Add("finalizer")

	suite.Require().NoError(suite.state.Create(ctx, cluster))
	suite.Require().NoError(suite.state.Create(ctx, machine))

	return cluster, machine
}

func (suite *MigrationSuite) createClusterWithMachines(ctx context.Context, name string, machines []machine, withTemplates bool) *omni.Cluster {
	cluster := omni.NewCluster(resources.DefaultNamespace, name)

	for _, m := range machines {
		id := fmt.Sprintf("%s.%s", name, m.name)
		machine := omni.NewClusterMachine(resources.DefaultNamespace, id)
		machine.TypedSpec().Value.KubernetesVersion = "v1.24.0"

		machine.Metadata().Labels().Set("cluster", cluster.Metadata().ID())
		machine.Metadata().Finalizers().Add(omnictrl.NewClusterMachineConfigController("factory-test.talos.dev", nil, 8090).Name())

		if withTemplates {
			template := omni.NewClusterMachineTemplate(resources.DefaultNamespace, fmt.Sprintf("%s.%s", name, m.name))
			template.TypedSpec().Value.Patch = m.patch
			template.TypedSpec().Value.InstallDisk = testInstallDisk
			template.TypedSpec().Value.InstallImage = "ghcr.io/siderolabs/installer:v1.1.1"
			template.TypedSpec().Value.KubernetesVersion = "1.24.1"
			template.Metadata().Labels().Set("cluster", cluster.Metadata().ID())

			for label, value := range m.labels {
				template.Metadata().Labels().Set(label, value)
			}

			suite.Require().NoError(suite.state.Create(ctx, template))
		} else {
			cluster.TypedSpec().Value.InstallImage = "ghcr.io/siderolabs/installer:v1.1.1" //nolint:staticcheck
		}

		for label, value := range m.labels {
			machine.Metadata().Labels().Set(label, value)
		}

		suite.Require().NoError(suite.state.Create(ctx, machine))
	}

	suite.Require().NoError(suite.state.Create(ctx, cluster))

	return cluster
}

func (suite *MigrationSuite) TestUserDefaultScopes() {
	var err error

	ctx := suite.T().Context()

	user1 := authres.NewUser(resources.DefaultNamespace, "user1")

	user2 := authres.NewUser(resources.DefaultNamespace, "user2")

	user3 := authres.NewUser(resources.DefaultNamespace, "user3")
	user3.TypedSpec().Value.Scopes = []string{"scopeExisting1", "scopeExisting2"} //nolint:staticcheck

	suite.Require().NoError(suite.state.Create(ctx, user1))
	suite.Require().NoError(suite.state.Create(ctx, user2))
	suite.Require().NoError(suite.state.Create(ctx, user3))

	// test migration in isolation
	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("addDefaultScopesToUsers"))))

	user1, err = safe.StateGet[*authres.User](ctx, suite.state, user1.Metadata())
	suite.Require().NoError(err)

	user2, err = safe.StateGet[*authres.User](ctx, suite.state, user2.Metadata())
	suite.Require().NoError(err)

	user3, err = safe.StateGet[*authres.User](ctx, suite.state, user3.Metadata())
	suite.Require().NoError(err)

	assert.Equal(suite.T(), scope.NewScopes(scope.UserDefaultScopes...).Strings(), user1.TypedSpec().Value.GetScopes())
	assert.Equal(suite.T(), scope.NewScopes(scope.UserDefaultScopes...).Strings(), user2.TypedSpec().Value.GetScopes())
	assert.Equal(suite.T(), []string{"scopeExisting1", "scopeExisting2"}, user3.TypedSpec().Value.GetScopes())
}

func (suite *MigrationSuite) TestRollingStrategyOnControlPlaneMachineSets() {
	var err error

	ctx := suite.T().Context()

	controlPlaneMachineSet := omni.NewMachineSet(resources.DefaultNamespace, "control-plane-set")
	controlPlaneMachineSet.Metadata().Labels().Set("role-controlplane", "")

	workerMachineSet := omni.NewMachineSet(resources.DefaultNamespace, "worker-set")
	workerMachineSet.Metadata().Labels().Set("role-worker", "")

	suite.Require().NoError(suite.state.Create(ctx, controlPlaneMachineSet))
	suite.Require().NoError(suite.state.Create(ctx, workerMachineSet))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(15)))

	controlPlaneMachineSet, err = safe.StateGet[*omni.MachineSet](ctx, suite.state, controlPlaneMachineSet.Metadata())
	suite.Require().NoError(err)

	workerMachineSet, err = safe.StateGet[*omni.MachineSet](ctx, suite.state, workerMachineSet.Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(specs.MachineSetSpec_Rolling, controlPlaneMachineSet.TypedSpec().Value.GetUpdateStrategy())
	suite.Assert().Equal(specs.MachineSetSpec_Unset, workerMachineSet.TypedSpec().Value.GetUpdateStrategy())
}

func (suite *MigrationSuite) TestUpdateConfigPatchLabels() {
	var err error

	ctx := suite.T().Context()

	cluster := omni.NewCluster(resources.DefaultNamespace, "cluster")
	cluster.TypedSpec().Value.InstallImage = fmt.Sprintf("%s:v%s", config.Config.TalosRegistry, constants.DefaultTalosVersion) //nolint:staticcheck

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "machine-set")
	machineSet.Metadata().Labels().Set("cluster", cluster.Metadata().ID())

	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "cluster-machine")
	clusterMachine.Metadata().Labels().Set("cluster", cluster.Metadata().ID())

	clusterConfigPatch := omni.NewConfigPatch(
		resources.DefaultNamespace,
		"config-patch-1",
		pair.MakePair("cluster-name", cluster.Metadata().ID()),
	)

	machineSetConfigPatchWithCluster := omni.NewConfigPatch(resources.DefaultNamespace, "config-patch-2",
		pair.MakePair("cluster-name", cluster.Metadata().ID()),
		pair.MakePair("machine-set-name", machineSet.Metadata().ID()),
	)

	machineSetConfigPatchWithoutCluster := omni.NewConfigPatch(
		resources.DefaultNamespace,
		"config-patch-3",
		pair.MakePair("machine-set-name", machineSet.Metadata().ID()),
	)

	clusterMachineConfigPatchWithCluster := omni.NewConfigPatch(resources.DefaultNamespace, "config-patch-4",
		pair.MakePair("cluster-name", cluster.Metadata().ID()),
		pair.MakePair("machine-uuid", clusterMachine.Metadata().ID()),
	)

	clusterMachineConfigPatchWithoutCluster := omni.NewConfigPatch(
		resources.DefaultNamespace,
		"config-patch-5",
		pair.MakePair("machine-uuid", clusterMachine.Metadata().ID()),
	)

	suite.Require().NoError(suite.state.Create(ctx, cluster))
	suite.Require().NoError(suite.state.Create(ctx, machineSet))
	suite.Require().NoError(suite.state.Create(ctx, clusterMachine))
	suite.Require().NoError(suite.state.Create(ctx, clusterConfigPatch))
	suite.Require().NoError(suite.state.Create(ctx, machineSetConfigPatchWithCluster))
	suite.Require().NoError(suite.state.Create(ctx, machineSetConfigPatchWithoutCluster))
	suite.Require().NoError(suite.state.Create(ctx, clusterMachineConfigPatchWithCluster))
	suite.Require().NoError(suite.state.Create(ctx, clusterMachineConfigPatchWithoutCluster))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(15)))

	clusterConfigPatch, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, clusterConfigPatch.Metadata())
	suite.Require().NoError(err)

	machineSetConfigPatchWithCluster, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, machineSetConfigPatchWithCluster.Metadata())
	suite.Require().NoError(err)

	machineSetConfigPatchWithoutCluster, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, machineSetConfigPatchWithoutCluster.Metadata())
	suite.Require().NoError(err)

	clusterMachineConfigPatchWithCluster, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, clusterMachineConfigPatchWithCluster.Metadata())
	suite.Require().NoError(err)

	clusterMachineConfigPatchWithoutCluster, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, clusterMachineConfigPatchWithoutCluster.Metadata())
	suite.Require().NoError(err)

	for _, configPatch := range []*omni.ConfigPatch{
		clusterConfigPatch,
		machineSetConfigPatchWithCluster,
		machineSetConfigPatchWithoutCluster,
		clusterMachineConfigPatchWithCluster,
		clusterMachineConfigPatchWithoutCluster,
	} {
		suite.Assert().Equal("cluster", configPatch.Metadata().Labels().Raw()["cluster"])

		_, ok := configPatch.Metadata().Labels().Get("cluster-name")
		suite.Assert().False(ok)
	}

	for _, configPatch := range []*omni.ConfigPatch{machineSetConfigPatchWithCluster, machineSetConfigPatchWithoutCluster} {
		suite.Assert().Equal("machine-set", configPatch.Metadata().Labels().Raw()["machine-set"])

		_, ok := configPatch.Metadata().Labels().Get("machine-set-name")
		suite.Assert().False(ok)
	}

	for _, configPatch := range []*omni.ConfigPatch{clusterMachineConfigPatchWithCluster, clusterMachineConfigPatchWithoutCluster} {
		suite.Assert().Equal("cluster-machine", configPatch.Metadata().Labels().Raw()["cluster-machine"])

		_, ok := configPatch.Metadata().Labels().Get("machine-uuid")
		suite.Assert().False(ok)
	}
}

func (suite *MigrationSuite) TestMigrateMachineFinalizers() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	m1 := omni.NewMachine(resources.DefaultNamespace, "m1")
	m2 := omni.NewMachine(resources.DefaultNamespace, "m2")
	m3 := omni.NewMachine(resources.DefaultNamespace, "m3")

	m1.Metadata().Finalizers().Add("ClusterMachineStatusController")
	m2.Metadata().Finalizers().Add("ClusterMachineStatusController")

	for _, m := range []*omni.Machine{m1, m2, m3} {
		suite.Require().NoError(suite.state.Create(ctx, m))
	}

	_, err := suite.state.Teardown(ctx, m2.Metadata())
	suite.Require().NoError(err)

	cm1 := omni.NewClusterMachine(resources.DefaultNamespace, "m1")
	cm3 := omni.NewClusterMachine(resources.DefaultNamespace, "m3")

	for _, cm := range []*omni.ClusterMachine{cm1, cm3} {
		suite.Require().NoError(suite.state.Create(ctx, cm))
	}

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(15)))

	m1, err = safe.StateGet[*omni.Machine](ctx, suite.state, m1.Metadata())
	suite.Require().NoError(err)

	// no old finalizer
	suite.Assert().True(m1.Metadata().Finalizers().Add("ClusterMachineStatusController"))

	// new finalizer is set
	suite.Assert().True(m1.Metadata().Finalizers().Remove("MachineSetStatusController"))

	m2, err = safe.StateGet[*omni.Machine](ctx, suite.state, m2.Metadata())
	suite.Require().NoError(err)

	// no old and new finalizers (as no ClusterMachine exists)
	suite.Assert().True(m2.Metadata().Finalizers().Add("ClusterMachineStatusController"))
	suite.Assert().False(m2.Metadata().Finalizers().Remove("MachineSetStatusController"))
}

func (suite *MigrationSuite) TestMigrateConfigPatchLabels() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	p1 := omni.NewConfigPatch(resources.DefaultNamespace, "000-p1")
	p2 := omni.NewConfigPatch(resources.DefaultNamespace, "001-p2")

	for _, p := range []*omni.ConfigPatch{p1, p2} {
		suite.Require().NoError(suite.state.Create(ctx, p))
	}

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(15)))

	var err error

	p1, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, p1.Metadata())
	suite.Require().NoError(err)

	_, ok := p1.Metadata().Labels().Get("system-patch")
	suite.Require().Truef(ok, "the label %s is not set on patch with 000- prefix", "system-patch")

	p2, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, p2.Metadata())
	suite.Require().NoError(err)

	_, ok = p2.Metadata().Labels().Get("system-patch")
	suite.Require().Falsef(ok, "the label %s is set on patch with 001- prefix", "system-patch")
}

func (suite *MigrationSuite) TestUpdateMachineStatusClusterRelations() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	msControlPlane := omni.NewMachineStatus(resources.DefaultNamespace, "msControlPlane")
	msControlPlane.Metadata().Labels().Set("cluster", "c1")
	msControlPlane.Metadata().Labels().Set("role-controlplane", "")

	suite.Require().NoError(suite.state.Create(ctx, msControlPlane, state.WithCreateOwner("some-owner")))

	msAvailable := omni.NewMachineStatus(resources.DefaultNamespace, "msAvailable")

	suite.Require().NoError(suite.state.Create(ctx, msAvailable))

	msTearingDown := omni.NewMachineStatus(resources.DefaultNamespace, "msTearingDown")
	msTearingDown.Metadata().Labels().Set("cluster", "c1")

	msTearingDown.Metadata().SetPhase(resource.PhaseTearingDown)

	suite.Require().NoError(suite.state.Create(ctx, msTearingDown))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(15)))

	var err error

	msControlPlane, err = safe.StateGet[*omni.MachineStatus](ctx, suite.state, msControlPlane.Metadata())
	suite.Require().NoError(err)

	// cluster label should disappear
	_, msControlPlaneClusterOk := msControlPlane.Metadata().Labels().Get("cluster")
	suite.Assert().False(msControlPlaneClusterOk, "the label %q is still set on machine status", "cluster")

	// control plane role label should disappear
	_, msControlPlaneRoleOk := msControlPlane.Metadata().Labels().Get("role-controlplane")
	suite.Assert().False(msControlPlaneRoleOk, "the label %q is still set on machine status", "role-controlplane")

	// available label shouldn't be there
	_, msControlPlaneAvailableOk := msControlPlane.Metadata().Labels().Get("available")
	suite.Assert().False(msControlPlaneAvailableOk, "the label %q is unexpectedly set on machine status", "available")

	// owner should be unchanged
	suite.Assert().Equal("some-owner", msControlPlane.Metadata().Owner())

	// cluster field should be set
	suite.Assert().Equal("c1", msControlPlane.TypedSpec().Value.Cluster)

	// role field should be set to control plane
	suite.Assert().Equal(specs.MachineStatusSpec_CONTROL_PLANE, msControlPlane.TypedSpec().Value.Role)

	msAvailable, err = safe.StateGet[*omni.MachineStatus](ctx, suite.state, msAvailable.Metadata())
	suite.Require().NoError(err)

	// cluster label should disappear
	_, msAvailableClusterOk := msAvailable.Metadata().Labels().Get("cluster")
	suite.Assert().False(msAvailableClusterOk, "the label %q is unexpectedly set on machine status", "cluster")

	// available label should be set
	_, msAvailableOk := msAvailable.Metadata().Labels().Get("available")
	suite.Assert().True(msAvailableOk, "the label %q is not set on machine status", "available")

	msTearingDown, err = safe.StateGet[*omni.MachineStatus](ctx, suite.state, msTearingDown.Metadata())
	suite.Require().NoError(err)

	// the labels on the tearing down machine status should be left untouched
	_, msTearingDownClusterOk := msTearingDown.Metadata().Labels().Get("cluster")
	suite.Assert().True(msTearingDownClusterOk, "the label %q is not set on machine status", "cluster")
}

//nolint:staticcheck
func (suite *MigrationSuite) TestAddServiceAccountScopesToUsers() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	id := uuid.New().String()

	user1 := authres.NewUser(resources.DefaultNamespace, fmt.Sprintf("user1-%s", id))
	user1.TypedSpec().Value.Scopes = scope.NewScopes(scope.ClusterAny, scope.MachineRead).Strings()

	user2 := authres.NewUser(resources.DefaultNamespace, fmt.Sprintf("user2-%s", id))
	user2.TypedSpec().Value.Scopes = scope.NewScopes(scope.UserAny).Strings()

	publicKey1 := authres.NewPublicKey(resources.DefaultNamespace, fmt.Sprintf("publicKey1-%s", id))
	publicKey1.TypedSpec().Value.Scopes = scope.NewScopes(scope.ClusterAny).Strings()

	publicKey2 := authres.NewPublicKey(resources.DefaultNamespace, fmt.Sprintf("publicKey2-%s", id))

	suite.Require().NoError(suite.state.Create(ctx, user1))
	suite.Require().NoError(suite.state.Create(ctx, user2))
	suite.Require().NoError(suite.state.Create(ctx, publicKey1))
	suite.Require().NoError(suite.state.Create(ctx, publicKey2))

	// test migration in isolation
	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("addServiceAccountScopesToUsers"))))

	user1, err := safe.StateGet[*authres.User](ctx, suite.state, user1.Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(scope.NewScopes(scope.ClusterAny, scope.MachineRead, scope.ServiceAccountAny).Strings(), user1.TypedSpec().Value.Scopes)

	user2, err = safe.StateGet[*authres.User](ctx, suite.state, user2.Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(scope.NewScopes(scope.UserAny, scope.ServiceAccountAny).Strings(), user2.TypedSpec().Value.Scopes)

	publicKey1, err = safe.StateGet[*authres.PublicKey](ctx, suite.state, publicKey1.Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(scope.NewScopes(scope.ClusterAny, scope.ServiceAccountAny).Strings(), publicKey1.TypedSpec().Value.Scopes)

	publicKey2, err = safe.StateGet[*authres.PublicKey](ctx, suite.state, publicKey2.Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(scope.NewScopes(scope.ServiceAccountAny).Strings(), publicKey2.TypedSpec().Value.Scopes)
}

func (suite *MigrationSuite) TestMigrateLabels() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	id := uuid.New().String()

	machineStatus := omni.NewClusterMachineStatus(resources.DefaultNamespace, "__1")
	machineStatus.Metadata().Labels().Set("role-controlplane", "")
	machineStatus.Metadata().Labels().Set("cluster", "cluster5")
	suite.Require().NoError(suite.state.Create(ctx, machineStatus))

	machineStatus = omni.NewClusterMachineStatus(resources.DefaultNamespace, id)
	machineStatus.Metadata().Labels().Set("role-worker", "")
	machineStatus.Metadata().Labels().Set("cluster", "cluster1")
	suite.Require().NoError(suite.state.Create(ctx, machineStatus))

	machineLabels := omni.NewMachineLabels(resources.DefaultNamespace, id)
	machineLabels.Metadata().Labels().Set("user-label", "value")
	suite.Require().NoError(suite.state.Create(ctx, machineLabels))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("migrateLabels", "dropOldLabels"))))

	updatedMachineStatus, err := suite.state.Get(ctx, machineStatus.Metadata())
	suite.Require().NoError(err)

	_, ok := updatedMachineStatus.Metadata().Labels().Get(omni.LabelWorkerRole)
	suite.Require().True(ok)

	cluster, ok := updatedMachineStatus.Metadata().Labels().Get(omni.LabelCluster)
	suite.Require().True(ok)
	suite.Require().Equal("cluster1", cluster)

	updatedMachineLabels, err := suite.state.Get(ctx, machineLabels.Metadata())
	suite.Require().NoError(err)

	userLabel, ok := updatedMachineLabels.Metadata().Labels().Get("user-label")
	suite.Require().True(ok)
	suite.Require().Equal("value", userLabel)

	_, ok = updatedMachineStatus.Metadata().Labels().Get("cluster")
	suite.Require().False(ok)
}

//nolint:staticcheck
func (suite *MigrationSuite) TestConvertScopesToRoles() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	userWithNoScopes := authres.NewUser(resources.DefaultNamespace, fmt.Sprintf("userWithNoScopes-%s", uuid.New().String()))
	userWithNoScopes.TypedSpec().Value.Scopes = []string{}

	userWithReadScopes := authres.NewUser(resources.DefaultNamespace, fmt.Sprintf("userWithReadScopes-%s", uuid.New().String()))
	userWithReadScopes.TypedSpec().Value.Scopes = scope.NewScopes(scope.MachineRead, scope.ClusterRead).Strings()

	userWithModifyScopes := authres.NewUser(resources.DefaultNamespace, fmt.Sprintf("userWithModifyScopes-%s", uuid.New().String()))
	userWithModifyScopes.TypedSpec().Value.Scopes = scope.NewScopes(scope.ClusterModify).Strings()

	userWithUserMgmtScopes := authres.NewUser(resources.DefaultNamespace, fmt.Sprintf("userWithUserMgmtScopes-%s", uuid.New().String()))
	userWithUserMgmtScopes.TypedSpec().Value.Scopes = scope.NewScopes(scope.UserRead).Strings()

	userWithServiceAccountScopes := authres.NewUser(resources.DefaultNamespace, fmt.Sprintf("userWithServiceAccountScopes-%s", uuid.New().String()))
	userWithServiceAccountScopes.TypedSpec().Value.Scopes = scope.NewScopes(scope.ServiceAccountCreate).Strings()

	pubKeyWithNoScopes := authres.NewPublicKey(resources.DefaultNamespace, fmt.Sprintf("pubKeyWithNoScopes-%s", uuid.New().String()))
	pubKeyWithNoScopes.TypedSpec().Value.Scopes = []string{}

	pubKeyWithReadScopes := authres.NewPublicKey(resources.DefaultNamespace, fmt.Sprintf("pubKeyWithReadScopes-%s", uuid.New().String()))
	pubKeyWithReadScopes.TypedSpec().Value.Scopes = scope.NewScopes(scope.MachineRead, scope.ClusterRead).Strings()

	pubKeyWithModifyScopes := authres.NewPublicKey(resources.DefaultNamespace, fmt.Sprintf("pubKeyWithModifyScopes-%s", uuid.New().String()))
	pubKeyWithModifyScopes.TypedSpec().Value.Scopes = scope.NewScopes(scope.ClusterModify).Strings()

	pubKeyWithUserMgmtScopes := authres.NewPublicKey(resources.DefaultNamespace, fmt.Sprintf("pubKeyWithUserMgmtScopes-%s", uuid.New().String()))
	pubKeyWithUserMgmtScopes.TypedSpec().Value.Scopes = scope.NewScopes(scope.UserRead).Strings()

	pubKeyWithServiceAccountScopes := authres.NewPublicKey(resources.DefaultNamespace, fmt.Sprintf("pubKeyWithServiceAccountScopes-%s", uuid.New().String()))
	pubKeyWithServiceAccountScopes.TypedSpec().Value.Scopes = scope.NewScopes(scope.ServiceAccountCreate).Strings()

	suite.Require().NoError(suite.state.Create(ctx, userWithNoScopes))
	suite.Require().NoError(suite.state.Create(ctx, userWithReadScopes))
	suite.Require().NoError(suite.state.Create(ctx, userWithModifyScopes))
	suite.Require().NoError(suite.state.Create(ctx, userWithUserMgmtScopes))
	suite.Require().NoError(suite.state.Create(ctx, userWithServiceAccountScopes))

	suite.Require().NoError(suite.state.Create(ctx, pubKeyWithNoScopes))
	suite.Require().NoError(suite.state.Create(ctx, pubKeyWithReadScopes))
	suite.Require().NoError(suite.state.Create(ctx, pubKeyWithModifyScopes))
	suite.Require().NoError(suite.state.Create(ctx, pubKeyWithUserMgmtScopes))
	suite.Require().NoError(suite.state.Create(ctx, pubKeyWithServiceAccountScopes))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("convertScopesToRoles"))))

	updatedUserWithNoScopes, err := safe.StateGet[*authres.User](ctx, suite.state, userWithNoScopes.Metadata())
	suite.Require().NoError(err)

	updatedUserWithReadScopes, err := safe.StateGet[*authres.User](ctx, suite.state, userWithReadScopes.Metadata())
	suite.Require().NoError(err)

	updatedUserWithModifyScopes, err := safe.StateGet[*authres.User](ctx, suite.state, userWithModifyScopes.Metadata())
	suite.Require().NoError(err)

	updatedUserWithUserMgmtScopes, err := safe.StateGet[*authres.User](ctx, suite.state, userWithUserMgmtScopes.Metadata())
	suite.Require().NoError(err)

	updatedUserWithServiceAccountScopes, err := safe.StateGet[*authres.User](ctx, suite.state, userWithServiceAccountScopes.Metadata())
	suite.Require().NoError(err)

	updatedPubKeyWithNoScopes, err := safe.StateGet[*authres.PublicKey](ctx, suite.state, pubKeyWithNoScopes.Metadata())
	suite.Require().NoError(err)

	updatedPubKeyWithReadScopes, err := safe.StateGet[*authres.PublicKey](ctx, suite.state, pubKeyWithReadScopes.Metadata())
	suite.Require().NoError(err)

	updatedPubKeyWithModifyScopes, err := safe.StateGet[*authres.PublicKey](ctx, suite.state, pubKeyWithModifyScopes.Metadata())
	suite.Require().NoError(err)

	updatedPubKeyWithUserMgmtScopes, err := safe.StateGet[*authres.PublicKey](ctx, suite.state, pubKeyWithUserMgmtScopes.Metadata())
	suite.Require().NoError(err)

	updatedPubKeyWithServiceAccountScopes, err := safe.StateGet[*authres.PublicKey](ctx, suite.state, pubKeyWithServiceAccountScopes.Metadata())
	suite.Require().NoError(err)

	suite.Require().Equal(string(role.None), updatedUserWithNoScopes.TypedSpec().Value.Role)
	suite.Require().Equal(string(role.None), updatedPubKeyWithNoScopes.TypedSpec().Value.Role)

	suite.Require().Equal(string(role.Reader), updatedUserWithReadScopes.TypedSpec().Value.Role)
	suite.Require().Equal(string(role.Reader), updatedPubKeyWithReadScopes.TypedSpec().Value.Role)

	suite.Require().Equal(string(role.Operator), updatedUserWithModifyScopes.TypedSpec().Value.Role)
	suite.Require().Equal(string(role.Operator), updatedPubKeyWithModifyScopes.TypedSpec().Value.Role)

	suite.Require().Equal(string(role.Admin), updatedUserWithUserMgmtScopes.TypedSpec().Value.Role)
	suite.Require().Equal(string(role.Admin), updatedPubKeyWithUserMgmtScopes.TypedSpec().Value.Role)

	suite.Require().Equal(string(role.Admin), updatedUserWithServiceAccountScopes.TypedSpec().Value.Role)
	suite.Require().Equal(string(role.Admin), updatedPubKeyWithServiceAccountScopes.TypedSpec().Value.Role)
}

func (suite *MigrationSuite) TestLowercaseEmails() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	identityUppercase := authres.NewIdentity(resources.DefaultNamespace, "USER@a.com")
	identityUppercase.TypedSpec().Value.UserId = "123"
	identityUppercase.Metadata().Labels().Set("user-id", "123")

	identityConflict := authres.NewIdentity(resources.DefaultNamespace, "B@a.com")
	identityConflict.TypedSpec().Value.UserId = "555"
	identityUnchanged := authres.NewIdentity(resources.DefaultNamespace, "b@a.com")
	identityUnchanged.TypedSpec().Value.UserId = "111"

	identityOverwrite := authres.NewIdentity(resources.DefaultNamespace, "C@a.com")
	identityOverwrite.TypedSpec().Value.UserId = "ABC"
	identityOverwritten := authres.NewIdentity(resources.DefaultNamespace, "c@a.com")
	identityOverwritten.TypedSpec().Value.UserId = "eee"

	publicKey := authres.NewPublicKey(resources.DefaultNamespace, "1")
	publicKey.TypedSpec().Value.Identity = &specs.Identity{
		Email: "USER@a.com",
	}

	danglingPublicKey := authres.NewPublicKey(resources.DefaultNamespace, "2")

	suite.Require().NoError(suite.state.Create(ctx, identityUppercase))
	suite.Require().NoError(suite.state.Create(ctx, identityConflict))
	suite.Require().NoError(suite.state.Create(ctx, identityUnchanged))
	suite.Require().NoError(suite.state.Create(ctx, identityOverwritten))
	suite.Require().NoError(suite.state.Create(ctx, identityOverwrite))

	suite.Require().NoError(suite.state.Create(ctx, publicKey))
	// Run shouldn't fail if it finds this invalid public key
	suite.Require().NoError(suite.state.Create(ctx, danglingPublicKey))

	suite.Require().NoError(suite.manager.Run(ctx))

	// USER@a.com was dropped
	_, err := safe.ReaderGet[*authres.Identity](ctx, suite.state, identityUppercase.Metadata())
	suite.Require().Error(err)

	// USER@a.com should be replaced by user@a.com
	identity, err := safe.ReaderGet[*authres.Identity](ctx, suite.state, authres.NewIdentity(resources.DefaultNamespace, "user@a.com").Metadata())
	suite.Require().NoError(err)

	suite.Require().Equal(identityUppercase.TypedSpec().Value.UserId, identity.TypedSpec().Value.UserId)
	suite.Require().True(identityUppercase.Metadata().Labels().Equal(*identity.Metadata().Labels()))

	// b@a.com should not be replaced as it's newer than B@a.com
	identity, err = safe.ReaderGet[*authres.Identity](ctx, suite.state, authres.NewIdentity(resources.DefaultNamespace, "b@a.com").Metadata())
	suite.Require().NoError(err)

	_, err = safe.ReaderGet[*authres.Identity](ctx, suite.state, authres.NewIdentity(resources.DefaultNamespace, "B@a.com").Metadata())
	suite.Require().Error(err)

	suite.Require().Equal(identityUnchanged.TypedSpec().Value.UserId, identity.TypedSpec().Value.UserId)

	// public key that belongs to email USER@a.com should be updated to point to user@a.com
	key, err := safe.ReaderGet[*authres.PublicKey](ctx, suite.state, publicKey.Metadata())
	suite.Require().NoError(err)
	suite.Require().Equal("user@a.com", key.TypedSpec().Value.Identity.Email)

	// c@a.com is overwritten by C@a.com as it has older creation time
	// should have a new spec with the new user id
	identity, err = safe.ReaderGet[*authres.Identity](ctx, suite.state, authres.NewIdentity(resources.DefaultNamespace, "c@a.com").Metadata())
	suite.Require().NoError(err)
	suite.Require().Equal(identityOverwrite.TypedSpec().Value.UserId, identity.TypedSpec().Value.UserId)
}

func (suite *MigrationSuite) TestPatchesExtraction() {
	ctx := suite.T().Context()

	clusterName := "patches"
	machines := []machine{
		{
			name: "m1",
			labels: map[string]string{
				"role-controlplane": "",
			},
		},
		{
			name: "m2",
			labels: map[string]string{
				"role-controlplane": "",
			},
		},
		{
			name: "m3",
			labels: map[string]string{
				"role-controlplane": "",
			},
		},
		{
			name: "m4",
			labels: map[string]string{
				"role-worker": "",
			},
		},
	}

	suite.createClusterWithMachines(ctx, clusterName, machines, true)

	createResources := []pair.Pair[string, resource.Resource]{
		pair.MakePair[string, resource.Resource]((&omnictrl.LoadBalancerController{}).Name(), omni.NewLoadBalancerConfig(resources.DefaultNamespace, clusterName)),
		pair.MakePair[string, resource.Resource]("", omni.NewClusterSecrets(resources.DefaultNamespace, clusterName)),
	}

	createResources = append(createResources, xslices.Map(machines, func(m machine) pair.Pair[string, resource.Resource] {
		return pair.MakePair[string, resource.Resource](omnictrl.NewClusterMachineConfigController("factory-test.talos.dev", nil, 8090).Name(),
			omni.NewClusterMachineConfig(resources.DefaultNamespace, clusterName+"."+m.name))
	})...)

	for _, res := range createResources {
		suite.Require().NoError(res.F2.Metadata().SetOwner(res.F1))
		suite.Require().NoError(suite.state.Create(ctx, res.F2, state.WithCreateOwner(res.F1)))
	}

	suite.Require().NoError(suite.manager.Run(ctx))

	for _, m := range []string{"patches.m1", "patches.m2", "patches.m3", "patches.m4"} {
		patches, err := safe.StateGet[*omni.ClusterMachineConfigPatches](ctx, suite.state, omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, m).Metadata())
		suite.Require().NoError(err)
		suite.assertLabel(patches, omni.SystemLabelPrefix+"cluster", "patches")

		patchList, err := patches.TypedSpec().Value.GetUncompressedPatches()
		suite.Require().NoError(err)

		suite.Require().Len(patchList, 1)

		config, err := safe.StateGet[*omni.ClusterMachineConfig](ctx, suite.state, omni.NewClusterMachineConfig(resources.DefaultNamespace, m).Metadata())
		suite.Require().NoError(err)

		_, ok := config.Metadata().Annotations().Get("inputResourceVersion")
		suite.Require().True(ok)
	}
}

func (suite *MigrationSuite) TestInstallDiskPatchMigration() {
	ctx := suite.T().Context()

	clusterName := "patches"
	machines := []machine{
		{
			name: "m1",
			labels: map[string]string{
				"role-worker": "",
			},
		},
	}

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	version.TypedSpec().Value.Version = 1

	suite.Require().NoError(suite.state.Create(ctx, version))

	suite.createClusterWithMachines(ctx, clusterName, machines, false)

	m1 := "patches.m1"

	m1Status := omni.NewMachineStatus(resources.DefaultNamespace, m1)
	m1Status.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{
		Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
			{
				LinuxName: "/dev/vdb",
				Size:      8e9,
			},
		},
	}
	m1Status.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		Id: "id",
	}

	lbStatus := omni.NewLoadBalancerStatus(resources.DefaultNamespace, clusterName)
	lbStatus.TypedSpec().Value.Healthy = true

	machineSet := omni.NewMachineSet(
		resources.DefaultNamespace,
		omni.WorkersResourceID(clusterName),
	)
	machineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	clusterConfigVersion := omni.NewClusterConfigVersion(resources.DefaultNamespace, clusterName)
	clusterConfigVersion.TypedSpec().Value.Version = "v1.4"

	createResources := []pair.Pair[string, resource.Resource]{
		pair.MakePair[string, resource.Resource]((&omnictrl.LoadBalancerController{}).Name(), omni.NewLoadBalancerConfig(resources.DefaultNamespace, clusterName)),
		pair.MakePair[string, resource.Resource]((&omnictrl.LoadBalancerController{}).Name(), lbStatus),
		pair.MakePair[string, resource.Resource]("", omni.NewClusterSecrets(resources.DefaultNamespace, clusterName)),
		pair.MakePair[string, resource.Resource]("", m1Status),
		pair.MakePair[string, resource.Resource]("", omni.NewMachine(resources.DefaultNamespace, m1)),
		pair.MakePair[string, resource.Resource]("", omni.NewMachineSetNode(resources.DefaultNamespace, m1, machineSet)),
		pair.MakePair[string, resource.Resource]("", clusterConfigVersion),
	}

	createResources = append(createResources, xslices.Map(machines, func(m machine) pair.Pair[string, resource.Resource] {
		return pair.MakePair[string, resource.Resource](omnictrl.NewClusterMachineConfigController("factory-test.talos.dev", nil, 8090).Name(),
			omni.NewClusterMachineConfig(resources.DefaultNamespace, clusterName+"."+m.name))
	})...)

	for _, res := range createResources {
		suite.Require().NoError(res.F2.Metadata().SetOwner(res.F1))
		suite.Require().NoError(suite.state.Create(ctx, res.F2, state.WithCreateOwner(res.F1)))
	}

	suite.Require().NoError(suite.manager.Run(ctx))

	options, err := safe.StateGet[*omni.MachineConfigGenOptions](ctx, suite.state, omni.NewMachineConfigGenOptions(resources.DefaultNamespace, m1).Metadata())
	suite.Require().NoError(err)
	suite.Require().Equal("/dev/vdb", options.TypedSpec().Value.InstallDisk)

	config, err := safe.StateGet[*omni.ClusterMachineConfig](ctx, suite.state, omni.NewClusterMachineConfig(resources.DefaultNamespace, m1).Metadata())
	suite.Require().NoError(err)

	_, ok := config.Metadata().Annotations().Get("inputResourceVersion")
	suite.Require().True(ok)

	oldVer := config.Metadata().Version()

	// run controllers and verify that the config resource hasn't changed
	runtime, err := runtime.NewRuntime(suite.state, suite.logger)
	suite.Require().NoError(err)

	suite.Require().NoError(runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController()))
	suite.Require().NoError(runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("factory-test.talos.dev", nil, 8090)))

	runCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err = runtime.Run(runCtx)
	if !errors.Is(err, context.Canceled) {
		suite.Require().NoError(err)
	}

	config, err = safe.StateGet[*omni.ClusterMachineConfig](ctx, suite.state, omni.NewClusterMachineConfig(resources.DefaultNamespace, m1).Metadata())
	suite.Require().NoError(err)

	suite.Require().Equal(oldVer, config.Metadata().Version())
}

func (suite *MigrationSuite) TestSiderolinkCounterMigration() {
	ctx := suite.T().Context()

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	version.TypedSpec().Value.Version = 1
	suite.Require().NoError(suite.state.Create(ctx, version))

	counter := siderolink.NewDeprecatedLinkCounter(resources.MetricsNamespace, "test")
	counter.TypedSpec().Value.BytesReceived = 100
	counter.TypedSpec().Value.BytesSent = 200
	counter.TypedSpec().Value.LastAlive = timestamppb.Now()
	suite.Require().NoError(suite.state.Create(ctx, counter))

	suite.Require().NoError(suite.manager.Run(ctx))

	msl, err := safe.StateGet[*omni.MachineStatusLink](ctx, suite.state, omni.NewMachineStatusLink(resources.MetricsNamespace, "test").Metadata())
	suite.Require().NoError(err)

	suite.Require().Equal(counter.TypedSpec().Value.BytesReceived, msl.TypedSpec().Value.SiderolinkCounter.BytesReceived)
	suite.Require().Equal(counter.TypedSpec().Value.BytesSent, msl.TypedSpec().Value.SiderolinkCounter.BytesSent)
	suite.Require().Equal(counter.TypedSpec().Value.LastAlive.AsTime(), msl.TypedSpec().Value.SiderolinkCounter.LastAlive.AsTime())

	_, err = safe.StateGet[*siderolink.DeprecatedLinkCounter](ctx, suite.state, counter.Metadata())
	suite.Require().Error(err)
	suite.Require().True(state.IsNotFoundError(err))
}

func (suite *MigrationSuite) TestFixClusterConfigVersionOwnership() {
	ctx := suite.T().Context()

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	version.TypedSpec().Value.Version = 1
	suite.Require().NoError(suite.state.Create(ctx, version))

	c1 := omni.NewClusterConfigVersion(resources.DefaultNamespace, "1")
	c2 := omni.NewClusterConfigVersion(resources.DefaultNamespace, "2")

	expectedName := "ClusterConfigVersionController"

	suite.Require().NoError(suite.state.Create(ctx, c1, state.WithCreateOwner("ClusterController")))
	suite.Require().NoError(suite.state.Create(ctx, c2, state.WithCreateOwner(expectedName)))

	suite.Require().NoError(suite.manager.Run(ctx))

	var err error

	c1, err = safe.StateGet[*omni.ClusterConfigVersion](ctx, suite.state, c1.Metadata())
	suite.Require().NoError(err)

	c2, err = safe.StateGet[*omni.ClusterConfigVersion](ctx, suite.state, c2.Metadata())
	suite.Require().NoError(err)

	suite.Require().Equal(expectedName, c1.Metadata().Owner())
	suite.Require().Equal(expectedName, c2.Metadata().Owner())
}

func (suite *MigrationSuite) TestUpdateClusterMachineConfigPatchesLabels() {
	ctx := suite.T().Context()

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	version.TypedSpec().Value.Version = 23
	suite.Require().NoError(suite.state.Create(ctx, version))

	cp1 := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "1")
	cp2 := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "2")

	suite.Require().NoError(suite.state.Create(ctx, cp1, state.WithCreateOwner("MachineSetStatusController")))
	suite.Require().NoError(suite.state.Create(ctx, cp2, state.WithCreateOwner("MachineSetStatusController")))

	cm := omni.NewClusterMachine(resources.DefaultNamespace, "2")

	cm.Metadata().Labels().Set(omni.LabelMachineSet, "some")
	cm.Metadata().Labels().Set(omni.LabelCluster, "c1")

	suite.Require().NoError(suite.state.Create(ctx, cm, state.WithCreateOwner("MachineSetStatusController")))

	suite.Require().NoError(suite.manager.Run(ctx))

	var err error

	cp1, err = safe.StateGetByID[*omni.ClusterMachineConfigPatches](ctx, suite.state, "1")
	suite.Require().NoError(err)

	cp2, err = safe.StateGetByID[*omni.ClusterMachineConfigPatches](ctx, suite.state, "2")
	suite.Require().NoError(err)

	suite.Assert().True(cp1.Metadata().Labels().Empty())
	suite.Assert().False(cp2.Metadata().Labels().Empty())

	val, ok := cp2.Metadata().Labels().Get(omni.LabelMachineSet)
	suite.Require().True(ok)

	suite.Assert().Equal("some", val)

	val, ok = cp2.Metadata().Labels().Get(omni.LabelCluster)
	suite.Require().True(ok)

	suite.Assert().Equal("c1", val)
}

func (suite *MigrationSuite) TestClearEmptyConfigPatches() {
	ctx := suite.T().Context()

	cp1 := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "1")

	err := cp1.TypedSpec().Value.SetUncompressedPatches([]string{
		"foo: yaml",
		"bar: yaml",
		"",
		"baz: yaml",
	})
	suite.Require().NoError(err)

	cp2 := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "2")

	err = cp2.TypedSpec().Value.SetUncompressedPatches([]string{
		"",
		"",
	})
	suite.Require().NoError(err)

	suite.Require().NoError(suite.state.Create(ctx, cp1, state.WithCreateOwner("MachineSetStatusController")))
	suite.Require().NoError(suite.state.Create(ctx, cp2, state.WithCreateOwner("MachineSetStatusController")))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("clearEmptyConfigPatches"))))

	cp1After, err := safe.StateGetByID[*omni.ClusterMachineConfigPatches](ctx, suite.state, "1")
	suite.Require().NoError(err)

	cp2After, err := safe.StateGetByID[*omni.ClusterMachineConfigPatches](ctx, suite.state, "2")
	suite.Require().NoError(err)

	patches, err := cp1After.TypedSpec().Value.GetUncompressedPatches()
	suite.Require().NoError(err)

	suite.Assert().Equal([]string{
		"foo: yaml",
		"bar: yaml",
		"baz: yaml",
	}, patches)

	patches, err = cp2After.TypedSpec().Value.GetUncompressedPatches()
	suite.Require().NoError(err)

	suite.Assert().Empty(patches)
}

func (suite *MigrationSuite) TestCleanupDanglingSchematicConfigurations() {
	ctx := suite.T().Context()

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	version.TypedSpec().Value.Version = 23
	suite.Require().NoError(suite.state.Create(ctx, version))

	sc1 := omni.NewSchematicConfiguration(resources.DefaultNamespace, "1")
	sc2 := omni.NewSchematicConfiguration(resources.DefaultNamespace, "2")
	sc3 := omni.NewSchematicConfiguration(resources.DefaultNamespace, "3")
	sc3.Metadata().Finalizers().Add("some-finalizer")

	suite.Require().NoError(suite.state.Create(ctx, sc1, state.WithCreateOwner("SchematicConfigurationController")))
	suite.Require().NoError(suite.state.Create(ctx, sc2, state.WithCreateOwner("SchematicConfigurationController")))
	suite.Require().NoError(suite.state.Create(ctx, sc3, state.WithCreateOwner("SchematicConfigurationController")))

	cm := omni.NewClusterMachine(resources.DefaultNamespace, "1")

	cm.Metadata().Labels().Set(omni.LabelMachineSet, "some")
	cm.Metadata().Labels().Set(omni.LabelCluster, "c1")

	suite.Require().NoError(suite.state.Create(ctx, cm, state.WithCreateOwner("MachineSetStatusController")))

	suite.Require().NoError(suite.manager.Run(ctx))

	var err error

	_, err = safe.StateGetByID[*omni.SchematicConfiguration](ctx, suite.state, "1")
	suite.Require().NoError(err)

	_, err = safe.StateGetByID[*omni.SchematicConfiguration](ctx, suite.state, "2")
	suite.Require().True(state.IsNotFoundError(err))

	_, err = safe.StateGetByID[*omni.SchematicConfiguration](ctx, suite.state, "3")
	suite.Require().True(state.IsNotFoundError(err))
}

func (suite *MigrationSuite) TestCleanupExtensionConfigurationStatuses() {
	ctx := suite.T().Context()

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	version.TypedSpec().Value.Version = 23
	suite.Require().NoError(suite.state.Create(ctx, version))

	status := omni.NewExtensionsConfigurationStatus(resources.DefaultNamespace, "1")

	suite.Require().NoError(suite.state.Create(ctx, status, state.WithCreateOwner("SchematicConfigurationController")))

	suite.Require().NoError(suite.manager.Run(ctx))

	_, err := safe.StateGetByID[*omni.ExtensionsConfigurationStatus](ctx, suite.state, "1")
	suite.Require().True(state.IsNotFoundError(err))
}

func (suite *MigrationSuite) TestDropExtensionsConfigurationFinalizers() {
	ctx := suite.T().Context()

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	suite.Require().NoError(suite.state.Create(ctx, version))

	configuration := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "1")

	configuration.Metadata().Finalizers().Add(omnictrl.SchematicConfigurationControllerName)
	configuration.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)

	suite.Require().NoError(suite.state.Create(ctx, configuration))

	suite.Require().NoError(suite.manager.Run(ctx))

	res, err := safe.StateGetByID[*omni.ExtensionsConfiguration](ctx, suite.state, "1")

	suite.Require().NoError(err)

	suite.Require().EqualValues([]string{omnictrl.MachineExtensionsControllerName}, *res.Metadata().Finalizers())
}

func (suite *MigrationSuite) TestSetMachineStatusSnapshotOwner() {
	ctx, cancel := context.WithCancel(suite.T().Context())
	defer cancel()

	items := []*omni.MachineStatusSnapshot{
		omni.NewMachineStatusSnapshot(resources.DefaultNamespace, "test1"),
		omni.NewMachineStatusSnapshot(resources.DefaultNamespace, "test2"),
		omni.NewMachineStatusSnapshot(resources.DefaultNamespace, "test3"),
	}

	for _, item := range items[:2] {
		suite.Require().NoError(suite.state.Create(ctx, item))
	}

	for _, item := range items[2:] {
		suite.Require().NoError(suite.state.Create(
			ctx,
			item,
			state.WithCreateOwner(omnictrl.MachineStatusSnapshotControllerName)),
		)
	}

	// test migration in isolation
	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("setMachineStatusSnapshotOwner"))))

	check := func(item *omni.MachineStatusSnapshot, expectedVersion int) {
		result, err := safe.StateGet[*omni.MachineStatusSnapshot](ctx, suite.state, item.Metadata())
		suite.Require().NoError(err)
		suite.Require().Equal(omnictrl.MachineStatusSnapshotControllerName, result.Metadata().Owner())
		suite.Require().EqualValues(result.Metadata().Version().Value(), expectedVersion)
	}

	for _, item := range items[:2] {
		check(item, 2)
	}

	for _, item := range items[2:] {
		check(item, 1)
	}
}

func (suite *MigrationSuite) TestMigrateInstallImageConfigIntoGenOptions() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	suite.T().Cleanup(cancel)

	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, "test")

	machineStatus.TypedSpec().Value.SecurityState = &specs.SecurityState{
		SecureBoot:    true,
		BootedWithUki: true,
	}

	machineStatus.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		Invalid: true,
	}

	suite.Require().NoError(suite.state.Create(ctx, machineStatus))

	clusterMachineTalosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, "test")

	clusterMachineTalosVersion.TypedSpec().Value.TalosVersion = "v1.4.0"
	clusterMachineTalosVersion.TypedSpec().Value.SchematicId = "test-schematic-id"

	suite.Require().NoError(suite.state.Create(ctx, clusterMachineTalosVersion))

	schematicConfig := omni.NewSchematicConfiguration(resources.DefaultNamespace, "test")

	suite.Require().NoError(suite.state.Create(ctx, schematicConfig))

	// prepare the needed resources for reconcileConfigInputs to update inputs versions in the migration

	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "test")
	clusterMachineConfig := omni.NewClusterMachineConfig(resources.DefaultNamespace, "test")
	configPatches := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, "test")
	genOptions := omni.NewMachineConfigGenOptions(resources.DefaultNamespace, "test")
	clusterSecrets := omni.NewClusterSecrets(resources.DefaultNamespace, "test-cluster")
	lbConfig := omni.NewLoadBalancerConfig(resources.DefaultNamespace, "test-cluster")
	cluster := omni.NewCluster(resources.DefaultNamespace, "test-cluster")

	clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "test-cluster")
	clusterMachineConfig.Metadata().Annotations().Set(helpers.InputResourceVersionAnnotation, "before")

	suite.Require().NoError(clusterMachineConfig.Metadata().SetOwner(omnictrl.ClusterMachineConfigControllerName))

	for _, res := range []resource.Resource{
		clusterMachine, clusterMachineConfig, configPatches, genOptions, clusterSecrets, lbConfig, cluster,
		omni.NewMachineStatus(resources.DefaultNamespace, "test2"),
		omni.NewMachineConfigGenOptions(resources.DefaultNamespace, "test2"),
		omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, "test2"),
	} {
		suite.Require().NoError(suite.state.Create(ctx, res, state.WithCreateOwner(res.Metadata().Owner())))
	}

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("migrateInstallImageConfigIntoGenOptions"))))

	genOptions, err := safe.StateGet[*omni.MachineConfigGenOptions](ctx, suite.state, omni.NewMachineConfigGenOptions(resources.DefaultNamespace, "test").Metadata())
	suite.Require().NoError(err)

	installImage := genOptions.TypedSpec().Value.InstallImage
	suite.Require().NotNil(installImage)

	suite.Equal("v1.4.0", installImage.TalosVersion)
	suite.Equal("test-schematic-id", installImage.SchematicId)
	suite.True(installImage.SchematicInitialized)
	suite.True(installImage.SchematicInvalid)
	suite.True(installImage.SecurityState.SecureBoot)

	// assert that the input version is updated on the ClusterMachineConfig

	clusterMachineConfig, err = safe.StateGet[*omni.ClusterMachineConfig](ctx, suite.state, clusterMachineConfig.Metadata())
	suite.Require().NoError(err)

	annotation, ok := clusterMachineConfig.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
	suite.True(ok)
	suite.NotEmpty(annotation)
	suite.NotEqual("before", annotation)
}

func (suite *MigrationSuite) TestDropAllMaintenanceConfigs() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), time.Second*5)

	defer cancel()

	connectionParams := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
	connectionParams.TypedSpec().Value.ApiEndpoint = "grpc://127.0.0.1:8080"

	suite.Require().NoError(suite.state.Create(ctx, connectionParams))

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	version.TypedSpec().Value.Version = 30

	suite.Require().NoError(suite.state.Create(ctx, version))

	clusterName := "maintenance"
	machines := []machine{
		{
			name: "m1",
			labels: map[string]string{
				"role-worker": "",
			},
		},
		{
			name: "m2",
			labels: map[string]string{
				"role-worker": "",
			},
		},
	}

	suite.createClusterWithMachines(ctx, clusterName, machines, false)

	m1 := "maintenance.m1"
	m2 := "maintenance.m2"

	m1Status := omni.NewMachineStatus(resources.DefaultNamespace, m1)
	m1Status.TypedSpec().Value.TalosVersion = "v1.4.0"
	m1Status.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		Id: "id",
	}

	m2Status := omni.NewMachineStatus(resources.DefaultNamespace, m2)
	m2Status.TypedSpec().Value.TalosVersion = "v1.5.0"
	m2Status.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		Id: "id",
	}

	suite.Require().NoError(suite.state.Create(ctx, m1Status))
	suite.Require().NoError(suite.state.Create(ctx, m2Status))
	suite.Require().NoError(suite.state.Create(ctx, omni.NewConfigPatch(resources.DefaultNamespace, migration.MaintenanceConfigPatchPrefix+m2Status.Metadata().ID())))

	deprecatedControllerName := "MaintenanceConfigPatchController"

	suite.Require().NoError(suite.state.AddFinalizer(ctx, m2Status.Metadata(), deprecatedControllerName))

	suite.Require().NoError(suite.manager.Run(ctx))

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, migration.MaintenanceConfigPatchPrefix+m2)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{m1, m2}, func(r *omni.MachineStatus, assertion *assert.Assertions) {
		assertion.False(r.Metadata().Finalizers().Has(deprecatedControllerName))
	})
}

func (suite *MigrationSuite) testDeleteDeprecatedResources(createRes func(id string) resource.Resource, migrationFilter string) {
	ctx, cancel := context.WithTimeout(suite.T().Context(), time.Second*5)

	defer cancel()

	err := suite.state.Create(ctx, createRes("1"))

	suite.Require().NoError(err)

	res := createRes("2")

	res.Metadata().Finalizers().Add("dummy")
	suite.Require().NoError(res.Metadata().SetOwner("some"))

	err = suite.state.Create(ctx, res, state.WithCreateOwner("some"))

	suite.Require().NoError(err)

	res = createRes("3")

	suite.Require().NoError(res.Metadata().SetOwner("some"))
	res.Metadata().SetPhase(resource.PhaseTearingDown)

	err = suite.state.Create(ctx, res, state.WithCreateOwner("some"))

	suite.Require().NoError(err)

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith(migrationFilter))))

	list, err := suite.state.List(ctx, resource.NewMetadata(res.Metadata().Namespace(), res.Metadata().Type(), "", resource.VersionUndefined))

	suite.Require().NoError(err)

	suite.Require().Empty(list.Items)
}

func (suite *MigrationSuite) TestDeleteMachineSetRequiredMachines() {
	suite.testDeleteDeprecatedResources(func(id string) resource.Resource {
		return typed.NewResource[null, machineSetRequiredMachinesExtension](
			resource.NewMetadata(resources.DefaultNamespace, migration.MachineSetRequiredMachinesType, id, resource.VersionUndefined),
			null{},
		)
	}, "deleteMachineSetRequiredMachines")
}

func (suite *MigrationSuite) TestDeleteMachineClassStatuses() {
	suite.testDeleteDeprecatedResources(func(id string) resource.Resource {
		return typed.NewResource[null, machineClassStatusExtension](
			resource.NewMetadata(resources.DefaultNamespace, migration.MachineClassStatusType, id, resource.VersionUndefined),
			null{},
		)
	}, "deleteMachineClassStatuses")
}

func (suite *MigrationSuite) TestRemoveMaintenanceConfigPatchFinalizers() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	deprecatedControllerName := "MaintenanceConfigPatchController"

	m1Status := omni.NewMachineStatus(resources.DefaultNamespace, "machine1")
	m1Status.Metadata().Finalizers().Add(deprecatedControllerName)

	m2Status := omni.NewMachineStatus(resources.DefaultNamespace, "machine2")
	m2Status.Metadata().Finalizers().Add(deprecatedControllerName)
	m2Status.Metadata().SetPhase(resource.PhaseTearingDown)

	m3Status := omni.NewMachineStatus(resources.DefaultNamespace, "machine3")
	m3Status.Metadata().Finalizers().Add(deprecatedControllerName)

	suite.Require().NoError(suite.state.Create(ctx, m1Status, state.WithCreateOwner("MachineStatusController")))
	suite.Require().NoError(suite.state.Create(ctx, m2Status, state.WithCreateOwner("MachineStatusController")))
	suite.Require().NoError(suite.state.Create(ctx, m3Status, state.WithCreateOwner("MachineStatusController")))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("removeMaintenanceConfigPatchFinalizers"))))

	rtestutils.AssertAll(ctx, suite.T(), suite.state, func(
		res *omni.MachineStatus, assert *assert.Assertions,
	) {
		assert.True(res.Metadata().Finalizers().Empty())
	})
}

func (suite *MigrationSuite) TestCompressUncompressMigrations() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	const (
		smallData1 = "Hello world!"
		smallData2 = "Hello Sidero!"
	)

	disabled := specs.WithConfigCompressionOption(specs.CompressionConfig{Enabled: false, MinThreshold: 2048})

	bigEnoughText := strings.Repeat(rand.Text(), 79) // 79 * 26 = 2054
	bigEnoughTextEncoded := must.Value(zstd.NewWriter(
		nil,
		zstd.WithEncoderConcurrency(2),
		zstd.WithWindowSize(1<<18), // 256KB
	))(suite.T()).EncodeAll([]byte(bigEnoughText), nil)

	const ns = resources.DefaultNamespace

	// Ensure we don't compress resources which are not in the running phase
	thirdPatch := omni.NewConfigPatch(ns, "config-patch-3")
	thirdPatch.Metadata().SetPhase(resource.PhaseTearingDown)

	checkMigrations := []func(t *testing.T){
		startMigration(
			ctx,
			suite.T(),
			suite.state,
			omni.NewConfigPatch(ns, "config-patch-0"),
			fillData[*specs.ConfigPatchSpec](bigEnoughText, disabled),
			checkCompressed[string, *specs.ConfigPatchSpec](bigEnoughTextEncoded),
			"",
		),
		startMigration(
			ctx,
			suite.T(),
			suite.state,
			omni.NewConfigPatch(ns, "config-patch-1"),
			fillData[*specs.ConfigPatchSpec](smallData1, disabled),
			checkUncompressed[string, *specs.ConfigPatchSpec](smallData1),
			"",
		),
		startMigration(
			ctx,
			suite.T(),
			suite.state,
			omni.NewConfigPatch(ns, "config-patch-2"),
			fillData[*specs.ConfigPatchSpec](smallData2, disabled),
			checkUncompressed[string, *specs.ConfigPatchSpec](smallData2),
			"",
		),
		startMigration(
			ctx,
			suite.T(),
			suite.state,
			thirdPatch,
			fillData[*specs.ConfigPatchSpec](smallData2, disabled),
			func(t *assert.Assertions, spec *omni.ConfigPatchSpec) {
				uncompressed := spec.Value.GetData()
				t.Equal(smallData2, uncompressed)
				t.Empty(spec.Value.GetCompressedData())
			},
			omnictrl.NewMachineSetController().Name(),
		),
	}

	encodedDifferently := must.Value(zstd.NewWriter(
		nil,
		zstd.WithEncoderConcurrency(1),
		zstd.WithWindowSize(1024),
		zstd.WithEncoderCRC(false),
	))(suite.T()).EncodeAll([]byte(smallData2), nil)

	// Ensure we don't compress resources which are already compressed
	checkMigrations = append(checkMigrations, startMigration(
		ctx,
		suite.T(),
		suite.state,
		omni.NewConfigPatch(ns, "config-patch-4"),
		func(_ *testing.T, spec *omni.ConfigPatchSpec) {
			spec.Value.CompressedData = encodedDifferently //nolint:staticcheck
		},
		func(t *assert.Assertions, spec *omni.ConfigPatchSpec) {
			uncompressed := spec.Value.GetData()
			t.Empty(uncompressed)
			t.Equal(encodedDifferently, spec.Value.GetCompressedData())

			data, err := spec.Value.GetUncompressedData()
			t.NoError(err)

			defer data.Free()

			t.Equal(smallData2, string(data.Data()))
		},
		omnictrl.NewMachineSetController().Name(),
	))

	require.NoError(suite.T(), suite.manager.Run(ctx, migration.WithFilter(filterWith("compressConfigPatches"))))

	for _, check := range checkMigrations {
		check(suite.T())
	}
}

func (suite *MigrationSuite) TestMoveEtcdBackupStatuses() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	newEtcdBackupStatusInOldNamespace := func(id resource.ID) *omni.EtcdBackupStatus {
		return typed.NewResource[omni.EtcdBackupStatusSpec, omni.EtcdBackupStatusExtension](
			resource.NewMetadata(resources.DefaultNamespace, omni.EtcdBackupStatusType, id, resource.VersionUndefined),
			protobuf.NewResourceSpec(&specs.EtcdBackupStatusSpec{}),
		)
	}

	status1 := newEtcdBackupStatusInOldNamespace("status1")

	status1.TypedSpec().Value.Status = specs.EtcdBackupStatusSpec_Running
	status1.TypedSpec().Value.Error = "e"

	suite.Require().NoError(status1.Metadata().SetOwner("test-owner"))

	status2 := newEtcdBackupStatusInOldNamespace("status2")

	overallStatus := typed.NewResource[omni.EtcdBackupOverallStatusSpec, omni.EtcdBackupOverallStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, omni.EtcdBackupOverallStatusType, omni.EtcdBackupOverallStatusID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdBackupOverallStatusSpec{}),
	)

	suite.Require().NoError(overallStatus.Metadata().SetOwner("EtcdBackupOverallStatusController"))

	overallStatus.TypedSpec().Value.ConfigurationError = "e"

	suite.Require().NoError(suite.state.Create(ctx, status1, state.WithCreateOwner(status1.Metadata().Owner())))
	suite.Require().NoError(suite.state.Create(ctx, status2))
	suite.Require().NoError(suite.state.Create(ctx, overallStatus, state.WithCreateOwner(overallStatus.Metadata().Owner())))

	_, err := suite.state.Teardown(ctx, status2.Metadata())
	suite.Require().NoError(err)

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("moveEtcdBackupStatuses"))))

	// Assert that ones in the old namespace are destroyed

	_, err = safe.StateGet[*omni.EtcdBackupStatus](ctx, suite.state, status1.Metadata())
	suite.Require().True(state.IsNotFoundError(err))

	_, err = safe.StateGet[*omni.EtcdBackupStatus](ctx, suite.state, status2.Metadata())
	suite.Require().True(state.IsNotFoundError(err))

	_, err = safe.StateGet[*omni.EtcdBackupOverallStatus](ctx, suite.state, overallStatus.Metadata())
	suite.Require().True(state.IsNotFoundError(err))

	// Assert the new ones

	status1, err = safe.StateGetByID[*omni.EtcdBackupStatus](ctx, suite.state, "status1")
	suite.Require().NoError(err)

	suite.Equal(resources.MetricsNamespace, status1.Metadata().Namespace())
	suite.Equal("test-owner", status1.Metadata().Owner())

	suite.Equal(specs.EtcdBackupStatusSpec_Running, status1.TypedSpec().Value.Status)
	suite.Equal("e", status1.TypedSpec().Value.Error)

	_, err = safe.StateGetByID[*omni.EtcdBackupStatus](ctx, suite.state, "status2")
	suite.Require().True(state.IsNotFoundError(err), "status2 should not exist as it was in tearing down phase")

	overallStatus, err = safe.StateGetByID[*omni.EtcdBackupOverallStatus](ctx, suite.state, omni.EtcdBackupOverallStatusID)
	suite.Require().NoError(err)

	suite.Equal(resources.MetricsNamespace, overallStatus.Metadata().Namespace())
	suite.Equal("EtcdBackupOverallStatusController", overallStatus.Metadata().Owner())
	suite.Equal("e", overallStatus.TypedSpec().Value.ConfigurationError)
}

func (suite *MigrationSuite) TestDropObsoleteConfigPatches() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	cluster := "cluster-a"
	machine := uuid.NewString()

	obsoleteConfigPatch := omni.NewConfigPatch(
		resources.DefaultNamespace,
		"000-23f15aa7-07da-4ba1-8d9a-b42541547ce7-preserve-version-contract",
		pair.MakePair(omni.LabelCluster, cluster),
		pair.MakePair(omni.LabelClusterMachine, machine),
	)

	normalConfigPatch := omni.NewConfigPatch(
		resources.DefaultNamespace,
		"000-23f15aa7-07da-4ba1-8d9a-b42541547cee",
		pair.MakePair(omni.LabelCluster, cluster),
		pair.MakePair(omni.LabelClusterMachine, machine),
	)

	obsoleteConfigPatch.Metadata().Annotations().Set(omni.ConfigPatchDescription, migration.ContractFixConfigPatch)

	suite.Require().NoError(suite.state.Create(ctx, obsoleteConfigPatch))
	suite.Require().NoError(suite.state.Create(ctx, normalConfigPatch))

	version := system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)
	version.TypedSpec().Value.Version = 40

	suite.Require().NoError(suite.state.Create(ctx, version))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("dropObsoleteConfigPatches"))))

	_, err := safe.StateGet[*omni.ConfigPatch](ctx, suite.state, obsoleteConfigPatch.Metadata())
	suite.Assert().True(state.IsNotFoundError(err))

	_, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, normalConfigPatch.Metadata())
	suite.Assert().NoError(err)
}

func (suite *MigrationSuite) TestDropObsoleteConfigPatchesSkipped() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	cluster := "cluster-a"
	machine := uuid.NewString()

	obsoleteConfigPatch := omni.NewConfigPatch(
		resources.DefaultNamespace,
		"000-23f15aa7-07da-4ba1-8d9a-b42541547ce7-preserve-version-contract",
		pair.MakePair(omni.LabelCluster, cluster),
		pair.MakePair(omni.LabelClusterMachine, machine),
	)

	obsoleteConfigPatch.Metadata().Annotations().Set(omni.ConfigPatchDescription, migration.ContractFixConfigPatch)

	suite.Require().NoError(suite.state.Create(ctx, obsoleteConfigPatch))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithMaxVersion(39)))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("dropObsoleteConfigPatches"))))

	_, err := safe.StateGet[*omni.ConfigPatch](ctx, suite.state, obsoleteConfigPatch.Metadata())
	suite.Assert().NoError(err)
}

func (suite *MigrationSuite) TestMarkVersionContract() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), 10*time.Second)
	defer cancel()

	clusterConfigVersion12 := omni.NewClusterConfigVersion(resources.DefaultNamespace, "cluster-12")
	clusterConfigVersion12.TypedSpec().Value.Version = "v1.2.0"

	clusterConfigVersion14 := omni.NewClusterConfigVersion(resources.DefaultNamespace, "cluster-14")
	clusterConfigVersion14.TypedSpec().Value.Version = "v1.4.8"

	// patch expected: apidCheckExtKeyUsage needs to be preserved
	cluster12config1 := omni.NewClusterMachineConfig(resources.DefaultNamespace, "cluster-12-machine-1")
	cluster12config1.Metadata().Labels().Set(omni.LabelCluster, "cluster-12")

	suite.Require().NoError(cluster12config1.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  features:
    apidCheckExtKeyUsage: true
    foo: bar
`)))

	cluster12configStatus1 := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "cluster-12-machine-1")
	cluster12configStatus1.TypedSpec().Value.ClusterMachineConfigVersion = "1" // mark the v1 to be applied

	// no patch expected: it does not have any of the features to be preserved
	cluster12config2 := omni.NewClusterMachineConfig(resources.DefaultNamespace, "cluster-12-machine-2")
	cluster12config2.Metadata().Labels().Set(omni.LabelCluster, "cluster-12")

	suite.Require().NoError(cluster12config2.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  features:
    foo: bar
`)))

	cluster12configStatus2 := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "cluster-12-machine-2")
	cluster12configStatus2.TypedSpec().Value.ClusterMachineConfigVersion = "1" // mark the v1 to be applied

	// patch expected: **both** apidCheckExtKeyUsage and diskQuotaSupport need to be preserved
	cluster12config3 := omni.NewClusterMachineConfig(resources.DefaultNamespace, "cluster-12-machine-3")
	cluster12config3.Metadata().Labels().Set(omni.LabelCluster, "cluster-12")

	suite.Require().NoError(cluster12config3.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  features:
    foo: bar
    apidCheckExtKeyUsage: true
    baz: qux
    diskQuotaSupport: true
    quux: corge
`)))

	cluster12configStatus3 := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "cluster-12-machine-3")
	cluster12configStatus3.TypedSpec().Value.ClusterMachineConfigVersion = "1" // mark the v1 to be applied

	// patch expected: **only** diskQuotaSupport needs to be preserved
	cluster14config1 := omni.NewClusterMachineConfig(resources.DefaultNamespace, "cluster-14-machine-1")
	cluster14config1.Metadata().Labels().Set(omni.LabelCluster, "cluster-14")

	suite.Require().NoError(cluster14config1.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  features:
    foo: bar
    diskQuotaSupport: true
    apidCheckExtKeyUsage: true
`)))

	cluster14configStatus1 := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "cluster-14-machine-1")
	cluster14configStatus1.TypedSpec().Value.ClusterMachineConfigVersion = "1" // mark the v1 to be applied

	// no patch expected: it does not have any of the features to be preserved
	cluster14config2 := omni.NewClusterMachineConfig(resources.DefaultNamespace, "cluster-14-machine-2")
	cluster14config2.Metadata().Labels().Set(omni.LabelCluster, "cluster-14")

	suite.Require().NoError(cluster14config2.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  features:
    foo: bar
`)))

	cluster14configStatus2 := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "cluster-14-machine-2")
	cluster14configStatus2.TypedSpec().Value.ClusterMachineConfigVersion = "1" // mark the v1 to be applied

	// no patch expected, since it will be in tearing down phase
	cluster14config3 := omni.NewClusterMachineConfig(resources.DefaultNamespace, "cluster-14-machine-3")
	cluster14config3.Metadata().Labels().Set(omni.LabelCluster, "cluster-14")

	suite.Require().NoError(cluster14config3.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  features:
    foo: bar
    diskQuotaSupport: true
`)))

	cluster14configStatus3 := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "cluster-14-machine-3")
	cluster14configStatus3.TypedSpec().Value.ClusterMachineConfigVersion = "1" // mark the v1 to be applied

	// no patch expected, since the ClusterMachineConfigStatus.Spec.ClusterMachineConfigVersion will indicate that this version of config was not applied
	cluster14config4 := omni.NewClusterMachineConfig(resources.DefaultNamespace, "cluster-14-machine-4")
	cluster14config4.Metadata().Labels().Set(omni.LabelCluster, "cluster-14")

	suite.Require().NoError(cluster14config4.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  features:
    foo: bar
    diskQuotaSupport: true
`)))

	cluster14configStatus4 := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "cluster-14-machine-4")
	cluster14configStatus4.TypedSpec().Value.ClusterMachineConfigVersion = "0" // mark the v0 to be applied, but v1 was not applied

	machineSetStatusControllerName := omnictrl.NewMachineSetStatusController().ControllerName

	cluster12machine1 := omni.NewClusterMachine(resources.DefaultNamespace, cluster12config1.Metadata().ID())
	cluster12machine2 := omni.NewClusterMachine(resources.DefaultNamespace, cluster12config2.Metadata().ID())
	cluster12machine3 := omni.NewClusterMachine(resources.DefaultNamespace, cluster12config3.Metadata().ID())

	cluster14machine1 := omni.NewClusterMachine(resources.DefaultNamespace, cluster14config1.Metadata().ID())
	cluster14machine2 := omni.NewClusterMachine(resources.DefaultNamespace, cluster14config2.Metadata().ID())
	cluster14machine3 := omni.NewClusterMachine(resources.DefaultNamespace, cluster14config3.Metadata().ID())
	cluster14machine4 := omni.NewClusterMachine(resources.DefaultNamespace, cluster14config4.Metadata().ID())

	suite.Require().NoError(suite.state.Create(ctx, cluster12machine1, state.WithCreateOwner(machineSetStatusControllerName)))
	suite.Require().NoError(suite.state.Create(ctx, cluster12machine2, state.WithCreateOwner(machineSetStatusControllerName)))
	suite.Require().NoError(suite.state.Create(ctx, cluster12machine3, state.WithCreateOwner(machineSetStatusControllerName)))
	suite.Require().NoError(suite.state.Create(ctx, cluster14machine1, state.WithCreateOwner(machineSetStatusControllerName)))
	suite.Require().NoError(suite.state.Create(ctx, cluster14machine2, state.WithCreateOwner(machineSetStatusControllerName)))
	suite.Require().NoError(suite.state.Create(ctx, cluster14machine3, state.WithCreateOwner(machineSetStatusControllerName)))
	suite.Require().NoError(suite.state.Create(ctx, cluster14machine4, state.WithCreateOwner(machineSetStatusControllerName)))

	suite.Require().NoError(suite.state.Create(ctx, clusterConfigVersion12))
	suite.Require().NoError(suite.state.Create(ctx, clusterConfigVersion14))

	suite.Require().NoError(suite.state.Create(ctx, cluster12config1))
	suite.Require().NoError(suite.state.Create(ctx, cluster12config2))
	suite.Require().NoError(suite.state.Create(ctx, cluster12config3))
	suite.Require().NoError(suite.state.Create(ctx, cluster14config1))
	suite.Require().NoError(suite.state.Create(ctx, cluster14config2))
	suite.Require().NoError(suite.state.Create(ctx, cluster14config3))
	suite.Require().NoError(suite.state.Create(ctx, cluster14config4))

	suite.Require().NoError(suite.state.Create(ctx, cluster12configStatus1))
	suite.Require().NoError(suite.state.Create(ctx, cluster12configStatus2))
	suite.Require().NoError(suite.state.Create(ctx, cluster12configStatus3))
	suite.Require().NoError(suite.state.Create(ctx, cluster14configStatus1))
	suite.Require().NoError(suite.state.Create(ctx, cluster14configStatus2))
	suite.Require().NoError(suite.state.Create(ctx, cluster14configStatus3))
	suite.Require().NoError(suite.state.Create(ctx, cluster14configStatus4))

	_, err := suite.state.Teardown(ctx, cluster14config3.Metadata())
	suite.Require().NoError(err)

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("markVersionContract"))))

	clusterMachines, err := safe.StateListAll[*omni.ClusterMachine](ctx, suite.state)
	suite.Require().NoError(err)

	var updatedCount int

	for clusterMachine := range clusterMachines.All() {
		_, preserveApidCheckExtKeyUsage := clusterMachine.Metadata().Annotations().Get(omni.PreserveApidCheckExtKeyUsage)
		_, preserveDiskQuotaSupport := clusterMachine.Metadata().Annotations().Get(omni.PreserveDiskQuotaSupport)

		if preserveDiskQuotaSupport || preserveApidCheckExtKeyUsage {
			updatedCount++
		}

		if clusterMachine.Metadata().ID() == cluster12config1.Metadata().ID() {
			suite.Assert().True(preserveApidCheckExtKeyUsage)
		}

		if clusterMachine.Metadata().ID() == cluster12config3.Metadata().ID() {
			suite.Assert().True(preserveApidCheckExtKeyUsage)
			suite.Assert().True(preserveDiskQuotaSupport)
		}

		if clusterMachine.Metadata().ID() == cluster14config1.Metadata().ID() {
			suite.Assert().True(preserveDiskQuotaSupport)
		}
	}

	suite.Require().Equal(3, updatedCount, "there should be 3 machines updated")
}

func (suite *MigrationSuite) TestDropMachineClassStatusFinalizers() {
	ctx, cancel := context.WithTimeout(suite.T().Context(), time.Second*5)
	defer cancel()

	finalizer := "MachineClassStatusController"

	c1 := omni.NewMachineClass(resources.DefaultNamespace, "c1")
	c2 := omni.NewMachineClass(resources.DefaultNamespace, "c2")
	c2.Metadata().SetPhase(resource.PhaseTearingDown)
	c2.Metadata().Finalizers().Add(finalizer)
	c2.Metadata().Finalizers().Add("some")

	c3 := omni.NewMachineClass(resources.DefaultNamespace, "c3")
	c3.Metadata().Finalizers().Add(finalizer)

	suite.Require().NoError(suite.state.Create(ctx, c1))
	suite.Require().NoError(suite.state.Create(ctx, c2))
	suite.Require().NoError(suite.state.Create(ctx, c3))

	suite.Require().NoError(suite.manager.Run(ctx, migration.WithFilter(filterWith("dropMachineClassStatusFinalizers"))))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{c1.Metadata().ID(), c2.Metadata().ID(), c3.Metadata().ID()},
		func(res *omni.MachineClass, assert *assert.Assertions) {
			assert.False(res.Metadata().Finalizers().Has(finalizer))

			if res.Metadata().ID() == c2.Metadata().ID() {
				assert.False(res.Metadata().Finalizers().Empty())
			}
		},
	)
}

func startMigration[
	R interface {
		generic.ResourceWithRD
		TypedSpec() *protobuf.ResourceSpec[T, S]
	},
	T any,
	S protobuf.Spec[T],
](
	ctx context.Context,
	t *testing.T,
	st state.State,
	res R,
	fill func(t *testing.T, spec *protobuf.ResourceSpec[T, S]),
	check func(t *assert.Assertions, spec *protobuf.ResourceSpec[T, S]),
	owner string,
) func(t *testing.T) {
	fill(t, res.TypedSpec())

	require.NoError(t, st.Create(ctx, res, state.WithCreateOwner(owner)))

	return func(t *testing.T) {
		rtestutils.AssertResource(
			ctx,
			t,
			st,
			res.Metadata().ID(),
			func(found R, assertion *assert.Assertions) { check(assertion, found.TypedSpec()) },
		)
	}
}

func fillData[
	S interface {
		SetUncompressedData(data []byte, opts ...specs.CompressionOption) error
		protobuf.Spec[T]
	},
	T any,
](data string, opts ...specs.CompressionOption) func(t *testing.T, spec *protobuf.ResourceSpec[T, S]) {
	return func(t *testing.T, spec *protobuf.ResourceSpec[T, S]) {
		require.NoError(t, spec.Value.SetUncompressedData([]byte(data), opts...))
	}
}

func checkCompressed[
	D string | []byte,
	S interface {
		GetData() D
		GetCompressedData() []byte
		protobuf.Spec[T]
	},
	T any,
](data []byte) func(t *assert.Assertions, spec *protobuf.ResourceSpec[T, S]) {
	return func(t *assert.Assertions, spec *protobuf.ResourceSpec[T, S]) {
		uncompressed := spec.Value.GetData()
		t.Empty(uncompressed)

		result := spec.Value.GetCompressedData()
		t.NotEmpty(result)
		t.Equalf(data, result, "%x != %x", data, result)
	}
}

func checkUncompressed[
	D string | []byte,
	S interface {
		GetData() D
		GetCompressedData() []byte
		protobuf.Spec[T]
	},
	T any,
](data D) func(t *assert.Assertions, spec *protobuf.ResourceSpec[T, S]) {
	return func(t *assert.Assertions, spec *protobuf.ResourceSpec[T, S]) {
		compressed := spec.Value.GetCompressedData()
		t.Empty(compressed)

		result := spec.Value.GetData()
		t.NotEmpty(result)
		t.Equalf(data, result, "%x != %x", data, result)
	}
}

func TestMigrationSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MigrationSuite))
}

func filterWith(vals ...string) func(string) bool {
	return func(cur string) bool {
		for _, val := range vals {
			if cur == val {
				return true
			}
		}

		return false
	}
}
