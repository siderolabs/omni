// Copyright (c) 2026 Sidero Labs, Inc.
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
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/redactedmachineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

//go:embed testdata/config.yaml
var machineConfig string

func TestReconcile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cleanupCh := make(chan struct{})

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
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
	cmc := omni.NewClusterMachineConfigStatus(id)

	require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
	require.NoError(t, st.Create(ctx, cmc))

	rtestutils.AssertResource(
		ctx, t, st, id,
		func(cmcr *omni.RedactedClusterMachineConfig, assert *assert.Assertions) {
			buffer, err := cmcr.TypedSpec().Value.GetUncompressedData()
			assert.NoError(err)

			defer buffer.Free()

			data := string(buffer.Data())

			assert.Equal(machineConfig, data)
		},
	)

	rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
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

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
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
	cmc := omni.NewClusterMachineConfigStatus(id)

	require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
	require.NoError(t, st.Create(ctx, cmc))
	rtestutils.AssertResource(ctx, t, st, id, func(*omni.RedactedClusterMachineConfig, *assert.Assertions) {})

	rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
		assert.True(res.Metadata().Finalizers().Has(redactedmachineconfig.ControllerName), "expected controller name finalizer to be set")
	})

	// update the config, it should generate a diff
	diffID1 := updateConfigAssertDiff(ctx, t, st, cmc, "aaa", "bbb", diffEventCh)
	diffID2 := updateConfigAssertDiff(ctx, t, st, cmc, "ccc", "ddd", diffEventCh)

	// delete the config, assert that the redacted config is deleted
	rtestutils.Destroy[*omni.ClusterMachineConfigStatus](ctx, t, st, []string{id})

	rtestutils.AssertNoResource[*omni.RedactedClusterMachineConfig](ctx, t, st, id)

	// assert that diffs are cleaned up
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID1)
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, t, st, diffID2)
}

func TestMaxDiffCount(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
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
	cmc := omni.NewClusterMachineConfigStatus(id)

	require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
	require.NoError(t, st.Create(ctx, cmc))
	rtestutils.AssertResource(ctx, t, st, id, func(*omni.RedactedClusterMachineConfig, *assert.Assertions) {})

	rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
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

func updateConfigAssertDiff(ctx context.Context, t *testing.T, st state.State, cmc *omni.ClusterMachineConfigStatus, testKey, testVal string, diffEventCh chan state.Event) resource.ID {
	_, err := safe.StateUpdateWithConflicts(ctx, st, cmc.Metadata(), func(res *omni.ClusterMachineConfigStatus) error {
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

	for {
		select {
		case <-ctx.Done():
			require.Fail(t, "timed out waiting for diff creation event")
		case event = <-diffEventCh:
		}

		require.NoError(t, event.Error, "error received in diff event")

		if event.Type == state.Created {
			break
		}
	}

	res, ok := event.Resource.(*omni.MachineConfigDiff)
	require.True(t, ok, "expected resource to be MachineConfigDiff")

	expectedPrefix := cmc.Metadata().ID() + "-"
	assert.Truef(t, strings.HasPrefix(res.Metadata().ID(), expectedPrefix), "expected diff ID to have the prefix %q, got %q", expectedPrefix, res.Metadata().ID())
	assert.Equalf(t, state.Created, event.Type, "expected event type to be %s, got %s", state.Created, event.Type.String())

	buffer, err := res.TypedSpec().Value.GetUncompressedData()
	require.NoError(t, err)

	defer buffer.Free()

	require.Contains(t, string(buffer.Data()), fmt.Sprintf("+        %s: %s", testKey, testVal))

	return res.Metadata().ID()
}

func updateConfig(t *testing.T, existingConfig []byte, nodeLabelsToAdd map[string]string) []byte {
	config, err := configloader.NewFromBytes(existingConfig)
	require.NoError(t, err)

	config, err = config.PatchV1Alpha1(func(config *v1alpha1.Config) error {
		config.MachineConfig.MachineNodeLabels = nodeLabelsToAdd //nolint:staticcheck

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

func TestInstallImageHistory(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(redactedmachineconfig.NewController(redactedmachineconfig.ControllerOptions{})))
		}, func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			id := "test-install-image-history"

			setImage := func(res *omni.ClusterMachineConfigStatus, version, schematic, host string) {
				res.TypedSpec().Value.TalosVersion = version
				res.TypedSpec().Value.SchematicId = schematic
				res.TypedSpec().Value.ImageFactoryHost = host
			}

			cmc := omni.NewClusterMachineConfigStatus(id)
			setImage(cmc, "1.14.0", "A", "factory-a.talos.dev")
			require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
			require.NoError(t, st.Create(ctx, cmc))

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.RedactedClusterMachineConfig, a *assert.Assertions) {
				a.Equal("1.14.0", res.TypedSpec().Value.GetInstallImage().GetTalosVersion())
			})

			_, err := safe.StateUpdateWithConflicts(ctx, st, cmc.Metadata(), func(res *omni.ClusterMachineConfigStatus) error {
				setImage(res, "1.14.1", "A", "factory-b.talos.dev")

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertLength[*omni.MachineConfigDiff](ctx, t, st, 1)
			rtestutils.AssertAll(ctx, t, st, func(res *omni.MachineConfigDiff, a *assert.Assertions) {
				a.False(hasDiff(t, res))
				a.True(proto.Equal(&specs.InstallImageInfo{TalosVersion: "1.14.0", SchematicId: "A", ImageFactoryHost: "factory-a.talos.dev"}, res.TypedSpec().Value.GetFromInstallImage()))
				a.True(proto.Equal(&specs.InstallImageInfo{TalosVersion: "1.14.1", SchematicId: "A", ImageFactoryHost: "factory-b.talos.dev"}, res.TypedSpec().Value.GetToInstallImage()))
			})

			_, err = safe.StateUpdateWithConflicts(ctx, st, cmc.Metadata(), func(res *omni.ClusterMachineConfigStatus) error {
				setImage(res, "1.14.1", "A", "factory-c.talos.dev")

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.RedactedClusterMachineConfig, a *assert.Assertions) {
				a.Equal("factory-c.talos.dev", res.TypedSpec().Value.GetInstallImage().GetImageFactoryHost())
			})
			rtestutils.AssertLength[*omni.MachineConfigDiff](ctx, t, st, 1)

			_, err = safe.StateUpdateWithConflicts(ctx, st, cmc.Metadata(), func(res *omni.ClusterMachineConfigStatus) error {
				data, dataErr := res.TypedSpec().Value.GetUncompressedData()
				if dataErr != nil {
					return dataErr
				}

				defer data.Free()

				return res.TypedSpec().Value.SetUncompressedData(updateConfig(t, data.Data(), map[string]string{"aaa": "bbb"}))
			})
			require.NoError(t, err)

			rtestutils.AssertLength[*omni.MachineConfigDiff](ctx, t, st, 2)

			diffs, err := safe.StateListAll[*omni.MachineConfigDiff](ctx, st)
			require.NoError(t, err)

			withDiff := 0

			for res := range diffs.All() {
				if !hasDiff(t, res) {
					continue
				}

				withDiff++

				assert.Nil(t, res.TypedSpec().Value.GetFromInstallImage())
				assert.Nil(t, res.TypedSpec().Value.GetToInstallImage())
			}

			assert.Equal(t, 1, withDiff)
		},
	)
}

func TestInstallImageFirstRecorded(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(redactedmachineconfig.NewController(redactedmachineconfig.ControllerOptions{})))
		}, func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State
			id := "test-install-image-first-recorded"

			cmc := omni.NewClusterMachineConfigStatus(id)
			require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
			require.NoError(t, st.Create(ctx, cmc))

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.RedactedClusterMachineConfig, a *assert.Assertions) {
				a.NotNil(res.TypedSpec().Value.GetInstallImage())
				a.Empty(res.TypedSpec().Value.GetInstallImage().GetTalosVersion())
			})

			_, err := safe.StateUpdateWithConflicts(ctx, st, cmc.Metadata(), func(res *omni.ClusterMachineConfigStatus) error {
				res.TypedSpec().Value.TalosVersion = "1.14.0"
				res.TypedSpec().Value.SchematicId = "A"

				data, dataErr := res.TypedSpec().Value.GetUncompressedData()
				if dataErr != nil {
					return dataErr
				}

				defer data.Free()

				return res.TypedSpec().Value.SetUncompressedData(updateConfig(t, data.Data(), map[string]string{"aaa": "bbb"}))
			})
			require.NoError(t, err)

			rtestutils.AssertLength[*omni.MachineConfigDiff](ctx, t, st, 1)
			rtestutils.AssertAll(ctx, t, st, func(res *omni.MachineConfigDiff, a *assert.Assertions) {
				a.True(hasDiff(t, res))
				a.Nil(res.TypedSpec().Value.GetFromInstallImage())
				a.Nil(res.TypedSpec().Value.GetToInstallImage())
			})
		},
	)
}

