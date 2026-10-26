// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory_test

import (
	"strings"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/siderolink"
	imagefactoryinternal "github.com/siderolabs/omni/internal/backend/imagefactory"
)

func TestWithJoinConfig(t *testing.T) {
	t.Parallel()

	joinArgs := []string{
		"siderolink.api=grpc://127.0.0.1:8090?jointoken=new",
		"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
		"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
	}

	joinDocuments := `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: grpc://127.0.0.1:8090?jointoken=new
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
`

	oldDocuments := `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: grpc://127.0.0.1:8090?jointoken=old
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: remote-siem
url: tcp://192.168.1.10:5000
`

	userDocument := `# keep my comment
apiVersion: v1alpha1
kind: KmsgLogConfig
name: remote-siem
url: tcp://192.168.1.10:5000
`

	for _, tc := range []struct {
		name         string
		talosVersion string
		wantConfig   string
		wantArgs     []string
		wantContains []string
		wantMissing  []string
		base         schematic.Customization
	}{
		{
			name:         "kernel args below 1.12",
			talosVersion: "1.11.6",
			base:         schematic.Customization{ExtraKernelArgs: []string{"siderolink.api=grpc://old", "console=ttyS0"}},
			wantArgs:     append(append([]string{}, joinArgs...), "console=ttyS0"),
		},
		{
			name:         "kernel args move into the embedded configuration from 1.12",
			talosVersion: "1.12.0",
			base:         schematic.Customization{ExtraKernelArgs: []string{"siderolink.api=grpc://old", "console=ttyS0"}},
			wantArgs:     []string{"console=ttyS0"},
			wantConfig:   joinDocuments,
		},
		{
			name:         "entry holding a join arg with another arg is dropped",
			talosVersion: "1.13.0",
			base:         schematic.Customization{ExtraKernelArgs: []string{"console=ttyS0 siderolink.api=grpc://old"}},
			wantConfig:   joinDocuments,
		},
		{
			name:         "the user's documents are kept as written",
			talosVersion: "1.13.0",
			base:         schematic.Customization{EmbeddedMachineConfiguration: userDocument},
			wantConfig:   userDocument + "---\n" + joinDocuments,
		},
		{
			name:         "Omni-managed documents are replaced, the user's kmsg destination is kept",
			talosVersion: "1.13.0",
			base:         schematic.Customization{EmbeddedMachineConfiguration: oldDocuments},
			wantContains: []string{"name: remote-siem", "jointoken=new"},
			wantMissing:  []string{"jointoken=old"},
		},
		{
			name:         "Omni-managed documents are replaced in a config with CRLF line endings",
			talosVersion: "1.13.0",
			base:         schematic.Customization{EmbeddedMachineConfiguration: strings.ReplaceAll(oldDocuments, "\n", "\r\n")},
			wantContains: []string{"name: remote-siem", "jointoken=new"},
			wantMissing:  []string{"jointoken=old"},
		},
		{
			name:         "Omni-managed documents are replaced next to a commented separator",
			talosVersion: "1.13.0",
			base:         schematic.Customization{EmbeddedMachineConfiguration: strings.Replace(oldDocuments, "---\n", "--- # siem\n", 1)},
			wantContains: []string{"name: remote-siem", "jointoken=new"},
			wantMissing:  []string{"jointoken=old"},
		},
		{
			name:         "a document of a kind this Omni does not know is kept",
			talosVersion: "1.13.0",
			base:         schematic.Customization{EmbeddedMachineConfiguration: oldDocuments + "---\napiVersion: v1alpha1\nkind: FutureConfig\nname: x\n"},
			wantContains: []string{"name: remote-siem", "kind: FutureConfig", "jointoken=new"},
			wantMissing:  []string{"jointoken=old"},
		},
		{
			name:         "embedded join documents move back into kernel args below 1.12",
			talosVersion: "1.11.6",
			base:         schematic.Customization{EmbeddedMachineConfiguration: oldDocuments},
			wantArgs:     joinArgs,
			wantContains: []string{"name: remote-siem"},
			wantMissing:  []string{"SideroLinkConfig"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := imagefactoryinternal.WithJoinConfig(schematic.Schematic{Customization: tc.base}, joinArgs, []byte(joinDocuments), tc.talosVersion)
			require.NoError(t, err)

			assert.Equal(t, tc.wantArgs, result.Customization.ExtraKernelArgs)

			if tc.wantConfig != "" {
				assert.Equal(t, tc.wantConfig, result.Customization.EmbeddedMachineConfiguration)
			}

			for _, s := range tc.wantContains {
				assert.Contains(t, result.Customization.EmbeddedMachineConfiguration, s)
			}

			for _, s := range tc.wantMissing {
				assert.NotContains(t, result.Customization.EmbeddedMachineConfiguration, s)
			}

			again, err := imagefactoryinternal.WithJoinConfig(result, joinArgs, []byte(joinDocuments), tc.talosVersion)
			require.NoError(t, err)

			assert.Equal(t, result, again, "applying the join config again must not change the schematic")
		})
	}
}

func TestWithJoinConfigEmpty(t *testing.T) {
	t.Parallel()

	base := schematic.Schematic{Customization: schematic.Customization{ExtraKernelArgs: []string{"siderolink.api=grpc://old"}}}

	_, err := imagefactoryinternal.WithJoinConfig(base, nil, nil, "1.13.0")
	require.Error(t, err, "an empty join config must not drop the join config of the schematic")

	_, err = imagefactoryinternal.WithJoinConfig(base, []string{"siderolink.api=grpc://new"}, nil, "1.11.6")
	require.Error(t, err)
}

