// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package redactedmachineconfig_test

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/redactedmachineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

//nolint:lll
const machineConfig = `version: v1alpha1
machine:
  type: controlplane
  token: '******'
  ca:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJQakNCOGFBREFnRUNBaEEydVNNVDNETWhhc3VreUd1d3pZVXhNQVVHQXl0bGNEQVFNUTR3REFZRFZRUUsKRXdWMFlXeHZjekFlRncweU5UQTNNRGd4TWpFNE5EaGFGdzB6TlRBM01EWXhNakU0TkRoYU1CQXhEakFNQmdOVgpCQW9UQlhSaGJHOXpNQ293QlFZREsyVndBeUVBNU15S3FTY2RSUjJLRzBXS0dUTllrUjFmM0dBRkNtbVFvMTk5CmVsM0YwdUtqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjREFRWUkKS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVRC9JQ0M4Mnl4QkFTOThRZQpaQzhneFVScUpVTXdCUVlESzJWd0EwRUFhSHM2S3Z1L0JDKzZzM2ZWQ1Y1NHRlQWpIZW5WTVdlcXFyb0V0bHBGCitDZXZQMlM3eHhXVU8zOTYzTjRxMFF1QzQvU2ZwVmFySzhmb1dKK0FBZ3pDQ3c9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    key: '******'
  certSANs: []
cluster:
  id: 1vUXXJzS9ahM3TE70vm29k6weYtYgGDxxY-edDjvf_k=
  secret: '******'
  controlPlane:
    endpoint: https://doesntmatter:6443
  token: '******'
  secretboxEncryptionSecret: '******'
  ca:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJpVENDQVMrZ0F3SUJBZ0lRSDV6TUR3SjhDdlpicEMwV2RZN2ZuakFLQmdncWhrak9QUVFEQWpBVk1STXcKRVFZRFZRUUtFd3ByZFdKbGNtNWxkR1Z6TUI0WERUSTFNRGN3T0RFeU1UZzBPRm9YRFRNMU1EY3dOakV5TVRnMApPRm93RlRFVE1CRUdBMVVFQ2hNS2EzVmlaWEp1WlhSbGN6QlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VICkEwSUFCUFg2bE5CMXBNdFAzMzdRb3orZUVnaWgwMDIzTkEzRWczNVZmQldYdnJ6aG5SNkU0SXIyaHJkRDhzOFcKK1hMMWllUDdKUlFmWklORVBVVzZjeExNakR5allUQmZNQTRHQTFVZER3RUIvd1FFQXdJQ2hEQWRCZ05WSFNVRQpGakFVQmdnckJnRUZCUWNEQVFZSUt3WUJCUVVIQXdJd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFCkZnUVVrYmQvN1pFYWVrb0tIYVptdUVJMXVnN3d6QTR3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQU56ZTFiNjEKdS9UV0tVU09mZ3JjVC9URTZYLytETGdDbXNDQU01OEg5Q3JtQWlCZlJXYktjVVpzWm9hOEZ6R1liNkNDL1V6bwozb3YwVDlSb2c3ZlJwM2tnaFE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    key: '******'
  aggregatorCA:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJZRENDQVFXZ0F3SUJBZ0lRTGRBcWdlUHQyUjg5dzZacTR5YUpmREFLQmdncWhrak9QUVFEQWpBQU1CNFgKRFRJMU1EY3dPREV5TVRnME9Gb1hEVE0xTURjd05qRXlNVGcwT0Zvd0FEQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxRwpTTTQ5QXdFSEEwSUFCQktnK3BadnVGd1NBZUpEYXA0K29FeEdEY05tWFR6d3hPSmtjVXZLSHVXTnQxNnY3b3EvCmtSb2JXYTlnSVZHVTlVYTNXYXg5ekc1SnFKL1duZGpEblIrallUQmZNQTRHQTFVZER3RUIvd1FFQXdJQ2hEQWQKQmdOVkhTVUVGakFVQmdnckJnRUZCUWNEQVFZSUt3WUJCUVVIQXdJd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZApCZ05WSFE0RUZnUVVxVy9kdW4wWGhETVgwaXUrTk5RbW50UHdvTEV3Q2dZSUtvWkl6ajBFQXdJRFNRQXdSZ0loCkFQWDNWM1R1TEdwZmc4Y21JWnFSMUZ2OFBWWE44cDgvaFR3Vk94clNMNlpkQWlFQTlCd0VzVGZDRWlUYm1vSFIKTmxPT3FmcndYQUtkZUxKeTJOZUdDdjZjV3JzPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    key: '******'
  serviceAccount:
    key: '******'
  etcd:
    ca:
      crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJmVENDQVNPZ0F3SUJBZ0lRRVEyU3FYYkhUc1cyQWVLejlZMlA2akFLQmdncWhrak9QUVFEQWpBUE1RMHcKQ3dZRFZRUUtFd1JsZEdOa01CNFhEVEkxTURjd09ERXlNVGcwT0ZvWERUTTFNRGN3TmpFeU1UZzBPRm93RHpFTgpNQXNHQTFVRUNoTUVaWFJqWkRCWk1CTUdCeXFHU000OUFnRUdDQ3FHU000OUF3RUhBMElBQkV6YkwyWjI2QlFzCmU2MHB6c3l4Wm1kK01FeFRrOUFLSUtGdVRBbmN4TWI5RE9CUHFwOE02ZFVyUnB5UUw2TTdVR1RxWkJGSlZYeUcKRGkyTXBGRVNWR3FqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjRApBUVlJS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVM01nQ3c4RWFjbGY4CnFjZ1dJTHR5VWxMTGVYY3dDZ1lJS29aSXpqMEVBd0lEU0FBd1JRSWdVN3llYU90enIrTUZrU0dHR2NlbWNNUCsKd1dUSVFOSzk5M3FnZWJlZHVlQUNJUURDODhnSlIwU1kxOWhDNkhmNlhZeHdQMlNiL2pMUTRpc3IrdGxFTG5odwpXQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      key: '******'`

