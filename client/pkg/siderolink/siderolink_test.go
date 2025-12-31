// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

type machine struct {
	providerID    string
	requestID     string
	useGRPCTUnnel bool
}

func TestConnectionParamsKernelArgs(t *testing.T) {
	//nolint:govet
	for _, tt := range []struct {
		machine       *machine
		expectedArgs  []string
		joinToken     string
		apiURL        string
		name          string
		providerID    string
		grpcTunnel    specs.GrpcTunnelMode
		eventSinkPort int
		logServerPort int
		expectError   bool
		extraOpts     []siderolink.JoinConfigOption
	}{
		{
			grpcTunnel: specs.GrpcTunnelMode_ENABLED,
			apiURL:     "grpc://127.0.0.1:8099",
			joinToken:  "abcd",
			name:       "insecure",
			expectedArgs: []string{
				"siderolink.api=grpc://127.0.0.1:8099?grpc_tunnel=true&jointoken=abcd",
				"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
				"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
			},
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			joinToken: "abcd",
			name:      "secure",
			apiURL:    "https://127.0.0.1:8099",
			expectedArgs: []string{
				"siderolink.api=https://127.0.0.1:8099?jointoken=abcd",
				"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
				"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
			},
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			joinToken: "abcd",
			name:      "with machine and request ID",
			apiURL:    "https://127.0.0.1:8099",
			expectedArgs: []string{
				"siderolink.api=https://127.0.0.1:8099?grpc_tunnel=true&jointoken=v2%3AeyJleHRyYV9kYXRhIjp7Im9tbmk" +
					"uc2lkZXJvLmRldi9pbmZyYS1wcm92aWRlci1pZCI6InRlc3QiLCJvbW5pLnNpZGVyby5kZXYvbWFjaGluZS1yZXF1ZXN0Ijo" +
					"icmVxdWVzdDEifSwic2lnbmF0dXJlIjoiellrTFRwOUVIanlwTGxrZG1mZjV2Z1A1SERXRktUUXBGR281REp2TDV4MD0ifQ%3D%3D",
				"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
				"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
			},
			machine: &machine{
				requestID:     "request1",
				providerID:    "test",
				useGRPCTUnnel: true,
			},
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			name:          "no token",
			apiURL:        "https://127.0.0.1:8099",
			expectError:   true,
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			joinToken: "abcd",
			name:      "with provider",
			apiURL:    "https://127.0.0.1:8099",
			expectedArgs: []string{
				"siderolink.api=https://127.0.0.1:8099?jointoken=v2%3AeyJleHRyYV9kYXRhIjp7Im9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm92a" +
					"WRlci1pZCI6InRlc3QifSwic2lnbmF0dXJlIjoiWTNpZ285V2xJSVZOWWpXZmgyWlg5NnpnWW5UQjlwWTI3ZEJaVnJwNDJMZz0ifQ%3D%3D",
				"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
				"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
			},
			providerID:    "test",
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			joinToken: "abcd",
			name:      "with all params",
			apiURL:    "https://127.0.0.1:8099",
			expectedArgs: []string{
				"siderolink.api=https://127.0.0.1:8099?jointoken=abcd",
				"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
				"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
			},
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			joinToken: "v1:eyJleHRyYV9kYXRhIjp7Im9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm92a" +
				"WRlci1pZCI6InRlc3QifSwic2lnbmF0dXJlIjoiWTNpZ285V2xJSVZOWWpXZmgyWlg5NnpnWW5UQjlwWTI3ZEJaVnJwNDJMZz0ifQ==",
			name:   "already encoded",
			apiURL: "https://127.0.0.1:8099",
			expectedArgs: []string{
				"siderolink.api=https://127.0.0.1:8099?jointoken=" +
					"v1%3AeyJleHRyYV9kYXRhIjp7Im9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm92a" +
					"WRlci1pZCI6InRlc3QifSwic2lnbmF0dXJlIjoiWTNpZ285V2xJSVZOWWpXZmgyWlg5NnpnWW5UQjlwWTI3ZEJaVnJwNDJMZz0ifQ%3D%3D",
				"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
				"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
			},
			eventSinkPort: 8091,
			logServerPort: 8092,
			machine: &machine{
				providerID: "test",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			options := []siderolink.JoinConfigOption{
				siderolink.WithMachineAPIURL(tt.apiURL),
				siderolink.WithEventSinkPort(tt.eventSinkPort),
				siderolink.WithLogServerPort(tt.logServerPort),
				siderolink.WithJoinToken(tt.joinToken),
			}

			if tt.machine != nil {
				m := omni.NewMachine("machine-1")
				m.TypedSpec().Value.UseGrpcTunnel = tt.machine.useGRPCTUnnel

				if tt.machine.requestID != "" {
					m.Metadata().Labels().Set(omni.LabelMachineRequest, tt.machine.requestID)
				}

				if tt.machine.providerID != "" {
					m.Metadata().Labels().Set(omni.LabelInfraProviderID, tt.machine.providerID)
				}

				options = append(options, siderolink.WithMachine(m))
			}

			if tt.providerID != "" {
				options = append(options, siderolink.WithProvider(infra.NewProvider(tt.providerID)))
			}

			if tt.grpcTunnel == specs.GrpcTunnelMode_ENABLED {
				options = append(options, siderolink.WithGRPCTunnel(true))
			}

			if tt.extraOpts != nil {
				options = append(options, tt.extraOpts...)
			}

			opts, err := siderolink.NewJoinOptions(options...)

			if tt.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			args := opts.GetKernelArgs()
			require.Equal(t, tt.expectedArgs, args)
		})
	}
}

