// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
)

func TestReconciler(t *testing.T) {
	t.Parallel()

	reconciler := workloadproxy.NewReconciler(zaptest.NewLogger(t), zapcore.InfoLevel, 30*time.Second)

	err := reconciler.Reconcile("cluster1", map[string][]string{
		"alias1": {
			"upstream1",
			"upstream2",
		},
		"alias2": {
			"upstream3",
			"upstream4",
		},
	})
	require.NoError(t, err)

	proxy, id, err := reconciler.GetProxy("alias1")
	require.NoError(t, err)

	require.NotNil(t, proxy)
	require.Equal(t, "cluster1", id)

	proxy, id, err = reconciler.GetProxy("alias2")
	require.NoError(t, err)

	require.NotNil(t, proxy)
	require.Equal(t, "cluster1", id)

	err = reconciler.Reconcile("cluster2", nil)
	require.NoError(t, err)

	proxy, id, err = reconciler.GetProxy("alias1")
	require.NoError(t, err)

	require.NotNil(t, proxy)
	require.Equal(t, "cluster1", id)

	proxy, id, err = reconciler.GetProxy("alias3")
	require.NoError(t, err)

	require.Nil(t, proxy)
	require.Zero(t, id)
}
