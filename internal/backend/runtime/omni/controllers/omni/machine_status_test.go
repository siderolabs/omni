// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/image-factory/pkg/schematic"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

const testSchematicKernelArgs = "key1=val1 key2=val2"

type imageFactoryClientMock struct{}

func (i *imageFactoryClientMock) EnsureSchematic(_ context.Context, sch schematic.Schematic) (imagefactory.EnsuredSchematic, error) {
	fullID, err := sch.ID()
	if err != nil {
		return imagefactory.EnsuredSchematic{}, err
	}

	plainSchematic := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: sch.Customization.SystemExtensions.OfficialExtensions,
			},
		},
	}

	plainID, err := plainSchematic.ID()
	if err != nil {
		return imagefactory.EnsuredSchematic{}, err
	}

	return imagefactory.EnsuredSchematic{
		FullID:  fullID,
		PlainID: plainID,
	}, nil
}

type MachineStatusSuite struct {
	OmniSuite
}

func (suite *MachineStatusSuite) setup() {
	suite.startRuntime()

	suite.Require().NoError(
		suite.machineService.state.Create(suite.ctx, runtime.NewSecurityStateSpec(runtime.NamespaceName)),
	)

	params := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
	params.TypedSpec().Value.Args = testSchematicKernelArgs

	suite.Require().NoError(suite.state.Create(suite.ctx, params))

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineStatusController(&imageFactoryClientMock{})))
}

const testID = "testID"

func (suite *MachineStatusSuite) TestMachineConnected() {
	suite.setup()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	// given
	machine := omni.NewMachine(resources.DefaultNamespace, testID)
	machine.TypedSpec().Value.Connected = true

	// when
	suite.Assert().NoError(suite.state.Create(ctx, machine))

	// then
	rtestutils.AssertResource(ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
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

	rtestutils.AssertResource(ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
		statusVal := status.TypedSpec().Value

		assert.Falsef(statusVal.Connected, "should not be connected anymore")

		_, ok := status.Metadata().Labels().Get(omni.MachineStatusLabelConnected)
		assert.Falsef(ok, "connected label should not be set anymore")
	})
}

func (suite *MachineStatusSuite) TestMachineReportingEvents() {
	suite.setup()

	// given
	machine := omni.NewMachine(resources.DefaultNamespace, testID)

	machineStatusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, testID)
	machineStatusSnapshot.TypedSpec().Value = &specs.MachineStatusSnapshotSpec{
		MachineStatus: &machineapi.MachineStatusEvent{},
	}

	// when
	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatusSnapshot))

	// then
	rtestutils.AssertResource(suite.ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
		_, ok := status.Metadata().Labels().Get(omni.MachineStatusLabelReportingEvents)
		assert.Truef(ok, "reporting-events label not set")
	})

	rtestutils.Destroy[*omni.MachineStatusSnapshot](suite.ctx, suite.T(), suite.state, []string{testID})

	rtestutils.AssertResource(suite.ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
		_, ok := status.Metadata().Labels().Get(omni.MachineStatusLabelReportingEvents)
		assert.Falsef(ok, "reporting-events label should not be set anymore")
	})
}

