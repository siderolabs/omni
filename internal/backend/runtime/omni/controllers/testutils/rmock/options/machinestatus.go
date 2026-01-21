// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package options

import (
	"context"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

// WithMachineServices creates machine service for each machine status resource.
func WithMachineServices(ctx context.Context, ms *testutils.MachineServices) MockOption {
	return Modify(func(res *omni.MachineStatus) error {
		service := ms.Create(ctx, res.Metadata().ID())

		res.TypedSpec().Value.ManagementAddress = service.SocketConnectionString

		return nil
	})
}
