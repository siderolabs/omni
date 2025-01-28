// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// AssertNumberOfLinks verifies that machines are discovered by the SideroLink.
func AssertNumberOfLinks(testCtx context.Context, st state.State, expectedLinks int) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 90*time.Second)
		defer cancel()

		eventCh := make(chan safe.WrappedStateEvent[*siderolink.Link])

		require.NoError(t,
			safe.StateWatchKind(
				ctx,
				st,
				resource.NewMetadata(resources.DefaultNamespace, siderolink.LinkType, "", resource.VersionUndefined),
				eventCh,
				state.WithBootstrapContents(true),
			))

		linksFound := 0

		for linksFound < expectedLinks {
			select {
			case event := <-eventCh:
				switch event.Type() { //nolint:exhaustive
				case state.Created:
					linksFound++
				case state.Destroyed:
					linksFound--
				case state.Errored:
					require.NoError(t, event.Error())
				}

			case <-ctx.Done():
				t.Fatal("timeout")
			}

			t.Logf("links discovered: %d", linksFound)
		}
	}
}

// AssertLinksConnected verifies that all SideroLink connections are operational.
func AssertLinksConnected(testCtx context.Context, st state.State) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 320*time.Second)
		defer cancel()

		rtestutils.AssertAll(ctx, t, st,
			func(link *siderolink.Link, assert *assert.Assertions) {
				spec := link.TypedSpec().Value

				assert.True(spec.Connected)

				// the link counter for this link must be created
				msl, err := safe.StateGet[*omni.MachineStatusLink](ctx, st,
					omni.NewMachineStatusLink(resources.MetricsNamespace, link.Metadata().ID()).Metadata(),
				)
				assert.NoError(err)

				if msl != nil {
					// the link counter must have some bytes sent and received
					assert.Greater(msl.TypedSpec().Value.GetSiderolinkCounter().GetBytesSent(), int64(0), resourceDetails(link))
					assert.Greater(msl.TypedSpec().Value.GetSiderolinkCounter().GetBytesReceived(), int64(0), resourceDetails(link))
				}
			})
	}
}

// AssertMachinesMatchLinks verifies that all SideroLink connections are discovered as Machines.
func AssertMachinesMatchLinks(testCtx context.Context, st state.State) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
		defer cancel()

		rtestutils.AssertResources(ctx, t, st,
			rtestutils.ResourceIDs[*siderolink.Link](ctx, t, st),
			func(machine *omni.Machine, assert *assert.Assertions) {
				spec := machine.TypedSpec().Value

				assert.NotEmpty(spec.ManagementAddress, resourceDetails(machine))
				assert.True(spec.Connected, resourceDetails(machine))

				addressLabel, ok := machine.Metadata().Labels().Get(omni.MachineAddressLabel)
				assert.True(ok, resourceDetails(machine))
				assert.Equal(spec.ManagementAddress, addressLabel, resourceDetails(machine))
			})
	}
}
