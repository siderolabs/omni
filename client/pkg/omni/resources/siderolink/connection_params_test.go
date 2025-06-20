// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

func TestConnectionParamsKernelArgs(t *testing.T) {
	for _, tt := range []struct {
		spec         *specs.ConnectionParamsSpec
		name         string
		expectedArgs string
		requestID    string
		expectError  bool
		grpcTunnel   specs.GrpcTunnelMode
	}{
		{
			spec: &specs.ConnectionParamsSpec{
				Args:      "siderolink.api=grpc://127.0.0.1:8099?jointoken=abcd&grpc_tunnel=true",
				JoinToken: "abcd",
			},
			name: "insecure",
			expectedArgs: "siderolink.api=grpc://127.0.0.1:8099?grpc_tunnel=true&" +
				"jointoken=v1%3AeyJleHRyYV9kYXRhIjp7Im9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm9" +
				"2aWRlci1pZCI6InRlc3QifSwic2lnbmF0dXJlIjoiWTNpZ285V2xJSVZOWWpXZmgyWlg5NnpnWW5UQjlwWTI3ZEJaVnJwNDJMZz0ifQ%3D%3D",
		},
		{
			spec: &specs.ConnectionParamsSpec{
				Args:      "siderolink.api=https://127.0.0.1:8099?jointoken=abcd a=b",
				JoinToken: "abcd",
			},
			name: "secure",
			expectedArgs: "siderolink.api=https://127.0.0.1:8099?jointoken=v1%3AeyJleHRyYV9kYXRhIjp7Im" +
				"9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm92aWRlci1pZCI6InRlc3QifSwic2lnbmF0dXJlIjoiWTNpZ285V2xJSVZ" +
				"OWWpXZmgyWlg5NnpnWW5UQjlwWTI3ZEJaVnJwNDJMZz0ifQ%3D%3D a=b",
		},
		{
			spec: &specs.ConnectionParamsSpec{
				Args:      "siderolink.api=https://127.0.0.1:8099?jointoken=abcd a=b",
				JoinToken: "abcd",
			},
			name: "secure",
			expectedArgs: "siderolink.api=https://127.0.0.1:8099?jointoken=v1%3AeyJleHRyYV9kYXRhIjp7Im9tbmk" +
				"uc2lkZXJvLmRldi9pbmZyYS1wcm92aWRlci1pZCI6InRlc3QiLCJvbW5pLnNpZGVyby5kZXYvbWFjaGluZS1yZXF1ZXN0Ijo" +
				"icmVxdWVzdDEifSwic2lnbmF0dXJlIjoiellrTFRwOUVIanlwTGxrZG1mZjV2Z1A1SERXRktUUXBGR281REp2TDV4MD0ifQ%3D%3D a=b",
			requestID: "request1",
		},
		{
			spec: &specs.ConnectionParamsSpec{
				Args:      "siderolink.api=https://127.0.0.1:8099?jointoken=abcd a=b",
				JoinToken: "abcd",
			},
			name: "secure",
			expectedArgs: "siderolink.api=https://127.0.0.1:8099?grpc_tunnel=true&jointoken=v1%3AeyJleHRyYV9kYXRhIjp7Im" +
				"9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm92aWRlci1pZCI6InRlc3QifSwic2lnbmF0dXJlIjoiWTNpZ285V2xJSVZ" +
				"OWWpXZmgyWlg5NnpnWW5UQjlwWTI3ZEJaVnJwNDJMZz0ifQ%3D%3D a=b",
			grpcTunnel: specs.GrpcTunnelMode_ENABLED,
		},
		{
			spec:        &specs.ConnectionParamsSpec{},
			name:        "no params",
			expectError: true,
		},
		{
			spec: &specs.ConnectionParamsSpec{
				Args: "test.param=a",
			},
			name:        "no siderolink api",
			expectError: true,
		},
		{
			spec: &specs.ConnectionParamsSpec{
				Args: "siderolink.api",
			},
			name:        "incorrect siderolink api",
			expectError: true,
		},
		{
			spec: &specs.ConnectionParamsSpec{
				Args: "siderolink.api=%D3:",
			},
			name:        "invalid siderolink URL",
			expectError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			connectionParams := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
			connectionParams.TypedSpec().Value = tt.spec

			opts := []siderolink.JoinConfigOption{}

			if tt.requestID != "" {
				opts = append(opts, siderolink.WithEncodeRequestID(tt.requestID))
			}

			args, err := siderolink.GetConnectionArgsForProvider(connectionParams, "test", tt.grpcTunnel, opts...)

			if tt.expectError {
				require.Error(t, err, args)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestConnectionParamsJoinConfig(t *testing.T) {
	for _, tt := range []struct {
		spec           *specs.ConnectionParamsSpec
		name           string
		expectedConfig string
		expectError    bool
		grpcTunnel     specs.GrpcTunnelMode
	}{
		{
			spec: &specs.ConnectionParamsSpec{
				Args:        "siderolink.api=https://127.0.0.1:8099?jointoken=abcd a=b",
				JoinToken:   "abcd",
				ApiEndpoint: "https://127.0.0.1:8099",
				LogsPort:    8093,
				EventsPort:  8094,
			},
			name: "secure",
			//nolint:lll
			expectedConfig: `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: https://127.0.0.1:8099?grpc_tunnel=true&jointoken=v1%3AeyJleHRyYV9kYXRhIjp7Im9tbmkuc2lkZXJvLmRldi9pbmZyYS1wcm92aWRlci1pZCI6InRlc3QifSwic2lnbmF0dXJlIjoiWTNpZ285V2xJSVZOWWpXZmgyWlg5NnpnWW5UQjlwWTI3ZEJaVnJwNDJMZz0ifQ%3D%3D
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8094'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8093
`,
			grpcTunnel: specs.GrpcTunnelMode_ENABLED,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			connectionParams := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
			connectionParams.TypedSpec().Value = tt.spec

			config, err := siderolink.GetJoinConfigForProvider(connectionParams, "test", tt.grpcTunnel)

			if tt.expectError {
				require.Error(t, err, config)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedConfig, config)

			dec := yaml.NewDecoder(bytes.NewBufferString(config))

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

func TestKernelArgsWithGRPCTunnelMode(t *testing.T) {
	for _, tt := range []struct {
		name     string
		args     string
		expected []string
		value    bool
	}{
		{
			name:  "no arg",
			args:  "aaaa siderolink.api=grpc://127.0.0.1:8099?jointoken=abcd cccc",
			value: true,
			expected: []string{
				"aaaa",
				"siderolink.api=grpc://127.0.0.1:8099?grpc_tunnel=true&jointoken=abcd", // this also tests the reordering of the query parameters alphabetically by key
				"cccc",
			},
		},
		{
			name:  "true->false",
			args:  "aaaa siderolink.api=grpc://127.0.0.1:8099?jointoken=abcd&grpc_tunnel=false cccc",
			value: true,
			expected: []string{
				"aaaa",
				"siderolink.api=grpc://127.0.0.1:8099?grpc_tunnel=true&jointoken=abcd", // this also tests the reordering of the query parameters alphabetically by key
				"cccc",
			},
		},
		{
			name:  "false->true",
			args:  "siderolink.api=grpc://127.0.0.1:8099?jointoken=abcd&xyz=123",
			value: true,
			expected: []string{
				"siderolink.api=grpc://127.0.0.1:8099?grpc_tunnel=true&jointoken=abcd&xyz=123",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			connectionParams := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
			connectionParams.TypedSpec().Value = &specs.ConnectionParamsSpec{
				Args: tt.args,
			}

			args, err := siderolink.KernelArgsWithGRPCRTunnelMode(connectionParams, tt.value)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, args)
		})
	}
}
