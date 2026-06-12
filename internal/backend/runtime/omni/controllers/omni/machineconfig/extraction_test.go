// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	documentconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	talossiderolink "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

// readerMock serves a fixed machine config as if read from a machine in maintenance mode.
type readerMock struct {
	config *config.MachineConfig
}

func (m readerMock) ReadMachineConfig(context.Context, string) (*config.MachineConfig, error) {
	return m.config, nil
}

// registerExtractionController wires the extraction controller with a config reader serving the given config.
func registerExtractionController(t *testing.T, testContext testutils.TestContext, observed *config.MachineConfig) {
	require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewExtractionController(readerMock{config: observed})))
}

func TestExtractionExtractOnce(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// machine arrives in maintenance with an embedded TrustedRootsConfig (preserve) and a SideroLinkConfig (drop)
	trustedRoots := security.NewTrustedRootsConfigV1Alpha1()
	trustedRoots.MetaName = "my-enterprise-ca"
	trustedRoots.Certificates = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	u, err := url.Parse("https://siderolink.example.org")
	require.NoError(t, err)

	siderolinkDoc := talossiderolink.NewConfigV1Alpha1()
	siderolinkDoc.APIUrlConfig.URL = u

	observed, err := container.New(trustedRoots, siderolinkDoc)
	require.NoError(t, err)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			registerExtractionController(t, testContext, config.NewMachineConfig(observed))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			machineStatus := omnires.NewMachineStatus("extract-machine")
			machineStatus.TypedSpec().Value.Maintenance = true
			machineStatus.TypedSpec().Value.Connected = true
			machineStatus.TypedSpec().Value.ManagementAddress = "extract-address"

			require.NoError(t, st.Create(ctx, machineStatus))

			// the extracted patch preserves the partial document, drops the connection document, and is machine-scoped
			rtestutils.AssertResources(ctx, t, st, []string{"000-initial-machine-config-extract-machine"},
				func(patch *omnires.ConfigPatch, assert *assert.Assertions) {
					// the patch is created ownerless, so the user manages it from here on
					assert.Empty(patch.Metadata().Owner())

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

			rtestutils.AssertResources(ctx, t, st, []string{"extract-machine"},
				func(status *omnires.MachineConfigExtractionStatus, assert *assert.Assertions) {
					assert.True(status.TypedSpec().Value.Initialized)
				})
		},
	)
}

func TestExtractionEmptyConfigStillInitializes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			// machine has no config at all
			registerExtractionController(t, testContext, nil)
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			machineStatus := omnires.NewMachineStatus("empty-machine")
			machineStatus.TypedSpec().Value.Maintenance = true
			machineStatus.TypedSpec().Value.Connected = true
			machineStatus.TypedSpec().Value.ManagementAddress = "empty-address"

			require.NoError(t, st.Create(ctx, machineStatus))

			// still marked initialized, but no patch is created
			rtestutils.AssertResources(ctx, t, st, []string{"empty-machine"},
				func(status *omnires.MachineConfigExtractionStatus, assert *assert.Assertions) {
					assert.True(status.TypedSpec().Value.Initialized)
				})

			// the patch decision is final once initialized, so no config patch must exist
			_, err := st.Get(ctx, omnires.NewConfigPatch("000-initial-machine-config-empty-machine").Metadata())
			require.Error(t, err)
		},
	)
}

func TestExtractionForbiddenConfigReportsError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// a config that carries machine token / PKI is not a valid config patch and cannot be kept as one
	observed, err := configloader.NewFromBytes([]byte(`version: v1alpha1
machine:
    type: worker
    token: aaaaaa.bbbbbbbbbbbbbbbb
    ca:
        crt: Zm9v
        key: YmFy
cluster: {}
`))
	require.NoError(t, err)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			registerExtractionController(t, testContext, config.NewMachineConfig(observed))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			machineStatus := omnires.NewMachineStatus("forbidden-machine")
			machineStatus.TypedSpec().Value.Maintenance = true
			machineStatus.TypedSpec().Value.Connected = true
			machineStatus.TypedSpec().Value.ManagementAddress = "forbidden-address"

			require.NoError(t, st.Create(ctx, machineStatus))

			// initialized, with the reason recorded, and no patch created
			rtestutils.AssertResources(ctx, t, st, []string{"forbidden-machine"},
				func(status *omnires.MachineConfigExtractionStatus, assert *assert.Assertions) {
					assert.True(status.TypedSpec().Value.Initialized)
					assert.NotEmpty(status.TypedSpec().Value.Error)
				})

			_, err = st.Get(ctx, omnires.NewConfigPatch("000-initial-machine-config-forbidden-machine").Metadata())
			require.Error(t, err)
		},
	)
}

func encodeDocuments(t *testing.T, documents ...documentconfig.Document) []byte {
	t.Helper()

	ctr, err := container.New(documents...)
	require.NoError(t, err)

	data, err := ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)

	return data
}

func trustedRootsDocument() documentconfig.Document {
	doc := security.NewTrustedRootsConfigV1Alpha1()
	doc.MetaName = "my-enterprise-ca"
	doc.Certificates = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	return doc
}

