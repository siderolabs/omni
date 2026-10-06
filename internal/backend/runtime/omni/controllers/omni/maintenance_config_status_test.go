// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configtry"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
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
	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, 123, 456, suite.state, nil)

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

// reconcileMaintenanceMachine registers a connected maintenance machine and returns the config apply request the controller issues for it.
func (suite *MaintenanceConfigStatusControllerSuite) reconcileMaintenanceMachine(
	maintenanceClient *maintenanceClientMock, machineIDCh chan string, id, talosVersion, publicKey string,
) *machine.ApplyConfigurationRequest {
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
	case maintenanceClient.getMachineConfigCh <- nil:
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

	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, 123, 456, suite.state, nil)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	reconcileMachine := func(id, talosVersion, publicKey string) *machine.ApplyConfigurationRequest {
		return suite.reconcileMaintenanceMachine(maintenanceClient, machineIDCh, id, talosVersion, publicKey)
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

func (suite *MaintenanceConfigStatusControllerSuite) TestRegistryMirrors() {
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

	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, 123, 456, suite.state,
		[]string{"docker.io=http://mirror.example.org:5000", "factory.talos.dev=http://mirror.example.org:5004", "docker.io=http://mirror-2.example.org:5000"})

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	reconcileMachine := func(id, talosVersion, publicKey string) *machine.ApplyConfigurationRequest {
		return suite.reconcileMaintenanceMachine(maintenanceClient, machineIDCh, id, talosVersion, publicKey)
	}

	// Talos version that predates RegistryMirrorConfig: mirrors must NOT be injected.
	oldReq := reconcileMachine("old-machine", "1.11.0", "pk-old")
	oldData := string(oldReq.Data)

	suite.Contains(oldData, "omni-kmsg")
	suite.NotContains(oldData, "kind: RegistryMirrorConfig")

	// Talos version that supports RegistryMirrorConfig: mirrors must be injected.
	newReq := reconcileMachine("new-machine", "1.12.0", "pk-new")
	newData := string(newReq.Data)

	suite.Contains(newData, "omni-kmsg")
	suite.Contains(newData, "kind: RegistryMirrorConfig")
	suite.Contains(newData, "name: docker.io")
	suite.Contains(newData, "url: http://mirror.example.org:5000")
	suite.Contains(newData, "name: factory.talos.dev")
	suite.Contains(newData, "url: http://mirror.example.org:5004")

	// two mirrors for the same registry are two endpoints of a single document
	suite.Equal(1, strings.Count(newData, "name: docker.io"))
	suite.Contains(newData, "url: http://mirror-2.example.org:5000")

	// image factory credentials that appear later must still make it into the config, next to the mirrors.
	auth := omnires.NewImageFactoryAuth("https://factory.example.org")
	auth.TypedSpec().Value.Username = "factory-user"
	auth.TypedSpec().Value.Password = "factory-pass"

	suite.Require().NoError(suite.state.Create(suite.ctx, auth))

	authReq := reconcileMachine("auth-machine", "1.12.0", "pk-auth")
	authData := string(authReq.Data)

	suite.Contains(authData, "kind: RegistryMirrorConfig")
	suite.Contains(authData, "kind: RegistryAuthConfig")
	suite.Contains(authData, "username: factory-user")
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

	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, 123, 456, suite.state, nil)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	// a machine-level config patch which carries a partial config document to preserve (TrustedRootsConfig) and a v1alpha1 document which must be stripped in maintenance mode
	const patchData = `machine:
  features:
    hostDNS:
      enabled: true
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

	// the v1alpha1 document is stripped
	suite.NotContains(dataStr, "hostDNS:")
}

func (suite *MaintenanceConfigStatusControllerSuite) TestMachineConfigPatchDocumentRemoval() {
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

	controller := omni.NewMaintenanceConfigStatusController(maintenanceClientFactory, 123, 456, suite.state, nil)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	suite.startRuntime()

	const (
		docA = `apiVersion: v1alpha1
kind: TrustedRootsConfig
name: doc-a
certificates: |
  -----BEGIN CERTIFICATE-----
  MIIA
  -----END CERTIFICATE-----
`
		docB = `apiVersion: v1alpha1
kind: TrustedRootsConfig
name: doc-b
certificates: |
  -----BEGIN CERTIFICATE-----
  MIIB
  -----END CERTIFICATE-----
