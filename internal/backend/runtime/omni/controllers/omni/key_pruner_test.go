// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

func TestKeyPrunerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(KeyPrunerSuite))
}

type KeyPrunerSuite struct {
	OmniSuite
}

const defaultExpirationTime = 8 * time.Second

func (suite *KeyPrunerSuite) setup() *clock.Mock {
	fakeClock := clock.NewMock()
	fakeClock.Set(time.Now())

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(
		omnictrl.NewKeyPrunerController(1*time.Second, omnictrl.WithClock(fakeClock)),
	))

	return fakeClock
}

func (suite *KeyPrunerSuite) TestRemoveExpiredKey() {
	fakeClock := suite.setup()

	publicKey := authres.NewPublicKey(testID)
	publicKey.TypedSpec().Value.Confirmed = true
	publicKey.TypedSpec().Value.Expiration = timestamppb.New(fakeClock.Now().Add(defaultExpirationTime))

	fakeClock.Add(4 * time.Second)

	publicKey2 := authres.NewPublicKey("testID2")
	publicKey2.TypedSpec().Value.Confirmed = true
	publicKey2.TypedSpec().Value.Expiration = timestamppb.New(fakeClock.Now().Add(defaultExpirationTime))

	suite.Assert().NoError(suite.state.Create(suite.ctx, publicKey, state.WithCreateOwner(new(omnictrl.KeyPrunerController{}).Name())))
	suite.Assert().NoError(suite.state.Create(suite.ctx, publicKey2, state.WithCreateOwner(new(omnictrl.KeyPrunerController{}).Name())))

	assertResource(&suite.OmniSuite, publicKey.Metadata(), func(*authres.PublicKey, *assert.Assertions) {})

	fakeClock.Add(3 * time.Second)

	assertResource(&suite.OmniSuite, publicKey.Metadata(), func(*authres.PublicKey, *assert.Assertions) {})

	fakeClock.Add(2 * time.Second)

	assertNoResource(&suite.OmniSuite, publicKey)

	// Check that second key is not removed
	assertResource(&suite.OmniSuite, publicKey2.Metadata(), func(*authres.PublicKey, *assert.Assertions) {})
}
