// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machine"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
	siderolinkmanager "github.com/siderolabs/omni/internal/pkg/siderolink"
)

const (
	testID  = "testID"
	testID2 = "testID2"
)

func setupTest(t *testing.T, deltaCh chan siderolinkmanager.LinkCounterDeltas) func(ctx context.Context, testContext testutils.TestContext) {
	return func(ctx context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(machine.NewStatusLinkController(deltaCh)))
	}
}

func TestBasicMachineOnAndOff(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	deltaCh := make(chan siderolinkmanager.LinkCounterDeltas)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, setupTest(t, deltaCh),
		func(ctx context.Context, testContext testutils.TestContext) {
			m := rmock.Mock[*siderolink.Link](ctx, t, testContext.State,
				options.WithID(testID),
			)

			rmock.Mock[*omni.MachineStatus](ctx, t, testContext.State,
				options.WithID(testID),
				options.Modify(func(st *omni.MachineStatus) error {
					st.TypedSpec().Value.Connected = true

					return nil
				}),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, []string{testID}, func(r *omni.MachineStatusLink, assert *assert.Assertions) {
				assert.True(r.TypedSpec().Value.GetMessageStatus().GetConnected())
			})

			select {
			case deltaCh <- siderolinkmanager.LinkCounterDeltas{
				testID: siderolinkmanager.LinkCounterDelta{
					BytesSent:     15,
					BytesReceived: 20,
					LastAlive:     time.Unix(1257894000, 0),
				},
			}:
			case <-ctx.Done():
				t.Fatal("timed out sending delta")
			}

			rtestutils.AssertResource(ctx, t, testContext.State, omni.NewMachineStatusLink(m.Metadata().ID()).Metadata().ID(),
				func(r *omni.MachineStatusLink, assert *assert.Assertions) {
					statusVal := r.TypedSpec().Value

					assert.True(statusVal.GetMessageStatus().GetConnected())
					assert.EqualValues(15, statusVal.GetSiderolinkCounter().GetBytesSent())
					assert.EqualValues(20, statusVal.GetSiderolinkCounter().GetBytesReceived())
					assert.EqualValues(1257894000, statusVal.GetSiderolinkCounter().GetLastAlive().AsTime().Unix())
				},
			)

			rmock.Destroy[*omni.MachineStatus](ctx, t, testContext.State, []string{m.Metadata().ID()})
			rmock.Destroy[*siderolink.Link](ctx, t, testContext.State, []string{m.Metadata().ID()})

			rtestutils.AssertNoResource[*omni.MachineStatusLink](ctx, t, testContext.State, omni.NewMachineStatusLink(m.Metadata().ID()).Metadata().ID())
		},
	)
}

