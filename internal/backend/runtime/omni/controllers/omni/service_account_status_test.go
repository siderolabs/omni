// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

type ClusterServiceAccountStatusSuite struct {
	OmniSuite
}

func (suite *ClusterServiceAccountStatusSuite) TestReconcile() {
	require := suite.Require()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.startRuntime()
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewServiceAccountStatusController()))

	infraProviderServiceAccount := auth.NewIdentity("p1" + access.InfraProviderServiceAccountNameSuffix)
	infraProviderServiceAccount.TypedSpec().Value.UserId = "user2"
	infraProviderServiceAccount.Metadata().Labels().Set(auth.LabelIdentityTypeServiceAccount, "")

	serviceAccount := auth.NewIdentity("u1" + access.ServiceAccountNameSuffix)
	serviceAccount.TypedSpec().Value.UserId = "user1"
	serviceAccount.Metadata().Labels().Set(auth.LabelIdentityTypeServiceAccount, "")

	userIdentity := auth.NewIdentity("p1")
	userIdentity.TypedSpec().Value.UserId = "user1"

	user1 := auth.NewUser(serviceAccount.TypedSpec().Value.UserId)
	user1.TypedSpec().Value.Role = string(role.Admin)

	user2 := auth.NewUser(infraProviderServiceAccount.TypedSpec().Value.UserId)
	user2.TypedSpec().Value.Role = string(role.InfraProvider)

	publicKey := auth.NewPublicKey("asdf")
	publicKey.Metadata().Labels().Set(auth.LabelPublicKeyUserID, user1.Metadata().ID())
	publicKey.TypedSpec().Value.Identity = &specs.Identity{
		Email: serviceAccount.Metadata().ID(),
	}

	require.NoError(suite.state.Create(suite.ctx, infraProviderServiceAccount))
	require.NoError(suite.state.Create(suite.ctx, serviceAccount))
	require.NoError(suite.state.Create(suite.ctx, userIdentity))

	require.NoError(suite.state.Create(suite.ctx, user1))
	require.NoError(suite.state.Create(suite.ctx, user2))

	require.NoError(suite.state.Create(suite.ctx, publicKey))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{serviceAccount.Metadata().ID()}, func(res *auth.ServiceAccountStatus, assert *assert.Assertions) {
		assert.Equal(string(role.Admin), res.TypedSpec().Value.Role)
		assert.Len(res.TypedSpec().Value.PublicKeys, 1)
	})

	rtestutils.AssertNoResource[*auth.ServiceAccountStatus](ctx, suite.T(), suite.state, userIdentity.Metadata().ID())
	rtestutils.AssertNoResource[*auth.ServiceAccountStatus](ctx, suite.T(), suite.state, infraProviderServiceAccount.Metadata().ID())

	rtestutils.Destroy[*auth.PublicKey](ctx, suite.T(), suite.state, []string{publicKey.Metadata().ID()})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{serviceAccount.Metadata().ID()}, func(res *auth.ServiceAccountStatus, assert *assert.Assertions) {
		assert.Equal(string(role.Admin), res.TypedSpec().Value.Role)
		assert.Len(res.TypedSpec().Value.PublicKeys, 0)
	})

	require.NoError(suite.state.Create(suite.ctx, publicKey))

	rtestutils.Destroy[*auth.Identity](ctx, suite.T(), suite.state, []string{serviceAccount.Metadata().ID()})
	rtestutils.Destroy[*auth.PublicKey](ctx, suite.T(), suite.state, []string{publicKey.Metadata().ID()})
}

func TestClusterServiceAccountStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterServiceAccountStatusSuite))
}
