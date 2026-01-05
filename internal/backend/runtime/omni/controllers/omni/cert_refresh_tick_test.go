// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type CertRefreshTickSuite struct {
	OmniSuite
}

func (suite *CertRefreshTickSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewCertRefreshTickController(100 * time.Millisecond)))

	watchCh := make(chan state.Event)

	suite.Require().NoError(suite.state.WatchKind(suite.ctx, system.NewCertRefreshTick("").Metadata(), watchCh))

	// wait for two ticks
	ticks := 0

	for {
		select {
		case ev := <-watchCh:
			suite.Require().Equal(state.Created, ev.Type)

			ticks++
		case <-time.After(500 * time.Millisecond):
			suite.Require().FailNow("timeout waiting for tick")
		}

		if ticks == 2 {
			break
		}
	}
}

func TestCertRefreshTickSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(CertRefreshTickSuite))
}
