// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/image-factory/pkg/schematic"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

type MachineStatusSuite struct {
	OmniSuite
}

func (suite *MachineStatusSuite) setup() {
	suite.startRuntime()

	suite.Require().NoError(
		suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)),
	)

	apiConfig := siderolink.NewAPIConfig()
	apiConfig.TypedSpec().Value.LogsPort = 8092
	apiConfig.TypedSpec().Value.EventsPort = 8091
	apiConfig.TypedSpec().Value.MachineApiAdvertisedUrl = "grpc://127.0.0.1:8090"

	suite.Require().NoError(suite.state.Create(suite.ctx, apiConfig))

	createJoinParams(suite.ctx, suite.state, suite.T())

	logger := zaptest.NewLogger(suite.T())

	exraKernelArgsInitializer, err := kernelargs.NewInitializer(suite.state, logger)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineStatusController(testutils.NewFactoryClientSet(), exraKernelArgsInitializer)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineJoinConfigController()))
	suite.Require().NoError(suite.runtime.RegisterQController(newMockJoinTokenUsageController[*omni.Machine]()))
}

const testID = "testID"

func (suite *MachineStatusSuite) TestMachineConnected() {
	suite.setup()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	// given
	machine := omni.NewMachine(testID)
	machine.TypedSpec().Value.Connected = true

	// when
	suite.Assert().NoError(suite.state.Create(ctx, machine))

	// then
	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		statusVal := status.TypedSpec().Value

		suite.Truef(statusVal.Connected, "not connected")

		_, ok := status.Metadata().Labels().Get(omni.MachineStatusLabelConnected)
		assert.Truef(ok, "connected label not set")
	})

	// check that the connected label is removed again, if the machine becomes disconnected.
	_, err := safe.StateUpdateWithConflicts(ctx, suite.state,
		resource.NewMetadata(resources.DefaultNamespace, omni.MachineType, testID, resource.VersionUndefined), func(res *omni.Machine) error {
			res.TypedSpec().Value.Connected = false

			return nil
		})
	suite.Assert().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		statusVal := status.TypedSpec().Value

		assert.Falsef(statusVal.Connected, "should not be connected anymore")

		_, ok := status.Metadata().Labels().Get(omni.MachineStatusLabelConnected)
		assert.Falsef(ok, "connected label should not be set anymore")
	})
}

func (suite *MachineStatusSuite) TestMachineReportingEvents() {
	suite.setup()

	// given
	machine := omni.NewMachine(testID)

	machineStatusSnapshot := omni.NewMachineStatusSnapshot(testID)
	machineStatusSnapshot.TypedSpec().Value = &specs.MachineStatusSnapshotSpec{
		MachineStatus: &machineapi.MachineStatusEvent{},
	}

	// when
	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatusSnapshot))

	// then
	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		_, ok := status.Metadata().Labels().Get(omni.MachineStatusLabelReportingEvents)
		assert.Truef(ok, "reporting-events label not set")
	})

	rtestutils.Destroy[*omni.MachineStatusSnapshot](suite.ctx, suite.T(), suite.state, []string{testID})

	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		_, ok := status.Metadata().Labels().Get(omni.MachineStatusLabelReportingEvents)
		assert.Falsef(ok, "reporting-events label should not be set anymore")
	})
}