// newCA returns a fresh CA certificate (PEM).
func newCA(t *testing.T) string {
	t.Helper()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	return string(ca.CrtPEM)
}

// renderJoinDocuments returns the join documents Omni renders, with the given trusted roots, if any.
func renderJoinDocuments(t *testing.T, trustedRoots string) string {
	t.Helper()

	opts, err := siderolink.NewJoinOptions(
		siderolink.WithoutMachineAPIURL(),
		siderolink.WithEventSinkPort(8091),
		siderolink.WithLogServerPort(8092),
		siderolink.WithTrustedRoots(trustedRoots),
	)
	require.NoError(t, err)

	documents, err := opts.RenderJoinConfig()
	require.NoError(t, err)

	return string(documents)
}

// requireTrustedRoots checks that the embedded config loads and carries exactly the given Omni trusted roots, if any.
func requireTrustedRoots(t *testing.T, embedded, wantCertificates string) {
	t.Helper()

	cfg, err := configloader.NewFromBytes([]byte(embedded))
	require.NoError(t, err)

	var found []string

	for _, document := range cfg.Documents() {
		if trustedRoots, ok := document.(*security.TrustedRootsConfigV1Alpha1); ok && siderolink.IsJoinConfigDocument(document) {
			found = append(found, trustedRoots.Certificates)
		}
	}

	if wantCertificates == "" {
		require.Empty(t, found)

		return
	}

	require.Equal(t, []string{wantCertificates}, found)
}

func TestWithJoinConfigReplacesTrustedRoots(t *testing.T) {
	t.Parallel()

	joinArgs := []string{"siderolink.api=grpc://127.0.0.1:8090?jointoken=new"}
	userDocument := "apiVersion: v1alpha1\nkind: KmsgLogConfig\nname: remote-siem\nurl: tcp://192.168.1.10:5000\n"

	newCertificate := newCA(t)

	base := schematic.Schematic{Customization: schematic.Customization{EmbeddedMachineConfiguration: userDocument + "---\n" + renderJoinDocuments(t, newCA(t))}}

	rotated, err := imagefactoryinternal.WithJoinConfig(base, joinArgs, []byte(renderJoinDocuments(t, newCertificate)), "1.12.0")
	require.NoError(t, err)

	requireTrustedRoots(t, rotated.Customization.EmbeddedMachineConfiguration, newCertificate)
	assert.Contains(t, rotated.Customization.EmbeddedMachineConfiguration, "name: remote-siem")

	removed, err := imagefactoryinternal.WithJoinConfig(rotated, joinArgs, []byte(renderJoinDocuments(t, "")), "1.12.0")
	require.NoError(t, err)

	requireTrustedRoots(t, removed.Customization.EmbeddedMachineConfiguration, "")
	assert.Contains(t, removed.Customization.EmbeddedMachineConfiguration, "name: remote-siem")
}

func TestWithTrustedRoots(t *testing.T) {
	t.Parallel()

	providerDocuments := "apiVersion: v1alpha1\nkind: SideroLinkConfig\napiUrl: grpc://127.0.0.1:8090?jointoken=provider\n"

	oldCertificate := newCA(t)
	newCertificate := newCA(t)

	base := schematic.Schematic{Customization: schematic.Customization{EmbeddedMachineConfiguration: providerDocuments}}

	withOld, err := imagefactoryinternal.WithTrustedRoots(base, oldCertificate, "1.12.0")
	require.NoError(t, err)

	requireTrustedRoots(t, withOld.Customization.EmbeddedMachineConfiguration, oldCertificate)

	withNew, err := imagefactoryinternal.WithTrustedRoots(withOld, newCertificate, "1.12.0")
	require.NoError(t, err)

	requireTrustedRoots(t, withNew.Customization.EmbeddedMachineConfiguration, newCertificate)
	assert.Contains(t, withNew.Customization.EmbeddedMachineConfiguration, "jointoken=provider", "the provider's own join documents must stay")

	withoutRoots, err := imagefactoryinternal.WithTrustedRoots(withNew, "", "1.12.0")
	require.NoError(t, err)

	requireTrustedRoots(t, withoutRoots.Customization.EmbeddedMachineConfiguration, "")
	assert.Contains(t, withoutRoots.Customization.EmbeddedMachineConfiguration, "jointoken=provider", "the provider's own join documents must stay")

	kernelArgsJoin := schematic.Schematic{Customization: schematic.Customization{ExtraKernelArgs: []string{"siderolink.api=grpc://127.0.0.1:8090?jointoken=provider"}}}

	withKernelArgsJoin, err := imagefactoryinternal.WithTrustedRoots(kernelArgsJoin, newCertificate, "1.12.0")
	require.NoError(t, err)

	requireTrustedRoots(t, withKernelArgsJoin.Customization.EmbeddedMachineConfiguration, newCertificate)

	noJoin := schematic.Schematic{Customization: schematic.Customization{ExtraKernelArgs: []string{"console=ttyS0"}}}

	for _, unchanged := range []struct {
		name, trustedRoots, talosVersion string
		base                             schematic.Schematic
	}{
		{name: "below 1.12", base: base, trustedRoots: newCertificate, talosVersion: "1.11.6"},
		{name: "no trusted roots", base: base, talosVersion: "1.12.0"},
		{name: "no SideroLink config", base: noJoin, trustedRoots: newCertificate, talosVersion: "1.12.0"},
	} {
		result, err := imagefactoryinternal.WithTrustedRoots(unchanged.base, unchanged.trustedRoots, unchanged.talosVersion)
		require.NoError(t, err)

		assert.Equal(t, unchanged.base, result, unchanged.name)
	}
}
