// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine_test

import (
	"context"
	"io"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/machine"
)

// readMachineClient serves Read with fixed contents.
type readMachineClient struct {
	machineapi.MachineServiceClient

	contents string
}

func (c *readMachineClient) Read(context.Context, *machineapi.ReadRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[common.Data], error) {
	return &dataStream{data: []*common.Data{{Bytes: []byte(c.contents)}}}, nil
}

type dataStream struct {
	grpc.ClientStream

	data []*common.Data
}

func (s *dataStream) Recv() (*common.Data, error) {
	if len(s.data) == 0 {
		return nil, io.EOF
	}

	next := s.data[0]
	s.data = s.data[1:]

	return next, nil
}

func (s *dataStream) CloseSend() error { return nil }

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

			c := &client.Client{MachineClient: &readMachineClient{contents: "talos.platform=metal siderolink.api=grpc://omni:8090\n"}}
			info := machine.Info{MaintenanceMode: tt.maintenance}

			require.NoError(t, machine.PollKernelCmdlineFile(t.Context(), c, &info))
			require.Equal(t, new(tt.expected), info.KernelCmdline)
		})
	}
}
