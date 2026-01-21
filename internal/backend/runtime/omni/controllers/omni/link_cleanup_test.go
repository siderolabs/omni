// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

func TestLinkCleanup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	controller := omni.NewLinkCleanupController()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)

	rt, err := runtime.NewRuntime(st, logger, omniruntime.RuntimeCacheOptions()...)
	require.NoError(t, err)

	require.NoError(t, rt.RegisterController(controller))

	errCh := make(chan error)

	go func() {
		errCh <- rt.Run(ctx)
	}()

	id := "test-link-cleanup"

	link := siderolink.NewLink(id, nil)

	usage := siderolink.NewJoinTokenUsage(id)
	uniqueToken := siderolink.NewNodeUniqueToken(id)
	labels := omnires.NewMachineLabels(id)

	require.NoError(t, st.Create(ctx, link))
	require.NoError(t, st.Create(ctx, usage))
	require.NoError(t, st.Create(ctx, uniqueToken))
	require.NoError(t, st.Create(ctx, labels))

	rtestutils.AssertResource[*siderolink.Link](ctx, t, st, link.Metadata().ID(), func(res *siderolink.Link, assertion *assert.Assertions) {
		assertion.False(res.Metadata().Finalizers().Empty(), "link should have the cleanup controller finalizer set")
	})

	rtestutils.Destroy[*siderolink.Link](ctx, t, st, []string{id})

	rtestutils.AssertNoResource[*siderolink.JoinTokenUsage](ctx, t, st, id)
	rtestutils.AssertNoResource[*siderolink.NodeUniqueToken](ctx, t, st, id)
	rtestutils.AssertNoResource[*omnires.MachineLabels](ctx, t, st, id)

	cancel()

	require.NoError(t, <-errCh, "runtime should exit without errors")
}