func TestReconcile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cleanupCh := make(chan struct{})

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
		controller := redactedmachineconfig.NewController(redactedmachineconfig.ControllerOptions{
			DiffMaxAge: 100 * time.Millisecond,
			CleanupCh:  cleanupCh,
		})

		require.NoError(t, testContext.Runtime.RegisterQController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		testReconcile(ctx, t, testContext.State, cleanupCh)
	},
	)
}

func testReconcile(ctx context.Context, t *testing.T, st state.State, cleanupCh chan struct{}) {
	t.Helper()

	diffEventCh := make(chan state.Event)
	require.NoError(t, st.WatchKind(ctx, omni.NewMachineConfigDiff("").Metadata(), diffEventCh))

	id := "test-reconcile"
	cmc := omni.NewClusterMachineConfig(id)

	require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
	require.NoError(t, st.Create(ctx, cmc))

	rtestutils.AssertResource(ctx, t, st, id,
		func(cmcr *omni.RedactedClusterMachineConfig, assert *assert.Assertions) {
			buffer, err := cmcr.TypedSpec().Value.GetUncompressedData()
			assert.NoError(err)

			defer buffer.Free()

			data := string(buffer.Data())

			//nolint:lll
			assert.Equal(`version: v1alpha1
machine:
    type: controlplane
    token: '******'
    ca:
        crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJQakNCOGFBREFnRUNBaEEydVNNVDNETWhhc3VreUd1d3pZVXhNQVVHQXl0bGNEQVFNUTR3REFZRFZRUUsKRXdWMFlXeHZjekFlRncweU5UQTNNRGd4TWpFNE5EaGFGdzB6TlRBM01EWXhNakU0TkRoYU1CQXhEakFNQmdOVgpCQW9UQlhSaGJHOXpNQ293QlFZREsyVndBeUVBNU15S3FTY2RSUjJLRzBXS0dUTllrUjFmM0dBRkNtbVFvMTk5CmVsM0YwdUtqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjREFRWUkKS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVRC9JQ0M4Mnl4QkFTOThRZQpaQzhneFVScUpVTXdCUVlESzJWd0EwRUFhSHM2S3Z1L0JDKzZzM2ZWQ1Y1NHRlQWpIZW5WTVdlcXFyb0V0bHBGCitDZXZQMlM3eHhXVU8zOTYzTjRxMFF1QzQvU2ZwVmFySzhmb1dKK0FBZ3pDQ3c9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
        key: '******'
    certSANs: []
cluster:
    id: 1vUXXJzS9ahM3TE70vm29k6weYtYgGDxxY-edDjvf_k=
    secret: '******'
    controlPlane:
        endpoint: https://doesntmatter:6443
    token: '******'
    secretboxEncryptionSecret: '******'
    ca:
        crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJpVENDQVMrZ0F3SUJBZ0lRSDV6TUR3SjhDdlpicEMwV2RZN2ZuakFLQmdncWhrak9QUVFEQWpBVk1STXcKRVFZRFZRUUtFd3ByZFdKbGNtNWxkR1Z6TUI0WERUSTFNRGN3T0RFeU1UZzBPRm9YRFRNMU1EY3dOakV5TVRnMApPRm93RlRFVE1CRUdBMVVFQ2hNS2EzVmlaWEp1WlhSbGN6QlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VICkEwSUFCUFg2bE5CMXBNdFAzMzdRb3orZUVnaWgwMDIzTkEzRWczNVZmQldYdnJ6aG5SNkU0SXIyaHJkRDhzOFcKK1hMMWllUDdKUlFmWklORVBVVzZjeExNakR5allUQmZNQTRHQTFVZER3RUIvd1FFQXdJQ2hEQWRCZ05WSFNVRQpGakFVQmdnckJnRUZCUWNEQVFZSUt3WUJCUVVIQXdJd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFCkZnUVVrYmQvN1pFYWVrb0tIYVptdUVJMXVnN3d6QTR3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQU56ZTFiNjEKdS9UV0tVU09mZ3JjVC9URTZYLytETGdDbXNDQU01OEg5Q3JtQWlCZlJXYktjVVpzWm9hOEZ6R1liNkNDL1V6bwozb3YwVDlSb2c3ZlJwM2tnaFE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
        key: '******'
    aggregatorCA:
        crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJZRENDQVFXZ0F3SUJBZ0lRTGRBcWdlUHQyUjg5dzZacTR5YUpmREFLQmdncWhrak9QUVFEQWpBQU1CNFgKRFRJMU1EY3dPREV5TVRnME9Gb1hEVE0xTURjd05qRXlNVGcwT0Zvd0FEQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxRwpTTTQ5QXdFSEEwSUFCQktnK3BadnVGd1NBZUpEYXA0K29FeEdEY05tWFR6d3hPSmtjVXZLSHVXTnQxNnY3b3EvCmtSb2JXYTlnSVZHVTlVYTNXYXg5ekc1SnFKL1duZGpEblIrallUQmZNQTRHQTFVZER3RUIvd1FFQXdJQ2hEQWQKQmdOVkhTVUVGakFVQmdnckJnRUZCUWNEQVFZSUt3WUJCUVVIQXdJd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZApCZ05WSFE0RUZnUVVxVy9kdW4wWGhETVgwaXUrTk5RbW50UHdvTEV3Q2dZSUtvWkl6ajBFQXdJRFNRQXdSZ0loCkFQWDNWM1R1TEdwZmc4Y21JWnFSMUZ2OFBWWE44cDgvaFR3Vk94clNMNlpkQWlFQTlCd0VzVGZDRWlUYm1vSFIKTmxPT3FmcndYQUtkZUxKeTJOZUdDdjZjV3JzPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
        key: '******'
    serviceAccount:
        key: '******'
    etcd:
        ca:
            crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJmVENDQVNPZ0F3SUJBZ0lRRVEyU3FYYkhUc1cyQWVLejlZMlA2akFLQmdncWhrak9QUVFEQWpBUE1RMHcKQ3dZRFZRUUtFd1JsZEdOa01CNFhEVEkxTURjd09ERXlNVGcwT0ZvWERUTTFNRGN3TmpFeU1UZzBPRm93RHpFTgpNQXNHQTFVRUNoTUVaWFJqWkRCWk1CTUdCeXFHU000OUFnRUdDQ3FHU000OUF3RUhBMElBQkV6YkwyWjI2QlFzCmU2MHB6c3l4Wm1kK01FeFRrOUFLSUtGdVRBbmN4TWI5RE9CUHFwOE02ZFVyUnB5UUw2TTdVR1RxWkJGSlZYeUcKRGkyTXBGRVNWR3FqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjRApBUVlJS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVM01nQ3c4RWFjbGY4CnFjZ1dJTHR5VWxMTGVYY3dDZ1lJS29aSXpqMEVBd0lEU0FBd1JRSWdVN3llYU90enIrTUZrU0dHR2NlbWNNUCsKd1dUSVFOSzk5M3FnZWJlZHVlQUNJUURDODhnSlIwU1kxOWhDNkhmNlhZeHdQMlNiL2pMUTRpc3IrdGxFTG5odwpXQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
            key: '******'
`, data)
		},
	)

	rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfig, assert *assert.Assertions) {
		assert.True(res.Metadata().Finalizers().Has(redactedmachineconfig.ControllerName), "expected controller name finalizer to be set")
	})

	// update the config, it should generate a diff
	diffID1 := updateConfigAssertDiff(ctx, t, st, cmc, "aaa", "bbb", diffEventCh)

	// update the config again, it should generate another diff
	diffID2 := updateConfigAssertDiff(ctx, t, st, cmc, "ccc", "ddd", diffEventCh)

	// update the config again, it should generate a third diff
	diffID3 := updateConfigAssertDiff(ctx, t, st, cmc, "eee", "fff", diffEventCh)

	sleep(ctx, t, 150*time.Millisecond)
	doCleanup(ctx, t, cleanupCh)

	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID1)
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID2)

	sleep(ctx, t, 75*time.Millisecond)
	doCleanup(ctx, t, cleanupCh)

	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID3)
}

