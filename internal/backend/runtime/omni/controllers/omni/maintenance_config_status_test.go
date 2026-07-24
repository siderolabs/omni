// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"net"
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	siderolinkomni "github.com/siderolabs/omni/internal/pkg/siderolink"
)

type MaintenanceConfigStatusControllerSuite struct {
	OmniSuite
}

// markConfigExtracted marks the machine's incoming config as already extracted, which the maintenance config controller waits for before applying.
func (suite *OmniSuite) markConfigExtracted(id string) {
	status := omnires.NewMachineConfigExtractionStatus(id)
	status.TypedSpec().Value.Initialized = true

	suite.Require().NoError(suite.state.Create(suite.ctx, status))
}

func (suite *MachineStatusSnapshotControllerSuite) TestMaintenanceConfigStatus() {
	// Prepare the mock maintenance client factory
	getMachineConfigCh := make(chan *config.MachineConfig)
	machineIDCh := make(chan string)
	applyConfigReqCh := make(chan *machine.ApplyConfigurationRequest)

	maintenanceClient := &maintenanceClientMock{
		getMachineConfigCh: getMachineConfigCh,
		applyConfigReqCh:   applyConfigReqCh,
	}

	maintenanceClientFactory := func(ctx context.Context, machineID string) (omni.MaintenanceClient, error) {
		select {
		case machineIDCh <- machineID:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		return maintenanceClient, nil
	}

	// Register the controller and start the runtime
	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, 123, 456, suite.state)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	// Trigger a full reconciliation with config apply
	link := siderolinkres.NewLink("test-machine", &specs.SiderolinkSpec{})
	link.TypedSpec().Value.Connected = true
	link.TypedSpec().Value.NodePublicKey = "test-public-key-1"

	suite.Require().NoError(suite.state.Create(suite.ctx, link))

	machineStatus := omnires.NewMachineStatus("test-machine")
	machineStatus.TypedSpec().Value.Maintenance = true

	machineStatus.TypedSpec().Value.ManagementAddress = "test-address"
	machineStatus.TypedSpec().Value.TalosVersion = "1.5.0"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	suite.markConfigExtracted("test-machine")

	// Assert that the maintenance client factory was called with the correct machine ID
	select {
	case observedMachineID := <-machineIDCh:
		suite.Require().Equal("test-machine", observedMachineID)
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
	suite.Contains(dataStr, net.JoinHostPort(siderolinkomni.ListenHost, "123"))
	suite.Contains(dataStr, net.JoinHostPort(siderolinkomni.ListenHost, "456"))
	suite.Equal(machine.ApplyConfigurationRequest_AUTO, applyConfigReq.GetMode())

	// Change machine's siderolink public key to simulate a reboot
	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, link.Metadata(), func(res *siderolinkres.Link) error {
		res.TypedSpec().Value.NodePublicKey = "test-public-key-2"

		return nil
	})
	suite.Require().NoError(err)

	// Assert again that the maintenance client factory was called with the correct machine ID
	select {
	case observedMachineID := <-machineIDCh:
		suite.Require().Equal("test-machine", observedMachineID)
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
	suite.Contains(dataStr, net.JoinHostPort(siderolinkomni.ListenHost, "123"))
	suite.Contains(dataStr, net.JoinHostPort(siderolinkomni.ListenHost, "456"))
	suite.Equal(machine.ApplyConfigurationRequest_AUTO, applyConfigReq.GetMode())
	suite.Contains(dataStr, u.String())
}

func (suite *MaintenanceConfigStatusControllerSuite) TestImageFactoryRegistryAuth() {
	getMachineConfigCh := make(chan *config.MachineConfig)
	machineIDCh := make(chan string)
	applyConfigReqCh := make(chan *machine.ApplyConfigurationRequest)

	maintenanceClient := &maintenanceClientMock{
		getMachineConfigCh: getMachineConfigCh,
		applyConfigReqCh:   applyConfigReqCh,
	}

	maintenanceClientFactory := func(ctx context.Context, machineID string) (omni.MaintenanceClient, error) {
		select {
		case machineIDCh <- machineID:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		return maintenanceClient, nil
	}

	auth := omnires.NewImageFactoryAuth("https://factory.example.org")
	auth.TypedSpec().Value.Username = "factory-user"
	auth.TypedSpec().Value.Password = "factory-pass"

	suite.Require().NoError(suite.state.Create(suite.ctx, auth))

	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, 123, 456, suite.state)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	reconcileMachine := func(id, talosVersion, publicKey string) *machine.ApplyConfigurationRequest {
		suite.T().Helper()

		link := siderolinkres.NewLink(id, &specs.SiderolinkSpec{})
		link.TypedSpec().Value.Connected = true
		link.TypedSpec().Value.NodePublicKey = publicKey

		suite.Require().NoError(suite.state.Create(suite.ctx, link))

		machineStatus := omnires.NewMachineStatus(id)
		machineStatus.TypedSpec().Value.Maintenance = true
		machineStatus.TypedSpec().Value.ManagementAddress = id + "-address"
		machineStatus.TypedSpec().Value.TalosVersion = talosVersion

		suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

		suite.markConfigExtracted(id)

		select {
		case <-machineIDCh:
		case <-suite.ctx.Done():
			suite.Require().Fail("timeout waiting for maintenance client factory to be called")
		}

		select {
		case getMachineConfigCh <- nil:
		case <-suite.ctx.Done():
			suite.Require().Fail("timeout waiting for get machine config request")
		}

		select {
		case req := <-maintenanceClient.applyConfigReqCh:
			return req
		case <-suite.ctx.Done():
			suite.Require().Fail("timeout waiting for apply configuration request")
		}

		return nil
	}

	// Talos version that predates RegistryAuthConfig: auth must NOT be injected.
	oldReq := reconcileMachine("old-machine", "1.11.0", "pk-old")
	oldData := string(oldReq.Data)

	suite.Contains(oldData, "omni-kmsg")
	suite.NotContains(oldData, "kind: RegistryAuthConfig")
	suite.NotContains(oldData, "factory-user")
	suite.NotContains(oldData, "factory-pass")

	// Talos version that supports RegistryAuthConfig: auth must be injected.
	newReq := reconcileMachine("new-machine", "1.12.0", "pk-new")
	newData := string(newReq.Data)

	suite.Contains(newData, "omni-kmsg")
	suite.Contains(newData, "kind: RegistryAuthConfig")
	suite.Contains(newData, "name: factory.example.org")
	suite.Contains(newData, "username: factory-user")
	suite.Contains(newData, "password: factory-pass")
}

func (suite *MaintenanceConfigStatusControllerSuite) TestMachineConfigPatchPreserved() {
	getMachineConfigCh := make(chan *config.MachineConfig)
	machineIDCh := make(chan string)
	applyConfigReqCh := make(chan *machine.ApplyConfigurationRequest)

	maintenanceClient := &maintenanceClientMock{
		getMachineConfigCh: getMachineConfigCh,
		applyConfigReqCh:   applyConfigReqCh,
	}

	maintenanceClientFactory := func(ctx context.Context, machineID string) (omni.MaintenanceClient, error) {
		select {
		case machineIDCh <- machineID:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		return maintenanceClient, nil
	}

	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, 123, 456, suite.state)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	// a machine-level config patch which carries a partial config document to preserve (TrustedRootsConfig) and a v1alpha1 document which must be stripped in maintenance mode
	const patchData = `machine:
  install:
    disk: /dev/sda
---
apiVersion: v1alpha1
kind: TrustedRootsConfig
name: my-enterprise-ca
certificates: |
  -----BEGIN CERTIFICATE-----
  MIIB
  -----END CERTIFICATE-----
`

	patch := omnires.NewConfigPatch("000-preserved-machine-config-patch-machine")
	patch.Metadata().Labels().Set(omnires.LabelMachine, "patch-machine")
	suite.Require().NoError(patch.TypedSpec().Value.SetUncompressedData([]byte(patchData)))
	suite.Require().NoError(suite.state.Create(suite.ctx, patch))

	link := siderolinkres.NewLink("patch-machine", &specs.SiderolinkSpec{})
	link.TypedSpec().Value.Connected = true
	link.TypedSpec().Value.NodePublicKey = "patch-machine-key"

	suite.Require().NoError(suite.state.Create(suite.ctx, link))

	machineStatus := omnires.NewMachineStatus("patch-machine")
	machineStatus.TypedSpec().Value.Maintenance = true
	machineStatus.TypedSpec().Value.ManagementAddress = "patch-address"
	machineStatus.TypedSpec().Value.TalosVersion = "1.5.0"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	suite.markConfigExtracted("patch-machine")

	select {
	case <-machineIDCh:
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for maintenance client factory to be called")
	}

	// no existing config on the machine
	select {
	case getMachineConfigCh <- nil:
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for get machine config request")
	}

	var applyConfigReq *machine.ApplyConfigurationRequest

	select {
	case applyConfigReq = <-applyConfigReqCh:
	case <-suite.ctx.Done():
		suite.Require().Fail("timeout waiting for apply configuration request")
	}

	dataStr := string(applyConfigReq.Data)

	// the preserved partial document and the base connection documents are applied
	suite.Contains(dataStr, "TrustedRootsConfig")
	suite.Contains(dataStr, "my-enterprise-ca")
	suite.Contains(dataStr, "omni-kmsg")

	// the v1alpha1 document is stripped, so the machine is not installed and stays in maintenance mode
	suite.NotContains(dataStr, "/dev/sda")
	suite.NotContains(dataStr, "install:")
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
