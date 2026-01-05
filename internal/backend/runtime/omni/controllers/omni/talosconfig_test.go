// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type TalosConfigSuite struct {
	OmniSuite
}

func (suite *TalosConfigSuite) TestReconcile() {
	require := suite.Require()

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(2 * time.Second)))

	clusterName := "talos-default-2"

	cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)
	cluster.TypedSpec().Value.TalosVersion = "1.2.0"
	require.NoError(suite.state.Create(suite.ctx, cluster))

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(cluster.Metadata().ID()))

	machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
	machineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	require.NoError(suite.state.Create(suite.ctx, machineSet))

	var firstCrt string

	config := omni.NewTalosConfig(resources.DefaultNamespace, clusterName)
	assertResource(
		&suite.OmniSuite,
		*config.Metadata(),
		func(res *omni.TalosConfig, _ *assert.Assertions) {
			spec := res.TypedSpec().Value

			cfg := &clientconfig.Config{
				Context: clusterName,
				Contexts: map[string]*clientconfig.Context{
					clusterName: {
						Endpoints: []string{"127.0.0.1"},
						CA:        spec.Ca,
						Crt:       spec.Crt,
						Key:       spec.Key,
					},
				},
			}
			_, err := client.New(suite.ctx, client.WithConfig(cfg))
			suite.Require().NoError(err)

			firstCrt = spec.Crt
		},
	)

	// wait 1 second so that the certificate is 50% stale
	time.Sleep(1 * time.Second)

	// issue a refresh tick
	suite.Require().NoError(suite.state.Create(suite.ctx, system.NewCertRefreshTick("refresh")))

	assertResource(
		&suite.OmniSuite,
		*config.Metadata(),
		func(res *omni.TalosConfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			// cert should be refreshed
			assertions.NotEqual(firstCrt, spec.Crt)
		},
	)

	rtestutils.Destroy[*omni.Cluster](suite.ctx, suite.T(), suite.state, []resource.ID{cluster.Metadata().ID()})
	assertNoResource(&suite.OmniSuite, config)
}

func TestTalosConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(TalosConfigSuite))
}