func (suite *MachineStatusSuite) TestClusterRelation() {
	suite.setup()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	machine := omni.NewMachine(testID)
	machine.TypedSpec().Value.Connected = true

	machineSet := omni.NewMachineSet("ms1")
	machineSet.Metadata().Labels().Set(omni.LabelCluster, "cluster1")
	machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	machineSetNode := omni.NewMachineSetNode(testID, machineSet)

	suite.Assert().NoError(suite.state.Create(ctx, machine))
	suite.Assert().NoError(suite.state.Create(ctx, machineSet))
	suite.Assert().NoError(suite.state.Create(ctx, machineSetNode))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		cluster, ok := status.Metadata().Labels().Get(omni.LabelCluster)
		assert.True(ok)
		assert.Equal("cluster1", cluster)
	})

	rtestutils.DestroyAll[*omni.MachineSetNode](ctx, suite.T(), suite.state)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		_, available := status.Metadata().Labels().Get(omni.MachineStatusLabelAvailable)
		assert.True(available)

		_, clusterSet := status.Metadata().Labels().Get(omni.LabelCluster)
		assert.False(clusterSet)
	})

	suite.Assert().NoError(suite.state.Create(ctx, machineSetNode))

	rtestutils.DestroyAll[*omni.Machine](ctx, suite.T(), suite.state)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(node *omni.MachineSetNode, assert *assert.Assertions) {
		assert.True(node.Metadata().Finalizers().Empty())
	})
}

