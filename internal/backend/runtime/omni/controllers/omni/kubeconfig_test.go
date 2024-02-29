// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type KubeconfigSuite struct {
	OmniSuite
}

func (suite *KubeconfigSuite) TestReconcile() {
	require := suite.Require()

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(nil)))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewClusterLoadBalancerController(1000, 2000)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewKubeconfigController(2 * time.Second)))

	siderolinkConfig := siderolink.NewConfig(resources.DefaultNamespace)
	siderolinkConfig.TypedSpec().Value.ServerAddress = "fa87::1"
	require.NoError(suite.state.Create(suite.ctx, siderolinkConfig))

	clusterName := "talos-default-3"

	cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)
	cluster.TypedSpec().Value.TalosVersion = "1.2.0"
	require.NoError(suite.state.Create(suite.ctx, cluster))

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(cluster.Metadata().ID()))

	machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
	machineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	require.NoError(suite.state.Create(suite.ctx, machineSet))

	var firstKubeconfig []byte

	config := omni.NewKubeconfig(resources.DefaultNamespace, clusterName)
	assertResource(
		&suite.OmniSuite,
		*config.Metadata(),
		func(res *omni.Kubeconfig, _ *assert.Assertions) {
			spec := res.TypedSpec().Value

			_, err := clientcmd.RESTConfigFromKubeConfig(spec.Data)
			suite.Require().NoError(err)

			firstKubeconfig = spec.Data
		},
	)

	// wait 1 second so that the certificate is 50% stale
	time.Sleep(1 * time.Second)

	// issue a refresh tick
	suite.Require().NoError(suite.state.Create(suite.ctx, system.NewCertRefreshTick(resources.EphemeralNamespace, "refresh")))

	assertResource(
		&suite.OmniSuite,
		*config.Metadata(),
		func(res *omni.Kubeconfig, assertions *assert.Assertions) {
			spec := res.TypedSpec().Value

			// cert should be refreshed
			assertions.NotEqual(firstKubeconfig, spec.Data)
		},
	)

	rtestutils.Destroy[*omni.Cluster](suite.ctx, suite.T(), suite.state, []resource.ID{cluster.Metadata().ID()})
	assertNoResource(&suite.OmniSuite, config)
}

func TestKubeconfigSuite(t *testing.T) {
	suite.Run(t, new(KubeconfigSuite))
}
