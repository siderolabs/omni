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
	embeddedEndpoint := "http://" + net.JoinHostPort(siderolink.ListenHost, strconv.Itoa(port))

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewDiscoveryServiceConfigPatchController(port)))

	assertPatchData := func(clusterID string, contains ...string) {
		patchID := omnictrl.DiscoveryServiceConfigPatchPrefix + clusterID

		rtestutils.AssertResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, patchID, func(r *omni.ConfigPatch, assertion *assert.Assertions) {
			buffer, err := r.TypedSpec().Value.GetUncompressedData()
			assertion.NoError(err)

			defer buffer.Free()

			data := string(buffer.Data())

			for _, c := range contains {
				assertion.Contains(data, c)
			}
		})
	}

	// the discovery patch format is decided from the creation version, surfaced as InitialTalosVersion on
	// ClusterStatus by the ClusterStatusController, not the current running version.
	createCluster := func(id, initialTalosVersion string, useEmbedded, usePublic bool) {
		status := omni.NewClusterStatus(id)
		status.TypedSpec().Value.UseEmbeddedDiscoveryService = useEmbedded
		status.TypedSpec().Value.DisablePublicDiscoveryService = !usePublic
		status.TypedSpec().Value.InitialTalosVersion = initialTalosVersion
		suite.Require().NoError(suite.state.Create(suite.ctx, status))
	}

	// embedded only on a cluster created with Talos < 1.14 uses the legacy .cluster.discovery block
	createCluster("test-cluster-legacy", "1.9.0", true, false)
	assertPatchData("test-cluster-legacy", "discovery:", "registries:", embeddedEndpoint)

	// a cluster created with < 1.14 but with the current version 1.14 (upgraded) still uses the legacy block
	createCluster("test-cluster-upgraded", "1.13.0", true, false)
	assertPatchData("test-cluster-upgraded", "discovery:", "registries:", embeddedEndpoint)

	// embedded only on a cluster created with Talos 1.14+ overrides the base public "default" document
	createCluster("test-cluster-embedded", "1.14.0", true, false)
	assertPatchData("test-cluster-embedded", "kind: DiscoveryServiceConfig", "name: default", embeddedEndpoint)

	// embedded + public on a cluster created with Talos 1.14+ adds a distinct document alongside the base "default"
	createCluster("test-cluster-both", "1.14.0", true, true)
	assertPatchData("test-cluster-both", "kind: DiscoveryServiceConfig", "name: omni-embedded", embeddedEndpoint)

	// everything off on a cluster created with Talos 1.14+ removes the base public "default" document
	createCluster("test-cluster-disabled", "1.14.0", false, false)
	assertPatchData("test-cluster-disabled", "kind: DiscoveryServiceConfig", "name: default", "$patch: delete")

	// public only (the base config default) produces no patch
	createCluster("test-cluster-public", "1.14.0", false, true)

	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, omnictrl.DiscoveryServiceConfigPatchPrefix+"test-cluster-public")

	// switching an embedded-only cluster back to public only removes the patch
	_, err := safe.StateUpdateWithConflicts[*omni.ClusterStatus](suite.ctx, suite.state, omni.NewClusterStatus("test-cluster-embedded").Metadata(), func(res *omni.ClusterStatus) error {
		res.TypedSpec().Value.UseEmbeddedDiscoveryService = false
		res.TypedSpec().Value.DisablePublicDiscoveryService = false

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, omnictrl.DiscoveryServiceConfigPatchPrefix+"test-cluster-embedded")
}

func TestDiscoveryServiceConfigPatchSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(DiscoveryServiceConfigPatchSuite))
}