func (suite *MachineStatusSuite) TestMachineUserLabels() {
	suite.setup()

	machine := omni.NewMachine(testID)
	spec := machine.TypedSpec().Value

	spec.Connected = true
	spec.ManagementAddress = suite.socketConnectionString

	metaKey := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.LabelsMeta))

	imageLabels := meta.ImageLabels{
		Labels: map[string]string{
			"imageLabel1": "imageLabelVal1",
		},
	}

	imageLabelsBytes, err := imageLabels.Encode()
	suite.Require().NoError(err)

	metaKey.TypedSpec().Value = string(imageLabelsBytes)

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, metaKey))

	machineStatusSnapshot := omni.NewMachineStatusSnapshot(testID)
	machineStatusSnapshot.TypedSpec().Value = &specs.MachineStatusSnapshotSpec{
		MachineStatus: &machineapi.MachineStatusEvent{},
	}

	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatusSnapshot))

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	// first let's see if initial labels get populated in the resource spec

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		assert.NotNilf(status.TypedSpec().Value.ImageLabels, "initial labels not loaded")

		imageLabel1Val, ok := status.Metadata().Labels().Get("imageLabel1")
		assert.Truef(ok, "imageLabel1 is not set in the initial labels")
		assert.EqualValues("imageLabelVal1", imageLabel1Val)
	})

	// now create user labels and see how it merges initial and user labels

	machineLabels := omni.NewMachineLabels(testID)
	machineLabels.Metadata().Labels().Set("test", "")
	machineLabels.Metadata().Labels().Set("duplicateInPlatformTagsAndUserLabels", "valueShouldStay")

	suite.Assert().NoError(suite.state.Create(suite.ctx, machineLabels))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("imageLabel1")
		assert.Truef(ok, "imageLabel1 is not set")
		assert.EqualValues("imageLabelVal1", val)

		val, ok = status.Metadata().Labels().Get("test")
		assert.Truef(ok, "test label is not set")
		assert.EqualValues("", val)

		val, ok = status.Metadata().Labels().Get("duplicateInPlatformTagsAndUserLabels")
		assert.Truef(ok, "duplicateInPlatformTagsAndUserLabels label is not set")
		assert.EqualValues("valueShouldStay", val)
	})

	// create PlatformMetadata with non-empty Tags and see if they are propagated through MachineLabels

	platformMetadata := runtime.NewPlatformMetadataSpec(runtime.NamespaceName, testID)
	platformMetadata.TypedSpec().Tags = map[string]string{
		"platformMetadataTag1":                 "platformMetadataValue",
		"duplicateInPlatformTagsAndUserLabels": "shouldNotOverwriteUserValue",
	}
	suite.Assert().NoError(suite.machineService.state.Create(ctx, platformMetadata))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineLabels *omni.MachineLabels, assert *assert.Assertions) {
		_, initialized := machineLabels.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(initialized, "PlatformTagLabelsInitialized annotation must be set on MachineLabels")
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		_, initialized := status.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(initialized, "PlatformTagLabelsInitialized annotation must be set on MachineStatus")

		assert.NotNilf(status.TypedSpec().Value.ImageLabels, "initial labels not loaded")

		imageLabel1Val, ok := status.Metadata().Labels().Get("imageLabel1")
		assert.Truef(ok, "imageLabel1 is not set")
		assert.EqualValues("imageLabelVal1", imageLabel1Val)

		platformMetadataVal, ok := status.Metadata().Labels().Get("platformMetadataTag1")
		assert.Truef(ok, "platformMetadataTag is not set")
		assert.EqualValues("platformMetadataValue", platformMetadataVal)

		duplicateInPlatformTagsAndUserLabelsVal, ok := status.Metadata().Labels().Get("duplicateInPlatformTagsAndUserLabels")
		assert.Truef(ok, "duplicateInPlatformTagsAndUserLabels is not set")
		assert.EqualValues("valueShouldStay", duplicateInPlatformTagsAndUserLabelsVal)
	})

	// overwrite image label value via MachineLabels
	_, err = safe.StateUpdateWithConflicts(
		ctx, suite.state,
		omni.NewMachineLabels(testID).Metadata(),
		func(machineLabels *omni.MachineLabels) error {
			machineLabels.Metadata().Labels().Set("imageLabel1", "userOverride")

			return nil
		},
		state.WithUpdateOwner(""),
	)
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("imageLabel1")
		assert.Truef(ok, "imageLabel1 doesn't exist")
		assert.EqualValues("userOverride", val)

		platformMetadataVal, ok := status.Metadata().Labels().Get("platformMetadataTag1")
		assert.Truef(ok, "platformMetadataTag doesn't exist")
		assert.EqualValues("platformMetadataValue", platformMetadataVal)
	})

	// labels created from PlatformMetadata tags should be removable by the user
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state,
		omni.NewMachineLabels(testID).Metadata(),
		func(machineLabels *omni.MachineLabels) error {
			machineLabels.Metadata().Labels().Delete("platformMetadataTag1")

			return nil
		}, state.WithUpdateOwner(""))
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("imageLabel1")
		assert.Truef(ok, "imageLabel1 doesn't exist")
		assert.EqualValues("userOverride", val)

		_, ok = status.Metadata().Labels().Get("platformMetadataTag1")
		assert.Falsef(ok, "platformMetadataTag1 should not be set after it's removed from MachineLabels")
	})

	// Trigger MachineStatusController reconciliation through an Info event.
	// Ensures platform tag labels are not re-added after being removed.
	_, err = safe.StateUpdateWithConflicts(ctx, suite.machineService.state, platformMetadata.Metadata(),
		func(res *runtime.PlatformMetadata) error {
			return nil
		})
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("imageLabel1")
		assert.Truef(ok, "imageLabel1 doesn't exist")
		assert.EqualValues("userOverride", val)

		_, ok = status.Metadata().Labels().Get("platformMetadataTag1")
		assert.Falsef(ok, "platformMetadataTag1 should not be set after removal and reconciliation")
	})

	// MachineLabels value takes precedence over image labels for the same key.
	// machineLabels was re-created by the controller, so update rather than create.
	_, err = safe.StateUpdateWithConflicts(
		ctx, suite.state,
		omni.NewMachineLabels(testID).Metadata(),
		func(machineLabels *omni.MachineLabels) error {
			machineLabels.Metadata().Labels().Set("imageLabel2", "aaa")

			return nil
		},
		state.WithUpdateOwner(""),
	)
	suite.Require().NoError(err)

	_, err = safe.StateUpdateWithConflicts(ctx, suite.machineService.state, metaKey.Metadata(), func(res *runtime.MetaKey) error {
		imageLabels.Labels["imageLabel1"] = "updated"
		imageLabels.Labels["imageLabel2"] = "override"

		encoded, encodeErr := imageLabels.Encode()
		if encodeErr != nil {
			return encodeErr
		}

		res.TypedSpec().Value = string(encoded)

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("imageLabel1")
		assert.Truef(ok, "imageLabel1 doesn't exist")
		assert.EqualValues("userOverride", val) // user override in MachineLabels takes precedence over image label update

		val, ok = status.Metadata().Labels().Get("imageLabel2")
		assert.Truef(ok, "imageLabel2 doesn't exist")
		assert.EqualValues("aaa", val)
	})
}

