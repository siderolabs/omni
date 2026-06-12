// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfigpatch_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	talossiderolink "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/machineconfigpatch"
)

const preservedPatchID = "000-preserved-machine-config-machine-1"

func encodeDocuments(t *testing.T, documents ...talosconfig.Document) []byte {
	t.Helper()

	ctr, err := container.New(documents...)
	require.NoError(t, err)

	data, err := ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)

	return data
}

func trustedRootsDocument() talosconfig.Document {
	doc := security.NewTrustedRootsConfigV1Alpha1()
	doc.MetaName = "my-enterprise-ca"
	doc.Certificates = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	return doc
}

func siderolinkDocument(t *testing.T) talosconfig.Document {
	t.Helper()

	doc := talossiderolink.NewConfigV1Alpha1()

	u, err := url.Parse("https://siderolink.example.org")
	require.NoError(t, err)

	doc.APIUrlConfig.URL = u

	return doc
}

func TestInitializerPreservesNonConnectionDocuments(t *testing.T) {
	ctx := context.Background()
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	extractor, err := machineconfigpatch.NewExtractor(st, zaptest.NewLogger(t))
	require.NoError(t, err)

	eventSink := runtimecfg.NewEventSinkV1Alpha1()
	eventSink.Endpoint = "127.0.0.1:1234"

	observed := encodeDocuments(t, trustedRootsDocument(), siderolinkDocument(t), eventSink)

	reason, err := extractor.Extract(ctx, "machine-1", observed)
	require.NoError(t, err)
	require.Empty(t, reason)

	patch, err := safe.StateGetByID[*omni.ConfigPatch](ctx, st, preservedPatchID)
	require.NoError(t, err)

	machineID, ok := patch.Metadata().Labels().Get(omni.LabelMachine)
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

func TestInitializerSkipsWhenOnlyConnectionDocuments(t *testing.T) {
	ctx := context.Background()
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	extractor, err := machineconfigpatch.NewExtractor(st, zaptest.NewLogger(t))
	require.NoError(t, err)

	observed := encodeDocuments(t, siderolinkDocument(t))

	reason, err := extractor.Extract(ctx, "machine-1", observed)
	require.NoError(t, err)
	require.Empty(t, reason)

	_, err = safe.StateGetByID[*omni.ConfigPatch](ctx, st, preservedPatchID)
	require.True(t, state.IsNotFoundError(err), "no preserved config patch should be created")
}

func TestInitializerSkipsOnEmptyConfig(t *testing.T) {
	ctx := context.Background()
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	extractor, err := machineconfigpatch.NewExtractor(st, zaptest.NewLogger(t))
	require.NoError(t, err)

	reason, err := extractor.Extract(ctx, "machine-1", nil)
	require.NoError(t, err)
	require.Empty(t, reason)

	_, err = safe.StateGetByID[*omni.ConfigPatch](ctx, st, preservedPatchID)
	require.True(t, state.IsNotFoundError(err), "no preserved config patch should be created")
}

func TestInitializerRejectsConfigWithForbiddenFields(t *testing.T) {
	ctx := context.Background()
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	extractor, err := machineconfigpatch.NewExtractor(st, zaptest.NewLogger(t))
	require.NoError(t, err)

	// a legacy v1alpha1 document carrying PKI / machine token is not a valid config patch and must not be surfaced as one
	observed := []byte(`version: v1alpha1
machine:
    type: worker
    token: aaaaaa.bbbbbbbbbbbbbbbb
    ca:
        crt: Zm9v
        key: YmFy
cluster: {}
`)

	reason, err := extractor.Extract(ctx, "machine-1", observed)
	require.NoError(t, err)
	require.NotEmpty(t, reason, "a config with forbidden fields should report a reason")

	_, err = safe.StateGetByID[*omni.ConfigPatch](ctx, st, preservedPatchID)
	require.True(t, state.IsNotFoundError(err), "no preserved config patch should be created for an invalid config")
}