//nolint:gocognit
func TestConnectionParamsJoinConfig(t *testing.T) {
	for _, tt := range []struct {
		machine        *machine
		expectedConfig string
		joinToken      string
		apiURL         string
		name           string
		providerID     string
		extraDocs      []config.Document
		eventSinkPort  int
		logServerPort  int
		expectError    bool
		grpcTunnel     specs.GrpcTunnelMode
	}{
		{
			grpcTunnel:    specs.GrpcTunnelMode_ENABLED,
			apiURL:        "grpc://127.0.0.1:8099",
			joinToken:     "abcd",
			name:          "insecure",
			eventSinkPort: 8091,
			logServerPort: 8092,
			expectedConfig: `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: grpc://127.0.0.1:8099?grpc_tunnel=true&jointoken=abcd
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
`,
		},
		{
			joinToken:     "abcd",
			name:          "secure",
			apiURL:        "https://127.0.0.1:8099",
			eventSinkPort: 8091,
			logServerPort: 8092,
			expectedConfig: `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: https://127.0.0.1:8099?jointoken=abcd
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
`,
		},
		{
			joinToken: "abcd",
			name:      "with machine and request ID",
			apiURL:    "https://127.0.0.1:8099",
			expectedConfig: "apiVersion: v1alpha1\n" +
				"kind: SideroLinkConfig\n" +
				"apiUrl: https://127.0.0.1:8099?grpc_tunnel=true&jointoken=v2%3AeyJleHRyYV9kYXRhIjp7Im9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm" +
				"92aWRlci1pZCI6InRlc3QiLCJvbW5pLnNpZGVyby5kZXYvbWFjaGluZS1yZXF1ZXN0IjoicmVxdWVzdDEifSwic2lnbmF0dXJlIjoiellrTFRwOUVIanl" +
				"wTGxrZG1mZjV2Z1A1SERXRktUUXBGR281REp2TDV4MD0ifQ%3D%3D\n" +
				"---\n" +
				"apiVersion: v1alpha1\n" +
				"kind: EventSinkConfig\n" +
				"endpoint: '[fdae:41e4:649b:9303::1]:8091'\n" +
				"---\n" +
				"apiVersion: v1alpha1\n" +
				"kind: KmsgLogConfig\n" +
				"name: omni-kmsg\n" +
				"url: tcp://[fdae:41e4:649b:9303::1]:8092\n",
			machine: &machine{
				requestID:     "request1",
				providerID:    "test",
				useGRPCTUnnel: true,
			},
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			joinToken: "abcd",
			name:      "with provider",
			apiURL:    "https://127.0.0.1:8099",
			expectedConfig: "apiVersion: v1alpha1\n" +
				"kind: SideroLinkConfig\n" +
				"apiUrl: https://127.0.0.1:8099?jointoken=v2%3AeyJleHRyYV9kYXRhIjp7Im9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm92a" +
				"WRlci1pZCI6InRlc3QifSwic2lnbmF0dXJlIjoiWTNpZ285V2xJSVZOWWpXZmgyWlg5NnpnWW5UQjlwWTI3ZEJaVnJwNDJMZz0ifQ%3D%3D\n" +
				"---\n" +
				"apiVersion: v1alpha1\n" +
				"kind: EventSinkConfig\n" +
				"endpoint: '[fdae:41e4:649b:9303::1]:8091'\n" +
				"---\n" +
				"apiVersion: v1alpha1\n" +
				"kind: KmsgLogConfig\n" +
				"name: omni-kmsg\n" +
				"url: tcp://[fdae:41e4:649b:9303::1]:8092\n",
			providerID:    "test",
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			joinToken: "abcd",
			name:      "with all params",
			apiURL:    "https://127.0.0.1:8099",
			expectedConfig: `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: https://127.0.0.1:8099?jointoken=abcd
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
`,
			eventSinkPort: 8091,
			logServerPort: 8092,
		},
		{
			expectError: true,
		},
		{
			joinToken: "abcd",
			name:      "with all params",
			apiURL:    "https://127.0.0.1:8099",
			expectedConfig: `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: https://127.0.0.1:8099?jointoken=abcd
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
---
apiVersion: v1alpha1
kind: TrustedRootsConfig
name: ""
certificates: ""
`,
			eventSinkPort: 8091,
			logServerPort: 8092,
			extraDocs: []config.Document{
				security.NewTrustedRootsConfigV1Alpha1(),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			options := []siderolink.JoinConfigOption{
				siderolink.WithMachineAPIURL(tt.apiURL),
				siderolink.WithEventSinkPort(tt.eventSinkPort),
				siderolink.WithLogServerPort(tt.logServerPort),
				siderolink.WithJoinToken(tt.joinToken),
			}

			if tt.machine != nil {
				m := omni.NewMachine("machine-1")
				m.TypedSpec().Value.UseGrpcTunnel = tt.machine.useGRPCTUnnel

				if tt.machine.requestID != "" {
					m.Metadata().Labels().Set(omni.LabelMachineRequest, tt.machine.requestID)
				}

				if tt.machine.providerID != "" {
					m.Metadata().Labels().Set(omni.LabelInfraProviderID, tt.machine.providerID)
				}

				options = append(options, siderolink.WithMachine(m))
			}

			if tt.providerID != "" {
				options = append(options, siderolink.WithProvider(infra.NewProvider(tt.providerID)))
			}

			if tt.grpcTunnel == specs.GrpcTunnelMode_ENABLED {
				options = append(options, siderolink.WithGRPCTunnel(true))
			}

			opts, err := siderolink.NewJoinOptions(options...)
			if tt.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			config, err := opts.RenderJoinConfig(tt.extraDocs...)
			if tt.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedConfig, string(config))

			dec := yaml.NewDecoder(bytes.NewBuffer(config))

			for i := range 5 {
				var dest any

				err = dec.Decode(&dest)
				if errors.Is(err, io.EOF) {
					break
				}

				require.NoError(t, err)

				if i > 3 {
					t.Errorf("found extra documents")

					t.FailNow()
				}
			}
		})
	}
}