// TestMachineUserLabelsEmptyPlatformTags verifies that when PlatformMetadata carries no tags,
// the PlatformTagLabelsInitialized annotation is still stamped on MachineLabels and pre-existing
// user labels are left untouched across repeated reconciliations. It also verifies that tags
// subsequently added to PlatformMetadata are not propagated to MachineLabels, because the
// annotation already marks the one-time bootstrap as done.
func (suite *MachineStatusSuite) TestMachineUserLabelsEmptyPlatformTags() {
	suite.setup()

	machine := omni.NewMachine(testID)
	spec := machine.TypedSpec().Value

	spec.Connected = true
	spec.ManagementAddress = suite.socketConnectionString

	machineStatusSnapshot := omni.NewMachineStatusSnapshot(testID)
	machineStatusSnapshot.TypedSpec().Value = &specs.MachineStatusSnapshotSpec{
		MachineStatus: &machineapi.MachineStatusEvent{},
	}

	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatusSnapshot))

	machineLabels := omni.NewMachineLabels(testID)
	machineLabels.Metadata().Labels().Set("userLabel1", "userValue1")
	machineLabels.Metadata().Labels().Set("userLabel2", "userValue2")

	suite.Assert().NoError(suite.state.Create(suite.ctx, machineLabels))

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	// create PlatformMetadata with empty Tags
	platformMetadata := runtime.NewPlatformMetadataSpec(runtime.NamespaceName, testID)
	suite.Assert().NoError(suite.machineService.state.Create(ctx, platformMetadata))

	// the annotation must be set even when Tags is empty, and no extra labels should appear
	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineLabels *omni.MachineLabels, assert *assert.Assertions) {
		_, initialized := machineLabels.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(initialized, "PlatformTagLabelsInitialized annotation must be set on MachineLabels even when platform tags are empty")
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
		_, initialized := machineStatus.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(initialized, "PlatformTagLabelsInitialized annotation must be set on MachineStatus even when platform tags are empty")

		val, ok := machineStatus.Metadata().Labels().Get("userLabel1")
		assert.Truef(ok, "userLabel1 should still be set")
		assert.EqualValues("userValue1", val)

		val, ok = machineStatus.Metadata().Labels().Get("userLabel2")
		assert.Truef(ok, "userLabel2 should still be set")
		assert.EqualValues("userValue2", val)
	})

	// update PlatformMetadata to now carry non-empty tags — because the annotation is already set,
	// these tags must NOT be bootstrapped into MachineLabels
	_, err := safe.StateUpdateWithConflicts(ctx, suite.machineService.state, platformMetadata.Metadata(),
		func(res *runtime.PlatformMetadata) error {
			res.TypedSpec().Tags = map[string]string{
				"lateTag1": "lateValue1",
			}

			return nil
		})
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineLabels *omni.MachineLabels, assert *assert.Assertions) {
		_, initialized := machineLabels.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(initialized, "PlatformTagLabelsInitialized annotation must remain set on MachineLabels after re-reconciliation")
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
		_, initialized := machineStatus.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(initialized, "PlatformTagLabelsInitialized annotation must remain set on MachineStatus after re-reconciliation")

		val, ok := machineStatus.Metadata().Labels().Get("userLabel1")
		assert.Truef(ok, "userLabel1 should still be set after re-reconciliation")
		assert.EqualValues("userValue1", val)

		val, ok = machineStatus.Metadata().Labels().Get("userLabel2")
		assert.Truef(ok, "userLabel2 should still be set after re-reconciliation")
		assert.EqualValues("userValue2", val)

		_, ok = machineStatus.Metadata().Labels().Get("lateTag1")
		assert.Falsef(ok, "lateTag1 must not be added: bootstrap already ran when tags were empty")
	})
}