func (suite *MachineStatusSuite) TestMachineUserLabels() {
	suite.setup()

	machine := omni.NewMachine(resources.DefaultNamespace, testID)
	spec := machine.TypedSpec().Value

	spec.Connected = true
	spec.ManagementAddress = suite.socketConnectionString

	metaKey := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.LabelsMeta))

	labels := meta.ImageLabels{
		Labels: map[string]string{
			"label1": "value1",
		},
	}

	d, err := labels.Encode()
	suite.Require().NoError(err)

	metaKey.TypedSpec().Value = string(d)

	suite.Require().NoError(suite.machineService.state.Create(suite.ctx, metaKey))

	machineStatusSnapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, testID)
	machineStatusSnapshot.TypedSpec().Value = &specs.MachineStatusSnapshotSpec{
		MachineStatus: &machineapi.MachineStatusEvent{},
	}

	suite.Assert().NoError(suite.state.Create(suite.ctx, machine))
	suite.Assert().NoError(suite.state.Create(suite.ctx, machineStatusSnapshot))

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	// first let's see if initial labels get populated in the resource spec

	rtestutils.AssertResource(ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
		assert.NotNilf(status.TypedSpec().Value.ImageLabels, "initial labels not loaded")

		val, ok := status.Metadata().Labels().Get("label1")
		assert.Truef(ok, "label1 is not set in the initial labels")
		assert.EqualValues("value1", val)
	})

	// now create user labels and see how it merges initial and user labels

	machineLabels := omni.NewMachineLabels(resources.DefaultNamespace, testID)
	machineLabels.Metadata().Labels().Set("test", "")

	suite.Assert().NoError(suite.state.Create(suite.ctx, machineLabels))

	rtestutils.AssertResource(ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("label1")
		assert.Truef(ok, "label1 is not set in the initial labels")
		assert.EqualValues("value1", val)

		val, ok = status.Metadata().Labels().Get("test")
		assert.Truef(ok, "label1 is not set in the initial labels")
		assert.EqualValues("", val)
	})

	// overwrite initial label value

	_, err = safe.StateUpdateWithConflicts[*omni.MachineLabels](ctx, suite.state, machineLabels.Metadata(), func(ml *omni.MachineLabels) error {
		ml.Metadata().Labels().Set("label1", "gasp")

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResource(ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("label1")
		assert.Truef(ok, "label1 doesn't exist")
		assert.EqualValues("gasp", val)
	})

	// reverts back to initial when the machine labels resource gets removed

	rtestutils.Destroy[*omni.MachineLabels](suite.ctx, suite.T(), suite.state, []string{testID})

	rtestutils.AssertResource(ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("label1")
		assert.Truef(ok, "label1 doesn't exist")
		assert.EqualValues("value1", val)
	})

	machineLabels.Metadata().Labels().Set("label2", "aaa")

	suite.Assert().NoError(suite.state.Create(suite.ctx, machineLabels))

	_, err = safe.StateUpdateWithConflicts(ctx, suite.machineService.state, metaKey.Metadata(), func(res *runtime.MetaKey) error {
		labels.Labels["label1"] = "updated"
		labels.Labels["label2"] = "override"

		d, err = labels.Encode()
		if err != nil {
			return err
		}

		res.TypedSpec().Value = string(d)

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResource(ctx, suite.T(), suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
		val, ok := status.Metadata().Labels().Get("label1")
		assert.Truef(ok, "label1 doesn't exist")
		assert.EqualValues("updated", val)

		val, ok = status.Metadata().Labels().Get("label2")
		assert.Truef(ok, "label2 doesn't exist")
		assert.EqualValues("aaa", val)
	})
}

func (suite *MachineStatusSuite) TestMachineSchematic() {
	suite.setup()

	kernelArgs := strings.Split(testSchematicKernelArgs, " ")

	vanillaID, err := pointer.To(schematic.Schematic{
		Customization: schematic.Customization{
			ExtraKernelArgs: kernelArgs,
		},
	}).ID()
	suite.Require().NoError(err)

	machine := omni.NewMachine(resources.DefaultNamespace, testID)
	spec := machine.TypedSpec().Value

	spec.Connected = true
	spec.ManagementAddress = suite.socketConnectionString

	suite.Require().NoError(suite.state.Create(suite.ctx, machine))

	defaultSchematic, err := (&schematic.Schematic{}).ID()
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
				Id:               "7d79f1ce28d7e6c099bc89ccf02238fb574165eb4834c2abf2a61eab998d4dc6",
				InitialSchematic: "full-id",
				Extensions: []string{
					"siderolabs/gvisor",
					"siderolabs/hello-world-service",
					"siderolabs/mdadm",
				},
				FullId: "full-id",
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
				Invalid: true,
			},
		},
		{
			name: "vanilla autodetect",
			expected: &specs.MachineStatusSpec_Schematic{
				Id:               defaultSchematic,
				InitialSchematic: vanillaID,
				FullId:           vanillaID,
				KernelArgs:       kernelArgs,
			},
		},
	} {
		suite.T().Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
			defer cancel()

			rtestutils.DestroyAll[*runtime.ExtensionStatus](ctx, t, suite.machineService.state)

			for _, spec := range tt.extensions {
				res := runtime.NewExtensionStatus(runtime.NamespaceName, spec.Metadata.Description)

				res.TypedSpec().Image = spec.Image
				res.TypedSpec().Metadata = spec.Metadata

				suite.Require().NoError(suite.machineService.state.Create(ctx, res))
			}

			rtestutils.AssertResource(ctx, t, suite.state, testID, func(status *omni.MachineStatus, assert *assert.Assertions) {
				assert.EqualValues(tt.expected, status.TypedSpec().Value.Schematic)
			})
		})
	}
}

func TestMachineStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineStatusSuite))
}
