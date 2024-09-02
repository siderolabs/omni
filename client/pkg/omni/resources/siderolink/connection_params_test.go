// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

func TestConnectionParamsKernelArgs(t *testing.T) {
	for _, tt := range []struct {
		spec         *specs.ConnectionParamsSpec
		name         string
		expectedArgs string
		expectError  bool
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

			args, err := siderolink.GetConnectionArgsForProvider(connectionParams, "test")

			if tt.expectError {
				require.Error(t, err, args)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedArgs, args)
		})
	}
}
