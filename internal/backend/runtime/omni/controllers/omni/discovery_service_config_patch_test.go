// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

type DiscoveryServiceConfigPatchSuite struct {
	OmniSuite
}

func (suite *DiscoveryServiceConfigPatchSuite) TestReconcile() {
	suite.startRuntime()

	port := 1234
	controller := omnictrl.NewDiscoveryServiceConfigPatchController(port)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	clusterStatus := omni.NewClusterStatus("test-cluster-1")
	patchID := omnictrl.DiscoveryServiceConfigPatchPrefix + clusterStatus.Metadata().ID()

	clusterStatus.TypedSpec().Value.UseEmbeddedDiscoveryService = true

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	// assert that the new clusterStatus is marked to use the embedded discovery service
	rtestutils.AssertResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, patchID, func(r *omni.ConfigPatch, assertion *assert.Assertions) {
		buffer, err := r.TypedSpec().Value.GetUncompressedData()
		assertion.NoError(err)

		defer buffer.Free()

		data := string(buffer.Data())

		assertion.Contains(data, "http://"+net.JoinHostPort(siderolink.ListenHost, strconv.Itoa(port)))
	})

	_, err := safe.StateUpdateWithConflicts[*omni.ClusterStatus](suite.ctx, suite.state, clusterStatus.Metadata(), func(res *omni.ClusterStatus) error {
		res.TypedSpec().Value.UseEmbeddedDiscoveryService = false

		return nil
	})
	suite.Require().NoError(err)

	// assert that the config patch is removed
	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, patchID)
}

func TestDiscoveryServiceConfigPatchSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(DiscoveryServiceConfigPatchSuite))
}