func TestTeardown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
		controller := redactedmachineconfig.NewController(redactedmachineconfig.ControllerOptions{})
		require.NoError(t, testContext.Runtime.RegisterQController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		testTeardown(ctx, t, testContext.State)
	},
	)
}

func testTeardown(ctx context.Context, t *testing.T, st state.State) {
	t.Helper()

	diffEventCh := make(chan state.Event)
	require.NoError(t, st.WatchKind(ctx, omni.NewMachineConfigDiff("").Metadata(), diffEventCh))

	id := "test-teardown"
	cmc := omni.NewClusterMachineConfig(id)

	require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
	require.NoError(t, st.Create(ctx, cmc))
	rtestutils.AssertResource(ctx, t, st, id, func(*omni.RedactedClusterMachineConfig, *assert.Assertions) {})

	rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfig, assert *assert.Assertions) {
		assert.True(res.Metadata().Finalizers().Has(redactedmachineconfig.ControllerName), "expected controller name finalizer to be set")
	})

	// update the config, it should generate a diff
	diffID1 := updateConfigAssertDiff(ctx, t, st, cmc, "aaa", "bbb", diffEventCh)
	diffID2 := updateConfigAssertDiff(ctx, t, st, cmc, "ccc", "ddd", diffEventCh)

	// delete the config, assert that the redacted config is deleted
	rtestutils.Destroy[*omni.ClusterMachineConfig](ctx, t, st, []string{id})

	rtestutils.AssertNoResource[*omni.RedactedClusterMachineConfig](ctx, t, st, id)

	// assert that diffs are cleaned up
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID1)
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID2)
}