func (suite *MachineStatusSuite) TestPlatformTagLabelsSetEventuallyConsistent() {
	suite.setup()

	machine := omni.NewMachine(testID)
	spec := machine.TypedSpec().Value
	spec.Connected = true
	spec.ManagementAddress = suite.socketConnectionString

	machineStatusSnapshot := omni.NewMachineStatusSnapshot(testID)
	machineStatusSnapshot.TypedSpec().Value = &specs.MachineStatusSnapshotSpec{
		MachineStatus: &machineapi.MachineStatusEvent{},
	}

	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatusSnapshot))

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	platformMetadata := runtime.NewPlatformMetadataSpec(runtime.NamespaceName, testID)
	platformMetadata.TypedSpec().Tags = map[string]string{"ec2Tag": "ec2Value"}
	suite.Assert().NoError(suite.machineService.state.Create(ctx, platformMetadata))

	// Wait for the normal bootstrap path: both resources must have the annotation.
	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineLabels *omni.MachineLabels, assert *assert.Assertions) {
		_, ok := machineLabels.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(ok, "MachineLabels must have PlatformTagLabelsInitialized after bootstrap")
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
		_, ok := machineStatus.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(ok, "MachineStatus must have PlatformTagLabelsInitialized after bootstrap")
	})

	// Case 1: MachineStatus retains the annotation; MachineLabels loses it.
	// Simulates a partial failure where the MachineStatus write committed but the subsequent
	// MachineLabels write did not. Removing the annotation from MachineLabels also triggers
	// re-reconciliation (it is a mapped input), so no explicit trigger is needed.
	_, err := safe.StateUpdateWithConflicts(ctx, suite.state,
		omni.NewMachineLabels(testID).Metadata(),
		func(ml *omni.MachineLabels) error {
			ml.Metadata().Annotations().Delete(omni.PlatformTagLabelsInitialized)

			return nil
		}, state.WithUpdateOwner(""))
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineLabels *omni.MachineLabels, assert *assert.Assertions) {
		_, ok := machineLabels.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(ok, "PlatformTagLabelsInitialized must be restored on MachineLabels when MachineStatus already has it")
	})

	// Case 2: MachineLabels retains the annotation; MachineStatus loses it.
	// Simulates a partial failure where the inner MachineLabels WriterModify committed but the
	// outer MachineStatus WriterModify failed. MachineStatus is not a regular mapped input, so
	// a direct annotation removal does not trigger reconciliation on its own; we drive the next
	// cycle via a no-op MachineLabels update.
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state,
		omni.NewMachineStatus(testID).Metadata(),
		func(status *omni.MachineStatus) error {
			status.Metadata().Annotations().Delete(omni.PlatformTagLabelsInitialized)

			return nil
		}, state.WithUpdateOwner(omnictrl.MachineStatusControllerName))
	suite.Require().NoError(err)

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state,
		omni.NewMachineLabels(testID).Metadata(),
		func(ml *omni.MachineLabels) error { return nil },
		state.WithUpdateOwner(""))
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{testID}, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
		_, ok := machineStatus.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized)
		assert.True(ok, "PlatformTagLabelsInitialized must be restored on MachineStatus when MachineLabels already has it")
	})
}

