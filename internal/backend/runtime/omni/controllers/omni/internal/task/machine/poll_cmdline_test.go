// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/machine"
)

func TestPollKernelCmdlineFile(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		expected    string
		maintenance bool
	}{
		{
			name:     "running",
			expected: "talos.platform=metal siderolink.api=grpc://omni:8090",
		},
		{
			name:        "maintenance mode",
			maintenance: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &fakeTalosClient{files: map[string]string{"/proc/cmdline": "talos.platform=metal siderolink.api=grpc://omni:8090\n"}}
			info := machine.Info{MaintenanceMode: tt.maintenance}

			require.NoError(t, machine.PollKernelCmdlineFile(t.Context(), c, &info))
			require.Equal(t, new(tt.expected), info.KernelCmdline)
		})
	}
}
