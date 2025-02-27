// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MaintenanceConfigStatusControllerSuite struct {
	OmniSuite
}

func (suite *MachineStatusSnapshotControllerSuite) TestMaintenanceConfigStatus() {
	// Prepare the mock maintenance client factory
	getMachineConfigCh := make(chan *config.MachineConfig)
	mgmtAddressCh := make(chan string)
	applyConfigReqCh := make(chan *machine.ApplyConfigurationRequest)

	maintenanceClient := &maintenanceClientMock{
		getMachineConfigCh: getMachineConfigCh,
		applyConfigReqCh:   applyConfigReqCh,
	}

	maintenanceClientFactory := func(ctx context.Context, managementAddress string) (omni.MaintenanceClient, error) {
		select {
		case mgmtAddressCh <- managementAddress:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		return maintenanceClient, nil
	}

	// Register the controller and start the runtime
	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, "test-host", 123, 456)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	// Trigger a full reconciliation with config apply
	link := siderolinkres.NewLink(resources.DefaultNamespace, "test-machine", &specs.SiderolinkSpec{})
	link.TypedSpec().Value.Connected = true
	link.TypedSpec().Value.NodePublicKey = "test-public-key-1"

	suite.Require().NoError(suite.state.Create(suite.ctx, link))

	machineStatus := omnires.NewMachineStatus(resources.DefaultNamespace, "test-machine")
	machineStatus.TypedSpec().Value.Maintenance = true

	machineStatus.TypedSpec().Value.ManagementAddress = "test-address"
	machineStatus.TypedSpec().Value.TalosVersion = "1.5.0"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	// Assert that the maintenance client factory was called with the correct management address
	select {
	case mgmtAddr := <-mgmtAddressCh:
		suite.Require().Equal("test-address", mgmtAddr)
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for maintenance client factory to be called")
	}

	// Return an empty existing machine config to assert that we can generate a fresh config correctly
	select {
	case getMachineConfigCh <- nil:
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for get machine config request")
	}

	// Capture the apply configuration request and assert that it contains the expected data
	var applyConfigReq *machine.ApplyConfigurationRequest

	select {
	case applyConfigReq = <-maintenanceClient.applyConfigReqCh:
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for apply configuration request")
	}

	dataStr := string(applyConfigReq.Data)

	suite.Contains(dataStr, "omni-kmsg")
	suite.Contains(dataStr, "test-host:123")
	suite.Contains(dataStr, "test-host:456")
	suite.Equal(machine.ApplyConfigurationRequest_AUTO, applyConfigReq.GetMode())

	// Change machine's siderolink public key to simulate a reboot
	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, link.Metadata(), func(res *siderolinkres.Link) error {
		res.TypedSpec().Value.NodePublicKey = "test-public-key-2"

		return nil
	})
	suite.Require().NoError(err)

	// Assert again that the maintenance client factory was called with the correct management address
	select {
	case mgmtAddr := <-mgmtAddressCh:
		suite.Require().Equal("test-address", mgmtAddr)
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for maintenance client factory to be called")
	}

	// Prepare an existing partial machine config which contains a SideroLinkConfig resource
	u, err := url.Parse("http://example.org")
	suite.Require().NoError(err)

	siderolinkDoc := siderolink.NewConfigV1Alpha1()
	siderolinkDoc.APIUrlConfig.URL = u

	configContainer, err := container.New(siderolinkDoc)
	suite.Require().NoError(err)

	// Return this partial config from the machine
	select {
	case getMachineConfigCh <- config.NewMachineConfig(configContainer):
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for get machine config request")
	}

	// Capture the apply configuration request and assert that it contains the expected data
	select {
	case applyConfigReq = <-maintenanceClient.applyConfigReqCh:
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for apply configuration request")
	}

	dataStr = string(applyConfigReq.Data)

	suite.Contains(dataStr, "kind: SideroLinkConfig")
	suite.Contains(dataStr, "http://example.org")
	suite.Contains(dataStr, "omni-kmsg")
	suite.Contains(dataStr, "test-host:123")
	suite.Contains(dataStr, "test-host:456")
	suite.Equal(machine.ApplyConfigurationRequest_AUTO, applyConfigReq.GetMode())
	suite.Contains(dataStr, u.String())
}

func TestMaintenanceConfigStatusControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MaintenanceConfigStatusControllerSuite))
}

type maintenanceClientMock struct {
	applyConfigReqCh   chan *machine.ApplyConfigurationRequest
	getMachineConfigCh chan *config.MachineConfig
}

func (m *maintenanceClientMock) GetMachineConfig(ctx context.Context) (*config.MachineConfig, error) {
	select {
	case cfg := <-m.getMachineConfigCh:
		return cfg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *maintenanceClientMock) ApplyConfiguration(ctx context.Context, req *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
	select {
	case m.applyConfigReqCh <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return &machine.ApplyConfigurationResponse{}, nil
}
