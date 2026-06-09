// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func newDiscoveryAffiliateDeleteTask() *omni.DiscoveryAffiliateDeleteTask {
	task := omni.NewDiscoveryAffiliateDeleteTask("affiliate1")
	task.TypedSpec().Value.ClusterId = "cluster1"
	task.TypedSpec().Value.DiscoveryServiceEndpoint = "endpoint1"

	return task
}

// TestDiscoveryAffiliateDeleteTaskReconcile verifies that the affiliate is deleted from the discovery service and the
// task is cleaned up.
func TestDiscoveryAffiliateDeleteTaskReconcile(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		mock := &discoveryClientCacheMock{}

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewDiscoveryAffiliateDeleteTaskController(mock)))
			},
			func(ctx context.Context, tc testutils.TestContext) {
				task := newDiscoveryAffiliateDeleteTask()
				require.NoError(t, tc.State.Create(ctx, task, state.WithCreateOwner(omnictrl.ClusterMachineTeardownControllerName)))

				// the affiliate is not yet expired, so it is deleted from the discovery service and the task is cleaned up
				rtestutils.AssertNoResource[*omni.DiscoveryAffiliateDeleteTask](ctx, t, tc.State, task.Metadata().ID())

				synctest.Wait()

				assert.Equal(t, []affiliateDelete{{"endpoint1", "cluster1", "affiliate1"}}, mock.calls())
			},
		)
	})
}

// TestDiscoveryAffiliateDeleteTaskExpiration verifies that while the affiliate cannot be deleted the task is retained,
// and once enough time passes for the discovery service to have pruned the affiliate itself, the task is cleaned up
// without attempting a deletion.
func TestDiscoveryAffiliateDeleteTaskExpiration(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		mock := &discoveryClientCacheMock{failDeletion: true}

		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{},
			func(_ context.Context, tc testutils.TestContext) {
				require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewDiscoveryAffiliateDeleteTaskController(mock)))
			},
			func(ctx context.Context, tc testutils.TestContext) {
				task := newDiscoveryAffiliateDeleteTask()
				require.NoError(t, tc.State.Create(ctx, task, state.WithCreateOwner(omnictrl.ClusterMachineTeardownControllerName)))

				synctest.Wait()

				// a deletion was attempted with the expected values, but it failed, so the task is retained
				calls := mock.calls()
				require.NotEmpty(t, calls)
				assert.Equal(t, affiliateDelete{"endpoint1", "cluster1", "affiliate1"}, calls[0])

				rtestutils.AssertResource[*omni.DiscoveryAffiliateDeleteTask](
					ctx, t, tc.State, task.Metadata().ID(),
					func(r *omni.DiscoveryAffiliateDeleteTask, assertion *assert.Assertions) {
						assertion.Equal(resource.PhaseRunning, r.Metadata().Phase())
					},
				)

				// advance past the TTL so that the affiliate is assumed to be pruned by the discovery service itself.
				// while the deletion keeps failing the task is never torn down, so the task being cleaned up below proves
				// the expired branch skipped the deletion and let the task go.
				time.Sleep(31 * time.Minute)

				rtestutils.AssertNoResource[*omni.DiscoveryAffiliateDeleteTask](ctx, t, tc.State, task.Metadata().ID())
			},
		)
	})
}

type affiliateDelete struct {
	endpoint  string
	cluster   string
	affiliate string
}

type discoveryClientCacheMock struct {
	recorded     []affiliateDelete
	mu           sync.Mutex
	failDeletion bool
}

func (d *discoveryClientCacheMock) AffiliateDelete(_ context.Context, endpoint, cluster, affiliate string) error {
	d.mu.Lock()
	d.recorded = append(d.recorded, affiliateDelete{endpoint, cluster, affiliate})
	d.mu.Unlock()

	if d.failDeletion {
		return fmt.Errorf("deletion blocked - discovery service is not accessible")
	}

	return nil
}

func (d *discoveryClientCacheMock) calls() []affiliateDelete {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]affiliateDelete(nil), d.recorded...)
}
