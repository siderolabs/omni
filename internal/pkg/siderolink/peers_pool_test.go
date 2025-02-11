// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

func TestPeersPool(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)

	t.Cleanup(cancel)

	t.Run("test deduplication and refcounter", func(t *testing.T) {
		t.Parallel()

		wgHandler := fakeWireguardHandler{
			peers: map[string]wgtypes.Peer{},
		}

		pool := siderolink.NewPeersPool(zaptest.NewLogger(t), &wgHandler)

		id1 := "111111"
		id2 := "222222"

		require.NoError(t, pool.Add(ctx, &specs.SiderolinkSpec{
			NodePublicKey: id1,
		}, omni.NewCluster(resources.DefaultNamespace, "a").Metadata()))

		require.NoError(t, pool.Add(ctx, &specs.SiderolinkSpec{
			NodePublicKey: id1,
		}, omni.NewCluster(resources.DefaultNamespace, "a").Metadata()))

		require.NoError(t, pool.Add(ctx, &specs.SiderolinkSpec{
			NodePublicKey: id1,
		}, omni.NewCluster(resources.DefaultNamespace, "b").Metadata()))

		assert.Len(t, wgHandler.GetPeersMap(), 1)

		require.NoError(t, pool.Add(ctx, &specs.SiderolinkSpec{
			NodePublicKey: id2,
		}, omni.NewCluster(resources.DefaultNamespace, "b").Metadata()))

		assert.Len(t, wgHandler.GetPeersMap(), 2)

		require.NoError(t, pool.Remove(ctx, siderolink.GetPeerID(&specs.SiderolinkSpec{
			NodePublicKey: id2,
		}), omni.NewCluster(resources.DefaultNamespace, "b").Metadata()))

		require.NoError(t, pool.Remove(ctx, siderolink.GetPeerID(&specs.SiderolinkSpec{
			NodePublicKey: id1,
		}), omni.NewCluster(resources.DefaultNamespace, "b").Metadata()))

		assert.Len(t, wgHandler.GetPeersMap(), 1)

		require.NoError(t, pool.Remove(ctx, siderolink.GetPeerID(&specs.SiderolinkSpec{
			NodePublicKey: id1,
		}), omni.NewCluster(resources.DefaultNamespace, "a").Metadata()))

		assert.Len(t, wgHandler.GetPeersMap(), 0)
	})
}