`
	)

	patch := omnires.NewConfigPatch("removal-machine-patch")
	patch.Metadata().Labels().Set(omnires.LabelMachine, "removal-machine")
	suite.Require().NoError(patch.TypedSpec().Value.SetUncompressedData([]byte(docA + "---\n" + docB)))
	suite.Require().NoError(suite.state.Create(suite.ctx, patch))

	link := siderolinkres.NewLink("removal-machine", &specs.SiderolinkSpec{})
	link.TypedSpec().Value.Connected = true
	link.TypedSpec().Value.NodePublicKey = "removal-machine-key"

	suite.Require().NoError(suite.state.Create(suite.ctx, link))

	machineStatus := omnires.NewMachineStatus("removal-machine")
	machineStatus.TypedSpec().Value.Maintenance = true
	machineStatus.TypedSpec().Value.ManagementAddress = "removal-address"
	machineStatus.TypedSpec().Value.TalosVersion = "1.5.0"

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	suite.markConfigExtracted("removal-machine")

	// the machine's own SideroLink document: it is never part of a patch and must survive every reapply
	u, err := url.Parse("http://example.org")
	suite.Require().NoError(err)

	siderolinkDoc := siderolink.NewConfigV1Alpha1()
	siderolinkDoc.APIUrlConfig.URL = u

	// the config the machine currently runs: initially only its SideroLink document, afterwards whatever Omni applied last
	var applied []byte

	currentMachineConfig := func() *config.MachineConfig {
		if applied == nil {
			configContainer, containerErr := container.New(siderolinkDoc)
			suite.Require().NoError(containerErr)

			return config.NewMachineConfig(configContainer)
		}

		provider, loadErr := configloader.NewFromBytes(applied)
		suite.Require().NoError(loadErr)

		return config.NewMachineConfig(provider)
	}

	reconcile := func() string {
		select {
		case <-machineIDCh:
		case <-suite.ctx.Done():
			suite.Require().Fail("timeout waiting for maintenance client factory to be called")
		}

		select {
		case getMachineConfigCh <- currentMachineConfig():
		case <-suite.ctx.Done():
			suite.Require().Fail("timeout waiting for get machine config request")
		}

		select {
		case applyConfigReq := <-applyConfigReqCh:
			applied = applyConfigReq.Data
		case <-suite.ctx.Done():
			suite.Require().Fail("timeout waiting for apply configuration request")
		}

		return string(applied)
	}

	dataStr := reconcile()

	suite.Contains(dataStr, "name: doc-a")
	suite.Contains(dataStr, "name: doc-b")
	suite.Contains(dataStr, "kind: SideroLinkConfig")
	suite.Contains(dataStr, "http://example.org")
	suite.Contains(dataStr, "omni-kmsg")

	// drop a document from the patch: it must leave the machine
	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, patch.Metadata(), func(res *omnires.ConfigPatch) error {
		return res.TypedSpec().Value.SetUncompressedData([]byte(docA))
	})
	suite.Require().NoError(err)

	dataStr = reconcile()

	suite.Contains(dataStr, "name: doc-a")
	suite.NotContains(dataStr, "name: doc-b")
	suite.Contains(dataStr, "kind: SideroLinkConfig")
	suite.Contains(dataStr, "http://example.org")
	suite.Contains(dataStr, "omni-kmsg")

	// delete the patch: only the machine's SideroLink document and the Omni-managed connection documents remain
	suite.Require().NoError(suite.state.Destroy(suite.ctx, patch.Metadata()))

	dataStr = reconcile()

	suite.NotContains(dataStr, "name: doc-a")
	suite.NotContains(dataStr, "name: doc-b")
	suite.Contains(dataStr, "kind: SideroLinkConfig")
	suite.Contains(dataStr, "http://example.org")
	suite.Contains(dataStr, "omni-kmsg")
}

//nolint:gocognit,gocyclo,cyclop,maintidx
func TestMaintenanceConfigStatusTryMode(t *testing.T) {
	t.Parallel()

	const patchID = "000-try-mode-patch"

	// maintenanceTryTimings keeps the shape of the production set, scaled down to fit inside a test.
	maintenanceTryTimings := configtry.Timings{
		Timeout:         4 * time.Second,
		ConfirmAfter:    300 * time.Millisecond,
		ConfirmDeadline: 3 * time.Second,
		RollbackGrace:   500 * time.Millisecond,
		ConfirmRetry:    200 * time.Millisecond,
		MaxAttempts:     2,
	}

	// runWithTryModeMachine brings up a machine in maintenance on Talos 1.14 with its first config applied, and passes the hash recorded for it.
	runWithTryModeMachine := func(
		t *testing.T, id string, maintenanceClient *recordingMaintenanceClient, test func(ctx context.Context, st state.State, firstHash string),
	) {
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				factory := func(context.Context, string) (omni.MaintenanceClient, error) { return maintenanceClient, nil }

				require.NoError(t, tc.Runtime.RegisterQController(
					omni.NewMaintenanceConfigStatusController(factory, 123, 456, tc.State, nil,
						omni.WithMaintenanceTryTimings(maintenanceTryTimings)),
				))
			},
			func(ctx context.Context, tc testutils.TestContext) {
				link := siderolinkres.NewLink(id, &specs.SiderolinkSpec{})
				link.TypedSpec().Value.Connected = true
				link.TypedSpec().Value.NodePublicKey = id + "-key"

				require.NoError(t, tc.State.Create(ctx, link))

				machineStatus := omnires.NewMachineStatus(id)
				machineStatus.TypedSpec().Value.Maintenance = true
				machineStatus.TypedSpec().Value.ManagementAddress = id + "-address"
				machineStatus.TypedSpec().Value.TalosVersion = "1.14.0"

				require.NoError(t, tc.State.Create(ctx, machineStatus))

				extractionStatus := omnires.NewMachineConfigExtractionStatus(id)
				extractionStatus.TypedSpec().Value.Initialized = true

				require.NoError(t, tc.State.Create(ctx, extractionStatus))

				// the machine boots with no config at all, so the first apply cannot be tried
				var firstHash string

				require.EventuallyWithT(t, func(c *assert.CollectT) {
					status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, tc.State, id)
					if !assert.NoError(c, err) {
						return
					}

					firstHash = status.TypedSpec().Value.LastAppliedConfigHash

					assert.NotEmpty(c, firstHash)
					assert.Nil(c, status.TypedSpec().Value.ConfigTry)
				}, 10*time.Second, 20*time.Millisecond)

				requests := maintenanceClient.getRequests()
				require.Len(t, requests, 1)
				require.Equal(t, machine.ApplyConfigurationRequest_AUTO, requests[0].GetMode())

				test(ctx, tc.State, firstHash)
			},
		)
	}

	createTryModePatch := func(ctx context.Context, t *testing.T, st state.State, id string) {
		patch := omnires.NewConfigPatch(patchID)
		patch.Metadata().Labels().Set(omnires.LabelMachine, id)
		require.NoError(t, patch.TypedSpec().Value.SetUncompressedData([]byte("apiVersion: v1alpha1\nkind: KmsgLogConfig\nname: try-mode-test\nurl: tcp://127.0.0.1:5170\n")))
		require.NoError(t, st.Create(ctx, patch))
	}

	waitForMaintenanceTry := func(t *testing.T, maintenanceClient *recordingMaintenanceClient) {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.True(c, slices.ContainsFunc(maintenanceClient.getRequests(), func(req *machine.ApplyConfigurationRequest) bool {
				return req.GetMode() == machine.ApplyConfigurationRequest_TRY
			}))
		}, 10*time.Second, 20*time.Millisecond)
	}

	rotateKey := func(ctx context.Context, t *testing.T, st state.State, id, key string) {
		_, err := safe.StateUpdateWithConflicts(ctx, st, siderolinkres.NewLink(id, nil).Metadata(), func(link *siderolinkres.Link) error {
			link.TypedSpec().Value.NodePublicKey = key

			return nil
		})
		require.NoError(t, err)
	}

	// The first config of a boot has no config to roll back to and goes in unprotected, while every change after it is tried first.
	t.Run("afterFirstApply", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-stable"}

		const id = "try-mode-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, firstHash string) {
			createTryModePatch(ctx, t, st, id)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				var try *machine.ApplyConfigurationRequest

				for _, req := range maintenanceClient.getRequests() {
					if req.GetMode() == machine.ApplyConfigurationRequest_TRY {
						try = req
					}
				}

				if !assert.NotNil(c, try, "the config change should have been applied in try mode") {
					return
				}

				assert.Equal(c, maintenanceTryTimings.Timeout, try.GetTryModeTimeout().AsDuration())
			}, 10*time.Second, 20*time.Millisecond)

			// once it has stayed on the machine long enough, it is confirmed with a regular apply and recorded
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.NotEqual(c, firstHash, status.TypedSpec().Value.LastAppliedConfigHash)
				assert.Nil(c, status.TypedSpec().Value.ConfigTry)
				assert.Empty(c, status.TypedSpec().Value.LastConfigError)
			}, 10*time.Second, 20*time.Millisecond)

			requests := maintenanceClient.getRequests()
			last := requests[len(requests)-1]

			assert.Equal(t, machine.ApplyConfigurationRequest_NO_REBOOT, last.GetMode())
			assert.Nil(t, last.GetTryModeTimeout())
		})
	})

	// A config that cuts the machine off before the try apply can answer: the try is recorded anyway and confirmed once the machine answers again.
	t.Run("recordsTryWithoutAnswer", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-stable", loseTryAnswer: true}

		const id = "try-mode-silent-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, firstHash string) {
			createTryModePatch(ctx, t, st, id)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.NotEqual(c, firstHash, status.TypedSpec().Value.LastAppliedConfigHash)
				assert.Nil(c, status.TypedSpec().Value.ConfigTry)
				assert.Empty(c, status.TypedSpec().Value.LastConfigError)
			}, 10*time.Second, 20*time.Millisecond)

			var tries int

			for _, req := range maintenanceClient.getRequests() {
				if req.GetMode() == machine.ApplyConfigurationRequest_TRY {
					tries++
				}
			}

			assert.Equal(t, 1, tries, "the unanswered try must be recorded, not sent again")
		})
	})

	// A try that never reached the machine must not be confirmed as a regular apply.
	t.Run("doesNotConfirmAnUnlandedTry", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-stable", dropTries: 1}

		const id = "try-mode-unlanded-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, firstHash string) {
			createTryModePatch(ctx, t, st, id)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.NotEqual(c, firstHash, status.TypedSpec().Value.LastAppliedConfigHash)
				assert.Nil(c, status.TypedSpec().Value.ConfigTry)
			}, 20*time.Second, 20*time.Millisecond)

			var tries, confirms int

			for _, req := range maintenanceClient.getRequests()[1:] {
				switch req.GetMode() { //nolint:exhaustive
				case machine.ApplyConfigurationRequest_TRY:
					tries++
				case machine.ApplyConfigurationRequest_NO_REBOOT:
					confirms++
				}
			}

			assert.Equal(t, 2, tries, "the unlanded try should be followed by a second one")
			assert.Equal(t, 1, confirms, "only the try that landed may be confirmed")
		})
	})

	// A config that costs the machine its boot: the boot ID no longer matches when it is time to confirm, so the config is never recorded.
	t.Run("stopsWhenMachineReboots", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-0", rebootOnTry: true}

		const id = "try-mode-rebooting-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, firstHash string) {
			createTryModePatch(ctx, t, st, id)

			// every attempt is spent, then the machine is left alone with an error to show for it
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.Contains(c, status.TypedSpec().Value.LastConfigError, "rolled back")
				assert.Equal(c, maintenanceTryTimings.MaxAttempts, status.TypedSpec().Value.ConfigTry.GetAttempts())
				assert.Equal(c, firstHash, status.TypedSpec().Value.LastAppliedConfigHash, "a config that was never confirmed must not be recorded")
			}, 30*time.Second, 20*time.Millisecond)

			for _, req := range maintenanceClient.getRequests()[1:] {
				assert.Equal(t, machine.ApplyConfigurationRequest_TRY, req.GetMode(), "the unconfirmed config must only ever be applied in try mode")
			}

			// taking the patch back leaves nothing to try, so the error goes away without another apply
			require.NoError(t, st.Destroy(ctx, omnires.NewConfigPatch(patchID).Metadata()))

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.Nil(c, status.TypedSpec().Value.ConfigTry)
				assert.Empty(c, status.TypedSpec().Value.LastConfigError)
				assert.Equal(c, firstHash, status.TypedSpec().Value.LastAppliedConfigHash)
			}, 10*time.Second, 20*time.Millisecond)

			requests := maintenanceClient.getRequests()

			assert.Equal(t, machine.ApplyConfigurationRequest_TRY, requests[len(requests)-1].GetMode(), "no apply should follow the patch removal")
		})
	})

	// A real reboot also rotates the SideroLink key, which makes the next apply the first one of a boot. A config Omni
	// gave up on must not be pushed then.
	t.Run("staysStoppedAfterReboot", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-0", rebootOnTry: true}

		const id = "try-mode-stopped-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, firstHash string) {
			createTryModePatch(ctx, t, st, id)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.Contains(c, status.TypedSpec().Value.LastConfigError, "rolled back")
			}, 30*time.Second, 20*time.Millisecond)

			requestsBefore := len(maintenanceClient.getRequests())

			rotateKey(ctx, t, st, id, "rebooted")

			// the reconcile that sees the new key drops the in-flight part of the try and returns without an apply
			rtestutils.AssertResource(ctx, t, st, id, func(res *omnires.MaintenanceConfigStatus, a *assert.Assertions) {
				a.Nil(res.TypedSpec().Value.ConfigTry.GetStartedAt())
				a.Equal(maintenanceTryTimings.MaxAttempts, res.TypedSpec().Value.ConfigTry.GetAttempts())
				a.Contains(res.TypedSpec().Value.LastConfigError, "rolled back")
				a.Equal(firstHash, res.TypedSpec().Value.LastAppliedConfigHash)
			})

			assert.Len(t, maintenanceClient.getRequests(), requestsBefore, "a config Omni gave up on must not be applied after a reboot")
		})
	})

	t.Run("clearsErrorOutsideMaintenance", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-0", rebootOnTry: true}

		const id = "try-mode-installed-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, _ string) {
			createTryModePatch(ctx, t, st, id)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.Contains(c, status.TypedSpec().Value.LastConfigError, "rolled back")
			}, 30*time.Second, 20*time.Millisecond)

			_, err := safe.StateUpdateWithConflicts(ctx, st, omnires.NewMachineStatus(id).Metadata(), func(res *omnires.MachineStatus) error {
				res.TypedSpec().Value.Maintenance = false

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, st, id, func(res *omnires.MaintenanceConfigStatus, a *assert.Assertions) {
				a.Empty(res.TypedSpec().Value.LastConfigError)
				a.Equal(maintenanceTryTimings.MaxAttempts, res.TypedSpec().Value.ConfigTry.GetAttempts())
			})
		})
	})

	// A reboot during a try: the config is tried again on the new boot, never applied plainly.
	t.Run("triesAgainAfterReboot", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-stable", failConfirmsOf: "127.0.0.1:5170"}

		const id = "try-mode-interrupted-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, _ string) {
			createTryModePatch(ctx, t, st, id)
			waitForMaintenanceTry(t, maintenanceClient)

			rotateKey(ctx, t, st, id, "rebooted")

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				var tries int

				for _, req := range maintenanceClient.getRequests()[1:] {
					if req.GetMode() == machine.ApplyConfigurationRequest_TRY {
						tries++
					}
				}

				assert.Equal(c, 2, tries)
			}, 10*time.Second, 20*time.Millisecond)

			for _, req := range maintenanceClient.getRequests()[1:] {
				assert.NotEqual(t, machine.ApplyConfigurationRequest_AUTO, req.GetMode(), "a config that was tried and never confirmed must not be applied plainly")
			}
		})
	})

	// A config change while a try is pending: the new config is tried and confirmed without waiting for the pending try to roll back.
	t.Run("replacesPendingTry", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-stable", failConfirmsOf: "127.0.0.1:5170"}

		const id = "try-mode-replacing-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, firstHash string) {
			createTryModePatch(ctx, t, st, id)
			waitForMaintenanceTry(t, maintenanceClient)

			_, err := safe.StateUpdateWithConflicts(ctx, st, omnires.NewConfigPatch(patchID).Metadata(), func(patch *omnires.ConfigPatch) error {
				return patch.TypedSpec().Value.SetUncompressedData([]byte("apiVersion: v1alpha1\nkind: KmsgLogConfig\nname: try-mode-test\nurl: tcp://127.0.0.1:5171\n"))
			})
			require.NoError(t, err)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.NotEqual(c, firstHash, status.TypedSpec().Value.LastAppliedConfigHash)
				assert.Nil(c, status.TypedSpec().Value.ConfigTry)
			}, maintenanceTryTimings.Timeout, 20*time.Millisecond, "the new config must be confirmed before the pending try times out")

			requests := maintenanceClient.getRequests()
			last := requests[len(requests)-1]

			assert.Equal(t, machine.ApplyConfigurationRequest_NO_REBOOT, last.GetMode())
			assert.Contains(t, string(last.GetData()), "127.0.0.1:5171")
		})
	})

	// Taking a change back while its try is pending: the confirmed config is tried and confirmed again, instead of being
	// treated as in sync while the other config is still active.
	t.Run("revertReplacesPendingTry", func(t *testing.T) {
		t.Parallel()

		maintenanceClient := &recordingMaintenanceClient{bootID: "boot-stable", failConfirmsOf: "127.0.0.1:5170"}

		const id = "try-mode-reverting-machine"

		runWithTryModeMachine(t, id, maintenanceClient, func(ctx context.Context, st state.State, firstHash string) {
			confirmedConfig := string(maintenanceClient.getRequests()[0].GetData())

			createTryModePatch(ctx, t, st, id)
			waitForMaintenanceTry(t, maintenanceClient)

			require.NoError(t, st.Destroy(ctx, omnires.NewConfigPatch(patchID).Metadata()))

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				var tried, confirmed bool

				for _, req := range maintenanceClient.getRequests()[1:] {
					if string(req.GetData()) != confirmedConfig {
						continue
					}

					switch req.GetMode() { //nolint:exhaustive
					case machine.ApplyConfigurationRequest_TRY:
						tried = true
					case machine.ApplyConfigurationRequest_NO_REBOOT:
						confirmed = tried
					}
				}

				assert.True(c, confirmed, "the confirmed config must be tried and confirmed again")
			}, maintenanceTryTimings.Timeout, 20*time.Millisecond)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				status, err := safe.StateGetByID[*omnires.MaintenanceConfigStatus](ctx, st, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.Equal(c, firstHash, status.TypedSpec().Value.LastAppliedConfigHash)
				assert.Nil(c, status.TypedSpec().Value.ConfigTry)
			}, 10*time.Second, 20*time.Millisecond)
		})
	})
}

func TestMaintenanceConfigStatusControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MaintenanceConfigStatusControllerSuite))
}

// recordingMaintenanceClient stands in for a machine in maintenance mode: it remembers the config it
// was last given, the way Talos keeps an active config, and records every request.
type recordingMaintenanceClient struct {
	active *config.MachineConfig
	bootID string

	// failConfirmsOf makes every confirming apply of a config containing it fail, which keeps its try pending.
	failConfirmsOf string

	requests []*machine.ApplyConfigurationRequest
	mu       sync.Mutex
	boots    int

	// dropTries is how many try applies never reach the machine: Omni gets no answer and nothing changes on the machine.
	dropTries int

	// rebootOnTry makes every try apply look like it rebooted the machine: the boot ID changes with it.
	rebootOnTry bool

	// loseTryAnswer makes every try apply land without answering, the way a config that cuts the machine off does.
	loseTryAnswer bool
}

func (m *recordingMaintenanceClient) GetMachineConfig(context.Context) (*config.MachineConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.active, nil
}

func (m *recordingMaintenanceClient) GetBootID(context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.bootID, nil
}

func (m *recordingMaintenanceClient) ApplyConfiguration(_ context.Context, req *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = append(m.requests, req)

	if m.dropTries > 0 && req.GetMode() == machine.ApplyConfigurationRequest_TRY {
		m.dropTries--

		return nil, grpcstatus.Error(codes.DeadlineExceeded, "the request never reached the machine")
	}

	if m.failConfirmsOf != "" && req.GetMode() == machine.ApplyConfigurationRequest_NO_REBOOT && strings.Contains(string(req.GetData()), m.failConfirmsOf) {
		return nil, grpcstatus.Error(codes.Unavailable, "machine is unreachable")
	}

	provider, err := configloader.NewFromBytes(req.GetData())
	if err != nil {
		return nil, err
	}

	m.active = config.NewMachineConfig(provider)

	if m.rebootOnTry && req.GetMode() == machine.ApplyConfigurationRequest_TRY {
		m.boots++
		m.bootID = fmt.Sprintf("boot-%d", m.boots)
	}

	if m.loseTryAnswer && req.GetMode() == machine.ApplyConfigurationRequest_TRY {
		return nil, grpcstatus.Error(codes.DeadlineExceeded, "the answer never came back")
	}

	return &machine.ApplyConfigurationResponse{}, nil
}

func (m *recordingMaintenanceClient) getRequests() []*machine.ApplyConfigurationRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	return slices.Clone(m.requests)
}

func (m *recordingMaintenanceClient) Close() error {
	return nil
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

func (m *maintenanceClientMock) Close() error {
	return nil
}

func (m *maintenanceClientMock) GetBootID(context.Context) (string, error) {
	return "boot-1", nil
}
