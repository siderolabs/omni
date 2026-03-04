// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package metrics_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/metrics"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

func TestUserMetrics(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	ctrl := &metrics.UserMetricsController{}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterController(ctrl))

			st := testContext.State

			// User active 1 day ago (within both 7d and 30d windows).
			rmock.Mock[*authres.IdentityLastActive](ctx, t, st,
				options.WithID("recent-user@example.com"),
				options.Modify(func(res *authres.IdentityLastActive) error {
					res.TypedSpec().Value.LastActive = timestamppb.New(time.Now().Add(-24 * time.Hour))

					return nil
				}),
			)

			// User active 10 days ago (within 30d window only).
			rmock.Mock[*authres.IdentityLastActive](ctx, t, st,
				options.WithID("older-user@example.com"),
				options.Modify(func(res *authres.IdentityLastActive) error {
					res.TypedSpec().Value.LastActive = timestamppb.New(time.Now().Add(-10 * 24 * time.Hour))

					return nil
				}),
			)

			// Service account active 2 days ago (within both windows).
			rmock.Mock[*authres.IdentityLastActive](ctx, t, st,
				options.WithID("mysa@serviceaccount.omni.sidero.dev"),
				options.Modify(func(res *authres.IdentityLastActive) error {
					res.TypedSpec().Value.LastActive = timestamppb.New(time.Now().Add(-2 * 24 * time.Hour))

					return nil
				}),
			)

			// User active 60 days ago (outside both windows).
			rmock.Mock[*authres.IdentityLastActive](ctx, t, st,
				options.WithID("inactive@example.com"),
				options.Modify(func(res *authres.IdentityLastActive) error {
					res.TypedSpec().Value.LastActive = timestamppb.New(time.Now().Add(-60 * 24 * time.Hour))

					return nil
				}),
			)

			// Identity resources for total count.
			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("recent-user@example.com"),
			)

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("older-user@example.com"),
			)

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("inactive@example.com"),
			)

			rmock.Mock[*authres.Identity](ctx, t, st,
				options.WithID("mysa@serviceaccount.omni.sidero.dev"),
				options.EmptyLabel(authres.LabelIdentityTypeServiceAccount),
			)
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			registry := prometheus.NewRegistry()
			registry.MustRegister(ctrl)

			expectedActive := `
# HELP omni_active_users Number of active users and service accounts by time window (7d, 30d).
# TYPE omni_active_users gauge
omni_active_users{type="service_account",window="30d"} 1
omni_active_users{type="service_account",window="7d"} 1
omni_active_users{type="user",window="30d"} 2
omni_active_users{type="user",window="7d"} 1
`

			expectedTotal := `
# HELP omni_users Total number of registered users and service accounts.
# TYPE omni_users gauge
omni_users{type="service_account"} 1
omni_users{type="user"} 3
`

			assert.Eventually(t, func() bool {
				return testutil.GatherAndCompare(registry, strings.NewReader(expectedActive), "omni_active_users") == nil &&
					testutil.GatherAndCompare(registry, strings.NewReader(expectedTotal), "omni_users") == nil
			}, 5*time.Second, 100*time.Millisecond)
		},
	)
}