func siderolinkDocument(t *testing.T) documentconfig.Document {
	t.Helper()

	doc := talossiderolink.NewConfigV1Alpha1()

	u, err := url.Parse("https://siderolink.example.org")
	require.NoError(t, err)

	doc.APIUrlConfig.URL = u

	return doc
}

func TestBuildConfigPatchKeepsNonConnectionDocuments(t *testing.T) {
	t.Parallel()

	eventSink := runtimecfg.NewEventSinkV1Alpha1()
	eventSink.Endpoint = "127.0.0.1:1234"

	observed := encodeDocuments(t, trustedRootsDocument(), siderolinkDocument(t), eventSink)

	patch, reason, err := (&machineconfig.ExtractionController{}).BuildConfigPatch("machine-1", observed)
	require.NoError(t, err)
	require.Empty(t, reason)
	require.NotNil(t, patch)

	require.Equal(t, "000-initial-machine-config-machine-1", patch.Metadata().ID())

	machineID, ok := patch.Metadata().Labels().Get(omnires.LabelMachine)
	require.True(t, ok)
	require.Equal(t, "machine-1", machineID)

	buffer, err := patch.TypedSpec().Value.GetUncompressedData()
	require.NoError(t, err)

	data := string(buffer.Data())
	buffer.Free()

	require.Contains(t, data, "TrustedRootsConfig")
	require.Contains(t, data, "my-enterprise-ca")
	require.NotContains(t, data, "SideroLinkConfig")
	require.NotContains(t, data, "EventSinkConfig")
}

// TestBuildConfigPatchKeepsEmbeddedRuntimeDocuments mirrors the config the integration test bakes into a machine's
// embedded configuration: the three SideroLink connection documents plus an EnvironmentConfig and a StaticHostConfig.
// It locks in that the connection documents are dropped while the two carrying user data survive into a valid patch,
// so the integration test's markers are guaranteed to land before spending a QEMU run on it.
func TestBuildConfigPatchKeepsEmbeddedRuntimeDocuments(t *testing.T) {
	t.Parallel()

	eventSink := runtimecfg.NewEventSinkV1Alpha1()
	eventSink.Endpoint = "127.0.0.1:1234"

	kmsgLog := runtimecfg.NewKmsgLogV1Alpha1()
	kmsgLog.MetaName = "omni-kmsg"

	environment := runtimecfg.NewEnvironmentV1Alpha1()
	environment.EnvironmentVariables = map[string]string{"OMNI_EMBEDDED_CONFIG_MARKER": "omni-embedded-config-env-marker"}

	staticHost := network.NewStaticHostConfigV1Alpha1("10.99.0.1")
	staticHost.Hostnames = []string{"omni-embedded-config-host-marker"}

	observed := encodeDocuments(t, siderolinkDocument(t), eventSink, kmsgLog, environment, staticHost)

	patch, reason, err := (&machineconfig.ExtractionController{}).BuildConfigPatch("machine-1", observed)
	require.NoError(t, err)
	require.Empty(t, reason)
	require.NotNil(t, patch)

	buffer, err := patch.TypedSpec().Value.GetUncompressedData()
	require.NoError(t, err)

	data := string(buffer.Data())
	buffer.Free()

	// the user-data documents survive, with their markers intact
	require.Contains(t, data, "EnvironmentConfig")
	require.Contains(t, data, "omni-embedded-config-env-marker")
	require.Contains(t, data, "StaticHostConfig")
	require.Contains(t, data, "omni-embedded-config-host-marker")

	// the Omni-managed connection documents are dropped
	require.NotContains(t, data, "SideroLinkConfig")
	require.NotContains(t, data, "EventSinkConfig")
	require.NotContains(t, data, "KmsgLogConfig")
}

func TestBuildConfigPatchSkipsWhenOnlyConnectionDocuments(t *testing.T) {
	t.Parallel()

	observed := encodeDocuments(t, siderolinkDocument(t))

	patch, reason, err := (&machineconfig.ExtractionController{}).BuildConfigPatch("machine-1", observed)
	require.NoError(t, err)
	require.Empty(t, reason)
	require.Nil(t, patch)
}

func TestBuildConfigPatchSkipsOnEmptyConfig(t *testing.T) {
	t.Parallel()

	patch, reason, err := (&machineconfig.ExtractionController{}).BuildConfigPatch("machine-1", nil)
	require.NoError(t, err)
	require.Empty(t, reason)
	require.Nil(t, patch)
}

func TestBuildConfigPatchRejectsConfigWithForbiddenFields(t *testing.T) {
	t.Parallel()

	// a v1alpha1 document carrying PKI / machine token is not a valid config patch and must not be surfaced as one
	observed := []byte(`version: v1alpha1
machine:
    type: worker
    token: aaaaaa.bbbbbbbbbbbbbbbb
    ca:
        crt: Zm9v
        key: YmFy
cluster: {}
`)

	patch, reason, err := (&machineconfig.ExtractionController{}).BuildConfigPatch("machine-1", observed)
	require.NoError(t, err)
	require.NotEmpty(t, reason, "a config with forbidden fields should report a reason")
	require.Nil(t, patch)
}
