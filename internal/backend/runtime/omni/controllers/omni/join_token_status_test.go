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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type JoinTokenStatusSuite struct {
	OmniSuite
}

func TestReconcile(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(JoinTokenStatusSuite))
}

func (suite *JoinTokenStatusSuite) TestReconcile() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewJoinTokenStatusController()))

	token := siderolink.NewJoinToken("token1")
	token.TypedSpec().Value.Name = "some name"

	suite.Require().NoError(suite.state.Create(ctx, token))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{token.Metadata().ID()},
		func(res *siderolink.JoinTokenStatus, assert *assert.Assertions) {
			assert.Equal(specs.JoinTokenStatusSpec_ACTIVE, res.TypedSpec().Value.State)
			assert.Equal(token.TypedSpec().Value.Name, res.TypedSpec().Value.Name)
			assert.False(res.TypedSpec().Value.IsDefault)
		},
	)

	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, token.Metadata(), func(res *siderolink.JoinToken) error {
		res.TypedSpec().Value.ExpirationTime = timestamppb.New(time.Now().Add(time.Second))

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{token.Metadata().ID()},
		func(res *siderolink.JoinTokenStatus, assert *assert.Assertions) {
			assert.Equal(specs.JoinTokenStatusSpec_ACTIVE, res.TypedSpec().Value.State)
		},
	)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{token.Metadata().ID()},
		func(res *siderolink.JoinTokenStatus, assert *assert.Assertions) {
			assert.Equal(specs.JoinTokenStatusSpec_EXPIRED, res.TypedSpec().Value.State)
		},
	)

	defaultToken := siderolink.NewDefaultJoinToken()
	defaultToken.TypedSpec().Value.TokenId = token.Metadata().ID()

	suite.Require().NoError(suite.state.Create(ctx, defaultToken))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{token.Metadata().ID()},
		func(res *siderolink.JoinTokenStatus, assert *assert.Assertions) {
			assert.True(res.TypedSpec().Value.IsDefault)
		},
	)

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, token.Metadata(), func(res *siderolink.JoinToken) error {
		res.TypedSpec().Value.ExpirationTime = nil
		res.TypedSpec().Value.Revoked = true

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{token.Metadata().ID()},
		func(res *siderolink.JoinTokenStatus, assert *assert.Assertions) {
			assert.Equal(specs.JoinTokenStatusSpec_REVOKED, res.TypedSpec().Value.State)
		},
	)

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, token.Metadata(), func(res *siderolink.JoinToken) error {
		res.TypedSpec().Value.ExpirationTime = nil
		res.TypedSpec().Value.Revoked = false

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{token.Metadata().ID()},
		func(res *siderolink.JoinTokenStatus, assert *assert.Assertions) {
			assert.Equal(specs.JoinTokenStatusSpec_ACTIVE, res.TypedSpec().Value.State)
		},
	)

	rtestutils.DestroyAll[*siderolink.JoinToken](ctx, suite.T(), suite.state)

	rtestutils.AssertNoResource[*siderolink.JoinTokenStatus](ctx, suite.T(), suite.state, token.Metadata().ID())
}