func TestTwoMachines(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	deltaCh := make(chan siderolinkmanager.LinkCounterDeltas)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, setupTest(t, deltaCh),
		func(ctx context.Context, testContext testutils.TestContext) {
			m := rmock.Mock[*siderolink.Link](ctx, t, testContext.State,
				options.WithID(testID),
			)

			rmock.Mock[*omni.MachineStatus](ctx, t, testContext.State,
				options.WithID(testID),
				options.Modify(func(st *omni.MachineStatus) error {
					st.TypedSpec().Value.Connected = true

					return nil
				}),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, []string{testID}, func(r *omni.MachineStatusLink, assert *assert.Assertions) {
				assert.True(r.TypedSpec().Value.GetMessageStatus().GetConnected())
			})

			m2 := rmock.Mock[*siderolink.Link](ctx, t, testContext.State,
				options.WithID(testID2),
			)

			rmock.Mock[*omni.MachineStatus](ctx, t, testContext.State,
				options.WithID(testID2),
				options.Modify(func(st *omni.MachineStatus) error {
					st.TypedSpec().Value.Connected = true

					return nil
				}),
			)

			rtestutils.AssertResources(ctx, t, testContext.State, []string{testID2}, func(r *omni.MachineStatusLink, assert *assert.Assertions) {
				assert.True(r.TypedSpec().Value.GetMessageStatus().GetConnected())
			})

			select {
			case deltaCh <- siderolinkmanager.LinkCounterDeltas{
				testID: siderolinkmanager.LinkCounterDelta{
					BytesSent:     15,
					BytesReceived: 20,
					LastAlive:     time.Unix(1257894000, 0),
				},
				testID2: siderolinkmanager.LinkCounterDelta{
					BytesSent:     16,
					BytesReceived: 21,
					LastAlive:     time.Unix(1257894001, 0),
				},
			}:
			case <-ctx.Done():
				t.Fatal("timed out sending delta")
			}

			rtestutils.AssertResource(ctx, t, testContext.State, omni.NewMachineStatusLink(m.Metadata().ID()).Metadata().ID(),
				func(r *omni.MachineStatusLink, assert *assert.Assertions) {
					statusVal := r.TypedSpec().Value

					assert.True(statusVal.GetMessageStatus().GetConnected())
					assert.EqualValues(15, statusVal.GetSiderolinkCounter().GetBytesSent())
					assert.EqualValues(20, statusVal.GetSiderolinkCounter().GetBytesReceived())
					assert.EqualValues(1257894000, statusVal.GetSiderolinkCounter().GetLastAlive().AsTime().Unix())
				},
			)

			rtestutils.AssertResource(ctx, t, testContext.State, omni.NewMachineStatusLink(m2.Metadata().ID()).Metadata().ID(),
				func(r *omni.MachineStatusLink, assert *assert.Assertions) {
					statusVal := r.TypedSpec().Value

					assert.True(statusVal.GetMessageStatus().GetConnected())
					assert.EqualValues(16, statusVal.GetSiderolinkCounter().GetBytesSent())
					assert.EqualValues(21, statusVal.GetSiderolinkCounter().GetBytesReceived())
					assert.EqualValues(1257894001, statusVal.GetSiderolinkCounter().GetLastAlive().AsTime().Unix())
				},
			)

			rmock.Destroy[*omni.MachineStatus](ctx, t, testContext.State, []string{m.Metadata().ID()})
			rmock.Destroy[*siderolink.Link](ctx, t, testContext.State, []string{m.Metadata().ID()})

			rtestutils.AssertNoResource[*omni.MachineStatusLink](ctx, t, testContext.State, omni.NewMachineStatusLink(m.Metadata().ID()).Metadata().ID())

			rtestutils.AssertResource(ctx, t, testContext.State, omni.NewMachineStatusLink(m2.Metadata().ID()).Metadata().ID(),
				func(r *omni.MachineStatusLink, assert *assert.Assertions) {
					statusVal := r.TypedSpec().Value

					assert.True(statusVal.GetMessageStatus().GetConnected())
				},
			)

			rmock.Destroy[*omni.MachineStatus](ctx, t, testContext.State, []string{m2.Metadata().ID()})
			rmock.Destroy[*siderolink.Link](ctx, t, testContext.State, []string{m2.Metadata().ID()})

			rtestutils.AssertNoResource[*omni.MachineStatusLink](ctx, t, testContext.State, omni.NewMachineStatusLink(m2.Metadata().ID()).Metadata().ID())
		},
	)
}

func TestNonExistingMachineDelta(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	deltaCh := make(chan siderolinkmanager.LinkCounterDeltas)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, setupTest(t, deltaCh),
		func(ctx context.Context, testContext testutils.TestContext) {
			select {
			case deltaCh <- siderolinkmanager.LinkCounterDeltas{
				testID: siderolinkmanager.LinkCounterDelta{
					BytesSent:     15,
					BytesReceived: 20,
					LastAlive:     time.Unix(1257894000, 0),
				},
			}:
			case <-ctx.Done():
				t.Fatal("timed out sending delta")
			}

			rtestutils.AssertNoResource[*omni.MachineStatusLink](ctx, t, testContext.State, omni.NewMachineStatusLink(resource.ID(testID)).Metadata().ID())
		},
	)
}
