// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	talossiderolink "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/machineconfigpatch"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineConfigExtractionControllerSuite struct {
	OmniSuite
}

func (suite *MachineConfigExtractionControllerSuite) registerController(getMachineConfigCh chan *config.MachineConfig) {
	maintenanceClient := &maintenanceClientMock{
		getMachineConfigCh: getMachineConfigCh,
	}

	maintenanceClientFactory := func(context.Context, string) (omni.MaintenanceClient, error) {
		return maintenanceClient, nil
	}

	extractor, err := machineconfigpatch.NewExtractor(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterQController(omni.NewMachineConfigExtractionController(maintenanceClientFactory, extractor)))

	suite.startRuntime()
}

func (suite *MachineConfigExtractionControllerSuite) TestExtractOnce() {
	getMachineConfigCh := make(chan *config.MachineConfig)

	suite.registerController(getMachineConfigCh)

	// machine arrives in maintenance with an embedded TrustedRootsConfig (preserve) and a SideroLinkConfig (drop)
	trustedRoots := security.NewTrustedRootsConfigV1Alpha1()
	trustedRoots.MetaName = "my-enterprise-ca"
	trustedRoots.Certificates = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	u, err := url.Parse("https://siderolink.example.org")
	suite.Require().NoError(err)

	siderolinkDoc := talossiderolink.NewConfigV1Alpha1()
	siderolinkDoc.APIUrlConfig.URL = u

	observed, err := container.New(trustedRoots, siderolinkDoc)
	suite.Require().NoError(err)

	machineStatus := omnires.NewMachineStatus("extract-machine")
	machineStatus.TypedSpec().Value.Maintenance = true
	machineStatus.TypedSpec().Value.ManagementAddress = "extract-address"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	// serve the observed config to the controller
	select {
	case getMachineConfigCh <- config.NewMachineConfig(observed):
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout serving machine config")
	}

	// the extracted patch preserves the partial document, drops the connection document, and is machine-scoped
	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{"000-preserved-machine-config-extract-machine"},
		func(patch *omnires.ConfigPatch, assert *assert.Assertions) {
			machineID, ok := patch.Metadata().Labels().Get(omnires.LabelMachine)
			assert.True(ok)
			assert.Equal("extract-machine", machineID)

			buffer, bufErr := patch.TypedSpec().Value.GetUncompressedData()
			assert.NoError(bufErr)

			data := string(buffer.Data())
			buffer.Free()

			assert.Contains(data, "TrustedRootsConfig")
			assert.Contains(data, "my-enterprise-ca")
			assert.NotContains(data, "SideroLinkConfig")
		})

	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{"extract-machine"},
		func(status *omnires.MachineConfigExtractionStatus, assert *assert.Assertions) {
			assert.True(status.TypedSpec().Value.Initialized)
		})
}

func (suite *MachineConfigExtractionControllerSuite) TestEmptyConfigStillInitializes() {
	getMachineConfigCh := make(chan *config.MachineConfig)

	suite.registerController(getMachineConfigCh)

	machineStatus := omnires.NewMachineStatus("empty-machine")
	machineStatus.TypedSpec().Value.Maintenance = true
	machineStatus.TypedSpec().Value.ManagementAddress = "empty-address"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	// machine has no config at all
	select {
	case getMachineConfigCh <- nil:
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout serving machine config")
	}

	// still marked initialized, but no patch is created
	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{"empty-machine"},
		func(status *omnires.MachineConfigExtractionStatus, assert *assert.Assertions) {
			assert.True(status.TypedSpec().Value.Initialized)
		})

	_, err := suite.state.Get(suite.ctx, omnires.NewConfigPatch("000-preserved-machine-config-empty-machine").Metadata())
	suite.Require().Error(err)
}

func (suite *MachineConfigExtractionControllerSuite) TestForbiddenConfigReportsError() {
	getMachineConfigCh := make(chan *config.MachineConfig)

	suite.registerController(getMachineConfigCh)

	// a config that carries machine token / PKI is not a valid config patch and cannot be preserved as one
	observed, err := configloader.NewFromBytes([]byte(`version: v1alpha1
machine:
    type: worker
    token: aaaaaa.bbbbbbbbbbbbbbbb
    ca:
        crt: Zm9v
        key: YmFy
cluster: {}
`))
	suite.Require().NoError(err)

	machineStatus := omnires.NewMachineStatus("forbidden-machine")
	machineStatus.TypedSpec().Value.Maintenance = true
	machineStatus.TypedSpec().Value.ManagementAddress = "forbidden-address"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	select {
	case getMachineConfigCh <- config.NewMachineConfig(observed):
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout serving machine config")
	}

	// initialized, with the reason recorded, and no patch created
	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{"forbidden-machine"},
		func(status *omnires.MachineConfigExtractionStatus, assert *assert.Assertions) {
			assert.True(status.TypedSpec().Value.Initialized)
			assert.NotEmpty(status.TypedSpec().Value.Error)
		})

	_, err = suite.state.Get(suite.ctx, omnires.NewConfigPatch("000-preserved-machine-config-forbidden-machine").Metadata())
	suite.Require().Error(err)
}

func TestMachineConfigExtractionControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineConfigExtractionControllerSuite))
}