func TestInstallImageRecordedForExistingResource(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	id := "test-install-image-existing-resource"

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			cmc := omni.NewClusterMachineConfigStatus(id)
			cmc.Metadata().Finalizers().Add(redactedmachineconfig.ControllerName)
			cmc.TypedSpec().Value.TalosVersion = "1.14.0"
			cmc.TypedSpec().Value.SchematicId = "A"
			require.NoError(t, cmc.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
			require.NoError(t, st.Create(ctx, cmc))

			cmc, err := safe.StateGetByID[*omni.ClusterMachineConfigStatus](ctx, st, id)
			require.NoError(t, err)

			redacted := omni.NewRedactedClusterMachineConfig(id)
			helpers.UpdateInputsVersions(redacted, cmc)
			require.NoError(t, redacted.TypedSpec().Value.SetUncompressedData([]byte(machineConfig)))
			require.NoError(t, st.Create(ctx, redacted, state.WithCreateOwner(redactedmachineconfig.ControllerName)))

			require.NoError(t, testContext.Runtime.RegisterQController(redactedmachineconfig.NewController(redactedmachineconfig.ControllerOptions{})))
		}, func(ctx context.Context, testContext testutils.TestContext) {
			st := testContext.State

			rtestutils.AssertResource(ctx, t, st, id, func(res *omni.RedactedClusterMachineConfig, a *assert.Assertions) {
				a.Equal("1.14.0", res.TypedSpec().Value.GetInstallImage().GetTalosVersion())
			})

			_, err := safe.StateUpdateWithConflicts(ctx, st, omni.NewClusterMachineConfigStatus(id).Metadata(), func(res *omni.ClusterMachineConfigStatus) error {
				res.TypedSpec().Value.TalosVersion = "1.14.1"

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertLength[*omni.MachineConfigDiff](ctx, t, st, 1)
			rtestutils.AssertAll(ctx, t, st, func(res *omni.MachineConfigDiff, a *assert.Assertions) {
				a.Equal("1.14.0", res.TypedSpec().Value.GetFromInstallImage().GetTalosVersion())
				a.Equal("1.14.1", res.TypedSpec().Value.GetToInstallImage().GetTalosVersion())
			})
		},
	)
}

func hasDiff(t *testing.T, res *omni.MachineConfigDiff) bool {
	t.Helper()

	buffer, err := res.TypedSpec().Value.GetUncompressedData()
	require.NoError(t, err)

	defer buffer.Free()

	return len(buffer.Data()) > 0
}