func TestMaxDiffCount(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
		controller := redactedmachineconfig.NewController(redactedmachineconfig.ControllerOptions{
			DiffCleanupInterval: 50 * time.Millisecond,
			DiffMaxAge:          24 * time.Hour,
			DiffMaxCount:        2,
		})
		require.NoError(t, testContext.Runtime.RegisterQController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		testMaxDiffCount(ctx, t, testContext.State)
	},
	)
}

func testMaxDiffCount(ctx context.Context, t *testing.T, st state.State) {
	t.Helper()

	diffEventCh := make(chan state.Event)
	require.NoError(t, st.WatchKind(ctx, omni.NewMachineConfigDiff("").Metadata(), diffEventCh))

	id := "test-max-diff"
	cmc := omni.NewClusterMachineConfig(id)

	require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
	require.NoError(t, st.Create(ctx, cmc))
	rtestutils.AssertResource(ctx, t, st, id, func(*omni.RedactedClusterMachineConfig, *assert.Assertions) {})

	rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfig, assert *assert.Assertions) {
		assert.True(res.Metadata().Finalizers().Has(redactedmachineconfig.ControllerName), "expected controller name finalizer to be set")
	})

	// update the config, it should generate a diff
	diffID1 := updateConfigAssertDiff(ctx, t, st, cmc, "aaa", "bbb", diffEventCh)
	diffID2 := updateConfigAssertDiff(ctx, t, st, cmc, "ccc", "ddd", diffEventCh)
	diffID3 := updateConfigAssertDiff(ctx, t, st, cmc, "eee", "fff", diffEventCh)
	diffID4 := updateConfigAssertDiff(ctx, t, st, cmc, "ggg", "hhh", diffEventCh)
	diffID5 := updateConfigAssertDiff(ctx, t, st, cmc, "iii", "jjj", diffEventCh)

	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID1)
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID2)
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID3)

	rtestutils.AssertResources(ctx, t, st, []resource.ID{diffID4, diffID5}, func(res *omni.MachineConfigDiff, assert *assert.Assertions) {})
}

