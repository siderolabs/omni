// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package installdisk_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/installdisk"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

// TestStatusController verifies the controller wiring around Resolve. The status exists for
// every machine from its machine status alone, a created MachineInstallDiskConfig re-resolves it
// (proving the config is a mapped input), and deleting the config falls back to the automatic
// default. The cluster label of the machine is synced to the status.
func TestStatusController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(installdisk.NewStatusController()))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			st := tc.State

			machineStatus := rmock.Mock[*omni.MachineStatus](
				ctx, t, st, options.WithID("machine-1"),
				options.Modify(func(res *omni.MachineStatus) error {
					res.Metadata().Labels().Set(omni.LabelCluster, "cluster-1")

					res.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{
						Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
							{DevPath: "/dev/sda", Size: 8e9},
							{DevPath: "/dev/sdb", Size: 10e9},
						},
					}

					return nil
				}),
			)

			id := machineStatus.Metadata().ID()

			// the status exists with the automatic default before any config is created
			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.MachineInstallDiskStatus, a *assert.Assertions) {
				a.Equal("/dev/sda", res.TypedSpec().Value.Disk)

				var candidates []string

				for _, disk := range res.TypedSpec().Value.Disks {
					if disk.SkipReason == "" {
						candidates = append(candidates, disk.DevPath)
					}
				}

				a.Equal([]string{"/dev/sda", "/dev/sdb"}, candidates)

				cluster, _ := res.Metadata().Labels().Get(omni.LabelCluster)
				a.Equal("cluster-1", cluster)
			})

			// a user selection re-resolves the status
			config := omni.NewMachineInstallDiskConfig(id)
			config.TypedSpec().Value.Disk = "/dev/sdb"
			require.NoError(t, st.Create(ctx, config))

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.MachineInstallDiskStatus, a *assert.Assertions) {
				a.Equal("/dev/sdb", res.TypedSpec().Value.Disk)
			})

			// deleting the selection falls back to the automatic default
			rtestutils.Destroy[*omni.MachineInstallDiskConfig](ctx, t, st, []string{id})

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.MachineInstallDiskStatus, a *assert.Assertions) {
				a.Equal("/dev/sda", res.TypedSpec().Value.Disk)
			})
		},
	)
}
