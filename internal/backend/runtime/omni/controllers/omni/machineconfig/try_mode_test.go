// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/resources/config"
	talosruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configtry"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

// fastTryTimings keeps the same shape as the production set, scaled down so a whole give-up cycle
// fits inside a test.
var fastTryTimings = configtry.Timings{
	Timeout:         4 * time.Second,
	ConfirmAfter:    300 * time.Millisecond,
	ConfirmDeadline: 3 * time.Second,
	RollbackGrace:   500 * time.Millisecond,
	ConfirmRetry:    200 * time.Millisecond,
	MaxAttempts:     2,
}

//nolint:gocognit,gocyclo,cyclop,maintidx
func TestTryMode(t *testing.T) {
	t.Parallel()

	// tryModeMarker appears only in the config change each test pushes, so assertions can tell the
	// applies of interest from the initial cluster config.
	const tryModeMarker = "try-mode-test-interface"

	tryModeConfig := func() []byte {
		return fmt.Appendf(nil, `machine:
  network:
    interfaces:
      - interface: %s
        dhcp: true`, tryModeMarker)
	}

	isTryModeConfig := func(req *machine.ApplyConfigurationRequest) bool {
		return strings.Contains(string(req.GetData()), tryModeMarker)
	}

	// setupTryModeCluster brings up a single-machine cluster with its initial config applied.
	setupTryModeCluster := func(
		ctx context.Context, t *testing.T, tc testutils.TestContext, clusterName string, opts ...createClusterOption,
	) (*testutils.MachineServiceMock, string) {
		t.Helper()

		machineServices := testutils.NewMachineServices(t, tc.State)

		_, machines := createCluster(ctx, t, tc.State, machineServices, clusterName, 1, 0, opts...)
		require.Len(t, machines, 1)

		id := machines[0].Metadata().ID()

		rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
			a.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256, "the machine is not configured yet")
		})

		return machineServices.Get(id), id
	}

	registerTryModeController := func(t *testing.T) func(context.Context, testutils.TestContext) {
		return func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(
				machineconfig.NewStatusController(
					testutils.NewLifecycleManager(t, tc.State, nil),
					machineconfig.WithTryTimings(fastTryTimings),
				),
			))
		}
	}

	// recordedSha waits for the machine to have a recorded config and returns its sha.
	recordedSha := func(ctx context.Context, t *testing.T, st state.State, id string) string {
		t.Helper()

		var sha string

		rtestutils.AssertResource(ctx, t, st, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
			sha = res.TypedSpec().Value.ClusterMachineConfigSha256
			a.NotEmpty(sha)
		})

		return sha
	}

	okApply := func() *machine.ApplyConfigurationResponse {
		return &machine.ApplyConfigurationResponse{
			Messages: []*machine.ApplyConfiguration{{Mode: machine.ApplyConfigurationRequest_NO_REBOOT}},
		}
	}

	// countModes counts the try and the confirming applies of the test config the machine got.
	countModes := func(ms *testutils.MachineServiceMock) (tries, confirms int) {
		for _, req := range ms.GetApplyRequests() {
			if !isTryModeConfig(req) {
				continue
			}

			switch req.GetMode() { //nolint:exhaustive
			case machine.ApplyConfigurationRequest_TRY:
				tries++
			case machine.ApplyConfigurationRequest_NO_REBOOT:
				confirms++
			}
		}

		return tries, confirms
	}

	pushConfig := func(ctx context.Context, t *testing.T, st state.State, id string, data []byte) {
		t.Helper()

		rmock.Mock[*omni.ClusterMachineConfig](ctx, t, st, options.WithID(id), options.Modify(func(res *omni.ClusterMachineConfig) error {
			return res.TypedSpec().Value.SetUncompressedData(data)
		}))
	}

	// confirmsOfFirstConfigFail keeps the try of tryModeConfig pending on the machine by failing its confirm.
	confirmsOfFirstConfigFail := func(_ context.Context, req *machine.ApplyConfigurationRequest, _ state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
		if req.GetMode() == machine.ApplyConfigurationRequest_NO_REBOOT && isTryModeConfig(req) && !strings.Contains(string(req.GetData()), "mtu") {
			return nil, errors.New("machine is unreachable")
		}

		return okApply(), nil
	}

	waitForTry := func(t *testing.T, ms *testutils.MachineServiceMock) {
		t.Helper()

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			tries, _ := countModes(ms)
			assert.Positive(c, tries)
		}, 10*time.Second, 20*time.Millisecond)
	}

	// The happy path: the config lands in try mode, stays on the machine, and is then confirmed with a regular apply.
	t.Run("confirmsHealthyConfig", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-happy")

			// the machine stays on the same boot for the whole test
			require.NoError(t, ms.SetBootID(ctx, "boot-stable"))

			// The very first config a machine gets is applied while it is still in maintenance, so it can
			// never go through try mode.
			initial := ms.GetApplyRequests()
			require.NotEmpty(t, initial)
			require.Equal(t, machine.ApplyConfigurationRequest_AUTO, initial[0].GetMode())

			shaBefore := recordedSha(ctx, t, tc.State, id)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			// The config reaches the machine in try mode, with a rollback armed.
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				for _, req := range ms.GetApplyRequests() {
					if isTryModeConfig(req) && req.GetMode() == machine.ApplyConfigurationRequest_TRY {
						assert.Equal(c, fastTryTimings.Timeout, req.GetTryModeTimeout().AsDuration())

						return
					}
				}

				assert.Fail(c, "the config was not applied in try mode")
			}, 10*time.Second, 20*time.Millisecond)

			// Having stayed on the machine long enough, it is confirmed with a regular apply carrying the same bytes.
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				var try, commit *machine.ApplyConfigurationRequest

				for _, req := range ms.GetApplyRequests() {
					if !isTryModeConfig(req) {
						continue
					}

					switch req.GetMode() { //nolint:exhaustive
					case machine.ApplyConfigurationRequest_TRY:
						try = req
					case machine.ApplyConfigurationRequest_NO_REBOOT:
						commit = req
					}
				}

				if !assert.NotNil(c, try) || !assert.NotNil(c, commit) {
					return
				}

				assert.Equal(c, string(try.GetData()), string(commit.GetData()), "the confirming apply must carry the config that was tried")
				assert.Nil(c, commit.GetTryModeTimeout())
			}, 10*time.Second, 20*time.Millisecond)

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256, "the config should be recorded once confirmed")
				a.Nil(res.TypedSpec().Value.ConfigTry, "the try state should be cleared once confirmed")
				a.Empty(res.TypedSpec().Value.LastConfigError)
			})
		})
	})

	// A config that costs the machine its boot: the boot ID no longer matches when it is time to confirm, so the
	// config is never persisted.
	t.Run("givesUpWhenMachineReboots", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-reboot")

			var boots atomic.Int64

			require.NoError(t, ms.SetBootID(ctx, "boot-0"))

			// applying the new config reboots the machine, which drops the try config with it
			ms.OnApplyConfig = func(ctx context.Context, req *machine.ApplyConfigurationRequest, _ state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
				if req.GetMode() == machine.ApplyConfigurationRequest_TRY && isTryModeConfig(req) {
					if err := ms.SetBootID(ctx, fmt.Sprintf("boot-%d", boots.Add(1))); err != nil {
						return nil, err
					}
				}

				return okApply(), nil
			}

			shaBefore := recordedSha(ctx, t, tc.State, id)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			// Every attempt is spent, then the machine is left alone with an error to show for it.
			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.Contains(res.TypedSpec().Value.LastConfigError, "rolled back")
				a.Equal(fastTryTimings.MaxAttempts, res.TypedSpec().Value.ConfigTry.GetAttempts())
				a.Equal(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256, "a config that was never confirmed must not be recorded")
			})

			// The config was never persisted on the machine.
			for _, req := range ms.GetApplyRequests() {
				if isTryModeConfig(req) {
					assert.Equal(t, machine.ApplyConfigurationRequest_TRY, req.GetMode(), "the unconfirmed config must only ever be applied in try mode")
				}
			}
		})
	})

	// Talos before 1.14 cannot apply every change without a reboot, so its config changes stay plain applies.
	t.Run("appliesPlainlyBeforeTalos114", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-113", withClusterMockOption(options.WithTalosVersion("1.13.0")))

			ms.OnApplyConfig = func(context.Context, *machine.ApplyConfigurationRequest, state.State, string) (*machine.ApplyConfigurationResponse, error) {
				return okApply(), nil
			}

			shaBefore := recordedSha(ctx, t, tc.State, id)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
				a.Nil(res.TypedSpec().Value.ConfigTry)
			})

			var applies int

			for _, req := range ms.GetApplyRequests() {
				if isTryModeConfig(req) {
					applies++

					assert.Equal(t, machine.ApplyConfigurationRequest_AUTO, req.GetMode())
				}
			}

			assert.Equal(t, 1, applies)
		})
	})

	// A machine that has just rebooted into maintenance while Omni still sees it running: a try would take it out of
	// maintenance with no way back, so nothing is tried until the machine itself reports it is running.
	t.Run("doesNotTryInMaintenance", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-maintenance")

			shaBefore := recordedSha(ctx, t, tc.State, id)

			setStage := func(stage talosruntime.MachineStage) {
				_, err := safe.StateModifyWithResult(ctx, ms.State, talosruntime.NewMachineStatus(), func(res *talosruntime.MachineStatus) error {
					res.TypedSpec().Stage = stage

					return nil
				})
				require.NoError(t, err)
			}

			var versionCalls atomic.Int64

			ms.SetVersionHandler(func(context.Context, *emptypb.Empty) (*machine.VersionResponse, error) {
				versionCalls.Add(1)

				return &machine.VersionResponse{Messages: []*machine.Version{{Version: &machine.VersionInfo{Tag: "v" + constants.DefaultTalosVersion}}}}, nil
			})

			setStage(talosruntime.MachineStageMaintenance)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			// every apply attempt calls Version first, so a second call means the first attempt is over
			require.Eventually(t, func() bool { return versionCalls.Load() >= 2 }, 20*time.Second, 20*time.Millisecond)

			tries, _ := countModes(ms)
			assert.Zero(t, tries, "a machine in maintenance must not get a try")

			setStage(talosruntime.MachineStageRunning)

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256, "the config should be tried and confirmed once the machine runs")
				a.Nil(res.TypedSpec().Value.ConfigTry)
			})
		})
	})

	// Losing the machine partway through the window: the confirming apply never gets through, so Talos rolls the
	// config back on its own.
	t.Run("givesUpWhenConfirmationFails", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-unreachable")

			require.NoError(t, ms.SetBootID(ctx, "boot-stable"))

			ms.OnApplyConfig = func(_ context.Context, req *machine.ApplyConfigurationRequest, _ state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
				if req.GetMode() == machine.ApplyConfigurationRequest_NO_REBOOT && isTryModeConfig(req) {
					return nil, errors.New("machine is unreachable")
				}

				return okApply(), nil
			}

			shaBefore := recordedSha(ctx, t, tc.State, id)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.Contains(res.TypedSpec().Value.LastConfigError, "rolled back")
				a.Equal(fastTryTimings.MaxAttempts, res.TypedSpec().Value.ConfigTry.GetAttempts())
				a.Equal(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
			})

			stoppedSha := ""

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				stoppedSha = res.TypedSpec().Value.ConfigTry.GetSha256()
				a.NotEmpty(stoppedSha)
			})

			// a new config gets tried without the error about the previous one
			pushConfig(ctx, t, tc.State, id, append(tryModeConfig(), []byte("\n        mtu: 1400")...))

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(stoppedSha, res.TypedSpec().Value.ConfigTry.GetSha256())
				a.Empty(res.TypedSpec().Value.LastConfigError)
			})
		})
	})

	// A machine that cannot serve its boot ID for a while: the config is held back until the boot ID can be read,
	// and then goes in through try mode, never as a plain apply.
	t.Run("waitsForBootID", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-boot-id-late")

			require.NoError(t, ms.State.Destroy(ctx, talosruntime.NewBootID().Metadata()))

			shaBefore := recordedSha(ctx, t, tc.State, id)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			require.NoError(t, ms.SetBootID(ctx, "boot-late"))

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
				a.Nil(res.TypedSpec().Value.ConfigTry)
			})

			var tries int

			for _, req := range ms.GetApplyRequests() {
				if !isTryModeConfig(req) {
					continue
				}

				assert.NotEqual(t, machine.ApplyConfigurationRequest_AUTO, req.GetMode(), "the config must never go in as a plain apply")

				if req.GetMode() == machine.ApplyConfigurationRequest_TRY {
					tries++
				}
			}

			assert.Equal(t, 1, tries)
		})
	})

	// A config that cuts the machine off before the try apply can answer: the try is recorded anyway, so the attempt
	// is counted and confirmed once the machine answers again.
	t.Run("recordsTryWithoutAnswer", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-no-answer")

			require.NoError(t, ms.SetBootID(ctx, "boot-stable"))

			ms.OnApplyConfig = func(_ context.Context, req *machine.ApplyConfigurationRequest, _ state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
				if req.GetMode() == machine.ApplyConfigurationRequest_TRY && isTryModeConfig(req) {
					return nil, grpcstatus.Error(codes.DeadlineExceeded, "the answer never came back")
				}

				return okApply(), nil
			}

			shaBefore := recordedSha(ctx, t, tc.State, id)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256, "the config should be confirmed once the machine answers again")
				a.Nil(res.TypedSpec().Value.ConfigTry)
				a.Empty(res.TypedSpec().Value.LastConfigError)
			})

			tries, _ := countModes(ms)
			assert.Equal(t, 1, tries, "the unanswered try must be recorded, not sent again")
		})
	})

	// The confirming apply cannot reach the machine after its rollback.
	t.Run("confirmEndsBeforeRollback", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-confirm-deadline")

			var (
				mu            sync.Mutex
				tryLanded     time.Time
				confirmByTime time.Time
			)

			ms.OnApplyConfig = func(ctx context.Context, req *machine.ApplyConfigurationRequest, _ state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
				if isTryModeConfig(req) {
					mu.Lock()

					switch req.GetMode() { //nolint:exhaustive
					case machine.ApplyConfigurationRequest_TRY:
						tryLanded = time.Now()
					case machine.ApplyConfigurationRequest_NO_REBOOT:
						confirmByTime, _ = ctx.Deadline()
					}

					mu.Unlock()
				}

				return okApply(), nil
			}

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				mu.Lock()
				defer mu.Unlock()

				if !assert.False(c, confirmByTime.IsZero(), "the config was not confirmed yet") {
					return
				}

				// gRPC rebuilds the deadline on the machine from the remaining time, which adds the transit time
				assert.False(c, confirmByTime.After(tryLanded.Add(fastTryTimings.Timeout+100*time.Millisecond)), "the confirm may still arrive after the rollback")
			}, 10*time.Second, 20*time.Millisecond)
		})
	})

	// Locking the machine during a try stops all calls to it, so the try is left to roll back. It does not count as an
	// attempt, and the unlock tries the config again.
	t.Run("doesNotConfirmWhileLocked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-locked")

			shaBefore := recordedSha(ctx, t, tc.State, id)

			setLocked := func(ctx context.Context, st state.State, locked bool) error {
				_, err := safe.StateUpdateWithConflicts(ctx, st, omni.NewMachineSetNode(id, omni.NewMachineSet("")).Metadata(), func(res *omni.MachineSetNode) error {
					if locked {
						res.Metadata().Annotations().Set(omni.MachineLocked, "")
					} else {
						res.Metadata().Annotations().Delete(omni.MachineLocked)
					}

					return nil
				})

				return err
			}

			var lockedOnce atomic.Bool

			ms.OnApplyConfig = func(ctx context.Context, req *machine.ApplyConfigurationRequest, st state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
				if req.GetMode() == machine.ApplyConfigurationRequest_TRY && isTryModeConfig(req) && lockedOnce.CompareAndSwap(false, true) {
					if err := setLocked(ctx, st, true); err != nil {
						return nil, err
					}
				}

				return okApply(), nil
			}

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			pushed, err := safe.StateGetByID[*omni.ClusterMachineConfig](ctx, tc.State, id)
			require.NoError(t, err)

			// the try write sets the version, so a cleared try with this version can only come from the locked reconcile
			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.Equal(pushed.Metadata().Version().String(), res.TypedSpec().Value.ClusterMachineConfigVersion)
				a.Nil(res.TypedSpec().Value.ConfigTry)
				a.Equal(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
			})

			tries, confirms := countModes(ms)
			assert.Equal(t, 1, tries)
			assert.Zero(t, confirms, "a locked machine must not get the confirm")

			require.NoError(t, setLocked(ctx, tc.State, false))

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256, "the unlock should try and confirm the config again")
				a.Nil(res.TypedSpec().Value.ConfigTry)
			})

			tries, confirms = countModes(ms)
			assert.Equal(t, 2, tries)
			assert.Equal(t, 1, confirms)
		})
	})

	// A config generation error during a try stops all calls to the machine, so the try is left to roll back. It does not
	// count as an attempt, and fixing the error tries the config again.
	t.Run("doesNotConfirmDuringGenerationError", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		const generationError = "failed to generate the config"

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-generation-error")

			shaBefore := recordedSha(ctx, t, tc.State, id)

			setGenerationError := func(ctx context.Context, st state.State, generationError string) error {
				current, err := safe.StateGetByID[*omni.ClusterMachineConfig](ctx, st, id)
				if err != nil {
					return err
				}

				_, err = safe.StateUpdateWithConflicts(ctx, st, current.Metadata(), func(res *omni.ClusterMachineConfig) error {
					res.TypedSpec().Value.GenerationError = generationError

					return nil
				}, state.WithUpdateOwner(current.Metadata().Owner()))

				return err
			}

			var failedOnce atomic.Bool

			ms.OnApplyConfig = func(ctx context.Context, req *machine.ApplyConfigurationRequest, st state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
				if req.GetMode() == machine.ApplyConfigurationRequest_TRY && isTryModeConfig(req) && failedOnce.CompareAndSwap(false, true) {
					if err := setGenerationError(ctx, st, generationError); err != nil {
						return nil, err
					}
				}

				return okApply(), nil
			}

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.Equal(generationError, res.TypedSpec().Value.LastConfigError)
				a.Nil(res.TypedSpec().Value.ConfigTry)
				a.Equal(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
			})

			tries, confirms := countModes(ms)
			assert.Equal(t, 1, tries)
			assert.Zero(t, confirms, "the try must not be confirmed while the config cannot be generated")

			require.NoError(t, setGenerationError(ctx, tc.State, ""))

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				tries, _ := countModes(ms)

				a.Equal(2, tries)
				a.EqualValues(1, res.TypedSpec().Value.ConfigTry.GetAttempts(), "the rolled back try must not count as an attempt")
			})

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256, "fixing the error should try and confirm the config again")
				a.Nil(res.TypedSpec().Value.ConfigTry)
				a.Empty(res.TypedSpec().Value.LastConfigError)
			})
		})
	})

	// A try apply that never reached the machine: Omni gets no answer, and the confirm must not apply the config as a
	// regular apply, as nothing on the machine could roll it back.
	t.Run("doesNotConfirmAnUnlandedTry", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-unlanded")

			var dropped atomic.Bool

			ms.OnApplyConfig = func(ctx context.Context, req *machine.ApplyConfigurationRequest, _ state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
				if req.GetMode() == machine.ApplyConfigurationRequest_TRY && isTryModeConfig(req) && dropped.CompareAndSwap(false, true) {
					persisted, err := safe.StateGetByID[*talosconfig.MachineConfig](ctx, ms.State, talosconfig.PersistentID)
					if err != nil {
						return nil, err
					}

					// the request never reached the machine, so its active config stays the persisted one
					if err = ms.SetConfig(ctx, persisted.Provider(), talosconfig.ActiveID); err != nil {
						return nil, err
					}

					return nil, grpcstatus.Error(codes.DeadlineExceeded, "the request never reached the machine")
				}

				return okApply(), nil
			}

			shaBefore := recordedSha(ctx, t, tc.State, id)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.NotEqual(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
				a.Nil(res.TypedSpec().Value.ConfigTry)
			})

			tries, confirms := countModes(ms)

			assert.Equal(t, 2, tries, "the unlanded try should be followed by a second one")
			assert.Equal(t, 1, confirms, "only the try that landed may be confirmed")
		})
	})

	// A config change while a try is pending: the new config is tried and confirmed without waiting for the pending
	// try to roll back.
	t.Run("replacesPendingTry", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-replace")

			require.NoError(t, ms.SetBootID(ctx, "boot-stable"))

			ms.OnApplyConfig = confirmsOfFirstConfigFail

			shaBefore := recordedSha(ctx, t, tc.State, id)

			pushConfig(ctx, t, tc.State, id, tryModeConfig())
			waitForTry(t, ms)

			pushConfig(ctx, t, tc.State, id, append(tryModeConfig(), []byte("\n        mtu: 1400")...))

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				res, err := safe.StateGetByID[*omni.ClusterMachineConfigStatus](ctx, tc.State, id)
				if !assert.NoError(c, err) {
					return
				}

				assert.NotEqual(c, shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
				assert.Nil(c, res.TypedSpec().Value.ConfigTry)
			}, fastTryTimings.Timeout, 20*time.Millisecond, "the new config must be confirmed before the pending try times out")

			tries, confirms := countModes(ms)
			assert.Equal(t, 2, tries)
			assert.Equal(t, 1, confirms)
		})
	})

	// Taking a change back while its try is pending: the confirmed config is tried and confirmed again, instead of
	// being treated as in sync while the other config is still active.
	t.Run("revertReplacesPendingTry", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, registerTryModeController(t), func(ctx context.Context, tc testutils.TestContext) {
			ms, id := setupTryModeCluster(ctx, t, tc, "try-mode-revert")

			require.NoError(t, ms.SetBootID(ctx, "boot-stable"))

			ms.OnApplyConfig = confirmsOfFirstConfigFail

			shaBefore := recordedSha(ctx, t, tc.State, id)

			machineConfig, err := safe.StateGetByID[*omni.ClusterMachineConfig](ctx, tc.State, id)
			require.NoError(t, err)

			buffer, err := machineConfig.TypedSpec().Value.GetUncompressedData()
			require.NoError(t, err)

			confirmedConfig := string(buffer.Data())

			buffer.Free()

			pushConfig(ctx, t, tc.State, id, tryModeConfig())
			waitForTry(t, ms)

			pushConfig(ctx, t, tc.State, id, []byte(confirmedConfig))

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				var tried, confirmed bool

				for _, req := range ms.GetApplyRequests() {
					if string(req.GetData()) != confirmedConfig {
						continue
					}

					switch req.GetMode() { //nolint:exhaustive
					case machine.ApplyConfigurationRequest_TRY:
						tried = true
					case machine.ApplyConfigurationRequest_NO_REBOOT:
						confirmed = tried
					}
				}

				assert.True(c, confirmed, "the confirmed config must be tried and confirmed again")
			}, fastTryTimings.Timeout, 20*time.Millisecond)

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.Equal(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
				a.Nil(res.TypedSpec().Value.ConfigTry)
			})
		})
	})
}