func (suite *MachineStatusSuite) TestMachineSchematic() {
	suite.setup()

	kernelArgs := []string{
		"siderolink.api=grpc://127.0.0.1:8090?jointoken=testtoken",
		"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
		"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
	}

	vanillaSchematic := schematic.Schematic{
		Customization: schematic.Customization{
			ExtraKernelArgs: kernelArgs,
		},
	}

	vanillaID, err := vanillaSchematic.ID()
	suite.Require().NoError(err)

	vanillaRaw, err := vanillaSchematic.Marshal()
	suite.Require().NoError(err)

	for _, tt := range []struct {
		expected   *specs.MachineStatusSpec_Schematic
		name       string
		extensions []*runtime.ExtensionStatusSpec
	}{
		{
			name: "extensions",
			extensions: []*runtime.ExtensionStatusSpec{
				{
					Metadata: extensions.Metadata{
						Name:        "gvisor",
						Description: "0",
					},
				},
				{
					Metadata: extensions.Metadata{
						Name:        "hello-world-service",
						Description: "1",
					},
				},
				{
					Metadata: extensions.Metadata{
						Name:        "mdadm",
						Description: "2",
					},
				},
				{
					Metadata: extensions.Metadata{
						Name:        constants.SchematicIDExtensionName,
						Description: "3",
						Version:     "full-id",
					},
				},
			},
			expected: &specs.MachineStatusSpec_Schematic{
				InitialSchematic: "full-id",
				Extensions:       []string{"siderolabs/gvisor", "siderolabs/hello-world-service", "siderolabs/mdadm"},
				FullId:           "full-id",
				InitialState: &specs.MachineStatusSpec_Schematic_InitialState{
					Extensions: []string{"siderolabs/gvisor", "siderolabs/hello-world-service", "siderolabs/mdadm"},
				},
			},
		},
		{
			name: "invalid",
			extensions: []*runtime.ExtensionStatusSpec{
				{
					Metadata: extensions.Metadata{
						Name:        "unknown",
						Version:     "1",
						Description: "unknown",
					},
				},
			},
			expected: &specs.MachineStatusSpec_Schematic{
				Invalid:      true,
				InitialState: &specs.MachineStatusSpec_Schematic_InitialState{},
			},
		},
		{
			name: "vanilla autodetect",
			expected: &specs.MachineStatusSpec_Schematic{
				InitialSchematic: vanillaID,
				FullId:           vanillaID,
				Raw:              string(vanillaRaw),
				KernelArgs:       kernelArgs,
				InitialState:     &specs.MachineStatusSpec_Schematic_InitialState{},
			},
		},
		{
			name: "agent mode empty list",
			extensions: []*runtime.ExtensionStatusSpec{
				{
					Metadata: extensions.Metadata{
						Name:        constants.SchematicIDExtensionName,
						Description: "0",
						Version:     "full-id",
					},
				},
				{
					Metadata: extensions.Metadata{
						Name:        "metal-agent",
						Description: "1",
					},
				},
				{
					Metadata: extensions.Metadata{
						Name:        "hello-world-service",
						Description: "2",
					},
				},
			},
			expected: &specs.MachineStatusSpec_Schematic{
				InAgentMode: true,
			},
		},
	} {
		suite.T().Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
			defer cancel()

			id := "test-" + tt.name

			machine := omni.NewMachine(id)
			spec := machine.TypedSpec().Value

			spec.Connected = true
			spec.ManagementAddress = suite.socketConnectionString

			suite.Require().NoError(suite.state.Create(suite.ctx, machine))

			rtestutils.DestroyAll[*runtime.ExtensionStatus](ctx, t, suite.machineService.state)

			for _, spec := range tt.extensions {
				res := runtime.NewExtensionStatus(runtime.NamespaceName, spec.Metadata.Description)

				res.TypedSpec().Image = spec.Image
				res.TypedSpec().Metadata = spec.Metadata

				suite.Require().NoError(suite.machineService.state.Create(ctx, res))
			}

			rtestutils.AssertResources(ctx, t, suite.state, []string{id}, func(status *omni.MachineStatus, assert *assert.Assertions) {
				assert.EqualValues(tt.expected, status.TypedSpec().Value.Schematic)
			})
		})
	}
}

func TestMachineStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineStatusSuite))
}
