// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	xmaps "github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/omni/client/api/omni/specs"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

type fakeWireguardHandler struct {
	peers   map[string]wgtypes.Peer
	peersMu sync.Mutex
}

func (h *fakeWireguardHandler) SetupDevice(wireguard.DeviceConfig) error {
	return nil
}

func (h *fakeWireguardHandler) Run(context.Context, *zap.Logger) error {
	return nil
}

func (h *fakeWireguardHandler) Shutdown() error {
	return nil
}

func (h *fakeWireguardHandler) PeerEvent(_ context.Context, spec *specs.SiderolinkSpec, deleted bool) error {
	h.peersMu.Lock()
	defer h.peersMu.Unlock()

	if deleted {
		delete(h.peers, spec.NodePublicKey)
	} else {
		if _, ok := h.peers[spec.NodePublicKey]; ok {
			return fmt.Errorf("peer already exists")
		}

		h.peers[spec.NodePublicKey] = wgtypes.Peer{}
	}

	return nil
}

func (h *fakeWireguardHandler) Peers() ([]wgtypes.Peer, error) {
	h.peersMu.Lock()
	defer h.peersMu.Unlock()

	return xmaps.Values(h.peers), nil
}

func (h *fakeWireguardHandler) GetPeersMap() map[string]wgtypes.Peer {
	h.peersMu.Lock()
	defer h.peersMu.Unlock()

	return maps.Clone(h.peers)
}

type LinkStatusControllerSuite struct {
	OmniSuite
}

func (suite *LinkStatusControllerSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	wgHandler := &fakeWireguardHandler{
		peers: make(map[string]wgtypes.Peer),
	}

	logger := zaptest.NewLogger(suite.T())

	peers := siderolink.NewPeersPool(logger, wgHandler)

	suite.Require().NoError(
		suite.runtime.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.Link](peers)),
		suite.runtime.RegisterQController(omnictrl.NewLinkStatusController[*siderolinkres.PendingMachine](peers)),
	)

	severalReferences := "12345"

	spec := &specs.SiderolinkSpec{
		NodePublicKey: severalReferences,
	}

	link := siderolinkres.NewLink("1", spec)

	suite.Require().NoError(suite.state.Create(ctx, link))

	err := retry.Constant(time.Second*2, retry.WithUnits(time.Millisecond*50)).Retry(func() error {
		peers, err := wgHandler.Peers()
		if err != nil {
			return err
		}

		if len(peers) != 1 {
			return retry.ExpectedErrorf("expected 1 peer")
		}

		return nil
	})
	suite.Require().NoError(err)

	// share the peer
	pendingMachine := siderolinkres.NewPendingMachine("1", spec)

	suite.Require().NoError(suite.state.Create(ctx, pendingMachine))

	link = siderolinkres.NewLink("3", &specs.SiderolinkSpec{
		NodePublicKey: "bbbb",
	})

	suite.Require().NoError(suite.state.Create(ctx, link))

	err = retry.Constant(time.Second*2, retry.WithUnits(time.Millisecond*50)).Retry(func() error {
		var peers []wgtypes.Peer

		peers, err = wgHandler.Peers()
		if err != nil {
			return err
		}

		if len(peers) != 2 {
			return retry.ExpectedErrorf("expected 2 peers")
		}

		return nil
	})

	suite.Require().NoError(err)

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, siderolinkres.NewLink("3", nil).Metadata(), func(
		res *siderolinkres.Link,
	) error {
		res.TypedSpec().Value.NodePublicKey = "ffff"

		return nil
	})
	suite.Require().NoError(err)

	err = retry.Constant(time.Second*2, retry.WithUnits(time.Millisecond*50)).Retry(func() error {
		peers := wgHandler.GetPeersMap()

		_, hasOld := peers["bbbb"]
		if hasOld {
			return retry.ExpectedErrorf("expect peers not to have node public key bbbb")
		}

		_, ok := peers["ffff"]
		if !ok {
			return retry.ExpectedErrorf("expect peers to have node public key ffff")
		}

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.Destroy[*siderolinkres.Link](ctx, suite.T(), suite.state, []string{"3"})

	err = retry.Constant(time.Second*2, retry.WithUnits(time.Millisecond*50)).Retry(func() error {
		peers := wgHandler.GetPeersMap()

		_, hasOld := peers["ffff"]
		if hasOld {
			return retry.ExpectedErrorf("expect peers not to have node public key bbbb")
		}

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{
		"1/" + siderolinkres.LinkType,
		"1/" + siderolinkres.PendingMachineType,
	}, func(r *siderolinkres.LinkStatus, assert *assert.Assertions) {
		assert.Equal(r.TypedSpec().Value.LinkId, "1")
	})

	rtestutils.Destroy[*siderolinkres.Link](ctx, suite.T(), suite.state, []string{"1"})

	rtestutils.AssertNoResource[*siderolinkres.LinkStatus](ctx, suite.T(), suite.state, "1/"+siderolinkres.LinkType)

	_, referencedIsKept := wgHandler.GetPeersMap()[severalReferences]
	suite.Require().True(referencedIsKept)

	rtestutils.Destroy[*siderolinkres.PendingMachine](ctx, suite.T(), suite.state, []string{"1"})

	rtestutils.AssertNoResource[*siderolinkres.LinkStatus](ctx, suite.T(), suite.state, "1/"+siderolinkres.PendingMachineType)

	_, unreferencedAndRemoved := wgHandler.GetPeersMap()[severalReferences]

	suite.Require().False(unreferencedAndRemoved)
}

func TestLinkStatusControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(LinkStatusControllerSuite))
}
