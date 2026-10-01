// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory_test

import (
	"strings"
	"testing"

	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/stretchr/testify/assert"

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
			name:         "entry holding a join arg with another arg keeps the other arg",
			talosVersion: "1.13.0",
			base:         schematic.Customization{ExtraKernelArgs: []string{"console=ttyS0 siderolink.api=grpc://old"}},
			wantArgs:     []string{"console=ttyS0"},
			wantConfig:   joinDocuments,
		},
		{
			name:         "the user's documents are kept as written",
			talosVersion: "1.13.0",
			base:         schematic.Customization{EmbeddedMachineConfiguration: userDocument},
			wantConfig:   userDocument + "---\n" + joinDocuments,
		},
		{
			name:         "Omni's documents are replaced, the user's kmsg destination is kept",
			talosVersion: "1.13.0",
			base:         schematic.Customization{EmbeddedMachineConfiguration: oldDocuments},
			wantContains: []string{"name: remote-siem", "jointoken=new"},
			wantMissing:  []string{"jointoken=old"},
		},
		{
			name:         "Omni's documents are replaced in a config with CRLF line endings",
			talosVersion: "1.13.0",
			base:         schematic.Customization{EmbeddedMachineConfiguration: strings.ReplaceAll(oldDocuments, "\n", "\r\n")},
			wantContains: []string{"name: remote-siem", "jointoken=new"},
			wantMissing:  []string{"jointoken=old"},
		},
		{
			name:         "Omni's documents are replaced next to a commented separator",
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

			result := imagefactoryinternal.WithJoinConfig(schematic.Schematic{Customization: tc.base}, joinArgs, []byte(joinDocuments), tc.talosVersion)

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

			again := imagefactoryinternal.WithJoinConfig(result, joinArgs, []byte(joinDocuments), tc.talosVersion)

			assert.Equal(t, result, again, "applying the join config again must not change the schematic")
		})
	}
}
