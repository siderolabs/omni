// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineSuite struct {
	OmniSuite
}

func (suite *MachineSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineController()))

	link1 := siderolink.NewLink("nodeID1", &specs.SiderolinkSpec{
		NodePublicKey: "HDAvKeXJAzYtOCaXPLWGASM2BgatwAnCSxrdcwXBxRk=",
		NodeSubnet:    netip.MustParsePrefix("fdae:41e4:649b:9303:7396:c9b3:213a:a86f/64").String(),
	})
	suite.Require().NoError(suite.state.Create(suite.ctx, link1))

	assertResource(
		&suite.OmniSuite,
		*omni.NewMachine(resources.DefaultNamespace, link1.Metadata().ID()).Metadata(),
		func(res *omni.Machine, _ *assert.Assertions) {
			require.Equal(suite.T(), "fdae:41e4:649b:9303:7396:c9b3:213a:a86f", res.TypedSpec().Value.ManagementAddress)
		},
	)

	link2 := siderolink.NewLink("nodeID2", &specs.SiderolinkSpec{
		NodePublicKey: "U522JKmQy/99NeMZa537ZHDlkJPv1SYaK0n8NTKIn3w=",
		NodeSubnet:    netip.MustParsePrefix("fdae:41e4:649b:9303:648e:f4a7:8ca4:ac75/64").String(),
	})

	suite.Assert().NoError(suite.assertNoResource(*omni.NewMachine(resources.DefaultNamespace, link2.Metadata().ID()).Metadata())())

	suite.Require().NoError(suite.state.Create(suite.ctx, link2))

	assertResource(
		&suite.OmniSuite,
		*omni.NewMachine(resources.DefaultNamespace, link2.Metadata().ID()).Metadata(),
		func(res *omni.Machine, _ *assert.Assertions) {
			require.Equal(suite.T(), "fdae:41e4:649b:9303:648e:f4a7:8ca4:ac75", res.TypedSpec().Value.ManagementAddress)
		},
	)
}

func TestMachineSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineSuite))
}