func updateConfigAssertDiff(ctx context.Context, t *testing.T, st state.State, cmc *omni.ClusterMachineConfig, testKey, testVal string, diffEventCh chan state.Event) resource.ID {
	_, err := safe.StateUpdateWithConflicts(ctx, st, cmc.Metadata(), func(res *omni.ClusterMachineConfig) error {
		data, err := res.TypedSpec().Value.GetUncompressedData()
		if err != nil {
			return err
		}

		defer data.Free()

		updatedConfig := updateConfig(t, data.Data(), map[string]string{testKey: testVal})

		return res.TypedSpec().Value.SetUncompressedData(updatedConfig)
	})
	require.NoError(t, err)

	var event state.Event

	select {
	case <-ctx.Done():
		require.Fail(t, "timed out waiting for diff creation event")
	case event = <-diffEventCh:
	}

	res, ok := event.Resource.(*omni.MachineConfigDiff)
	require.True(t, ok, "expected resource to be MachineConfigDiff")

	expectedPrefix := cmc.Metadata().ID() + "-"
	assert.Truef(t, strings.HasPrefix(res.Metadata().ID(), expectedPrefix), "expected diff ID to have the prefix %q, got %q", expectedPrefix, res.Metadata().ID())
	assert.Equal(t, state.Created, event.Type)

	diff := res.TypedSpec().Value.Diff
	require.Contains(t, diff, fmt.Sprintf("+        %s: %s", testKey, testVal))

	return res.Metadata().ID()
}

func updateConfig(t *testing.T, existingConfig []byte, nodeLabelsToAdd map[string]string) []byte {
	config, err := configloader.NewFromBytes(existingConfig)
	require.NoError(t, err)

	config, err = config.PatchV1Alpha1(func(config *v1alpha1.Config) error {
		config.MachineConfig.MachineNodeLabels = nodeLabelsToAdd

		return nil
	})
	require.NoError(t, err)

	encoded, err := config.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)

	return encoded
}

func doCleanup(ctx context.Context, t *testing.T, cleanupCh chan struct{}) {
	t.Helper()

	select {
	case <-ctx.Done():
		require.Fail(t, "timed out waiting for cleanup signal")
	case cleanupCh <- struct{}{}:
	}
}

func sleep(ctx context.Context, t *testing.T, duration time.Duration) {
	t.Helper()

	select {
	case <-ctx.Done():
		require.Fail(t, "timed out waiting during sleep")
	case <-time.After(duration):
	}
}
