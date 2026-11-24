// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig_test

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configtry"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

type dryRunTestSetup struct {
	st              state.State
	machineServices *testutils.MachineServices
	cluster         *omni.Cluster
	id              string
}

func withDryRunTestCluster(t *testing.T, workers int, test func(ctx context.Context, s dryRunTestSetup)) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			lifecycleManager := testutils.NewLifecycleManager(t, tc.State, nil)

			require.NoError(t, tc.Runtime.RegisterQController(machineconfig.NewStatusController(lifecycleManager, machineconfig.WithTryTimings(fastTryTimings))))
			require.NoError(t, tc.Runtime.RegisterQController(machineconfig.NewDryRunStatusController(lifecycleManager)))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			machineServices := testutils.NewMachineServices(t, tc.State)
			cluster, machines := createCluster(ctx, t, tc.State, machineServices, "dry-run", 1, workers)
			id := machines[len(machines)-1].Metadata().ID()

			for _, m := range machines {
				rtestutils.AssertResource(ctx, t, tc.State, m.Metadata().ID(), func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
					a.NotEmpty(res.TypedSpec().Value.ClusterMachineConfigSha256)
				})
			}

			test(ctx, dryRunTestSetup{st: tc.State, machineServices: machineServices, cluster: cluster, id: id})
		},
	)
}

// withDryRunOnlyCluster runs only the dry-run controller, so the test controls the try state and the pending diff of the machine.
func withDryRunOnlyCluster(t *testing.T, test func(ctx context.Context, s dryRunTestSetup)) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(machineconfig.NewDryRunStatusController(testutils.NewLifecycleManager(t, tc.State, nil))))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			machineServices := testutils.NewMachineServices(t, tc.State)
			cluster, machines := createCluster(ctx, t, tc.State, machineServices, "dry-run", 1, 0)
			id := machines[0].Metadata().ID()

			rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, tc.State, options.WithID(id))
			rmock.Mock[*omni.MachinePendingUpdates](ctx, t, tc.State, options.WithID(id), options.Modify(func(res *omni.MachinePendingUpdates) error {
				return res.TypedSpec().Value.SetUncompressedData([]byte("+hostname: pending"))
			}))

			test(ctx, dryRunTestSetup{st: tc.State, machineServices: machineServices, cluster: cluster, id: id})
		},
	)
}

func (s dryRunTestSetup) dryRunRequests() []*machine.ApplyConfigurationRequest {
	return slices.DeleteFunc(s.machineServices.Get(s.id).GetApplyRequests(), func(req *machine.ApplyConfigurationRequest) bool {
		return !req.GetDryRun()
	})
}

func (s dryRunTestSetup) lockCluster(ctx context.Context, t *testing.T, locked bool) {
	rmock.Mock[*omni.Cluster](ctx, t, s.st, options.SameID(s.cluster), options.Modify(func(res *omni.Cluster) error {
		if locked {
			res.Metadata().Annotations().Set(omni.ClusterLocked, "")
		} else {
			res.Metadata().Annotations().Delete(omni.ClusterLocked)
		}

		return nil
	}))
}

func (s dryRunTestSetup) setHostname(ctx context.Context, t *testing.T, hostname string) {
	rmock.Mock[*omni.ClusterMachineConfig](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.ClusterMachineConfig) error {
		return res.TypedSpec().Value.SetUncompressedData([]byte("machine:\n  network:\n    hostname: " + hostname))
	}))
}

func (s dryRunTestSetup) onDryRun(handler func(*machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error)) {
	s.machineServices.Get(s.id).OnApplyConfig = func(_ context.Context, req *machine.ApplyConfigurationRequest, _ state.State, _ string) (*machine.ApplyConfigurationResponse, error) {
		if req.GetDryRun() {
			return handler(req)
		}

		return &machine.ApplyConfigurationResponse{Messages: []*machine.ApplyConfiguration{{Mode: machine.ApplyConfigurationRequest_NO_REBOOT}}}, nil
	}
}

func (s dryRunTestSetup) assertOutcome(ctx context.Context, t *testing.T, check func(*specs.DryRunStatusSpec, *assert.Assertions)) {
	rtestutils.AssertResource(ctx, t, s.st, s.id, func(res *omni.DryRunStatus, a *assert.Assertions) {
		check(res.TypedSpec().Value, a)
	})
}

//nolint:maintidx
func TestDryRunStatusController(t *testing.T) {
	t.Parallel()

	t.Run("passed", func(t *testing.T) {
		t.Parallel()

		withDryRunTestCluster(t, 0, func(ctx context.Context, s dryRunTestSetup) {
			s.lockCluster(ctx, t, true)
			s.setHostname(ctx, t, "dry-run-passed")

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
				a.False(spec.RebootRequired)
				a.NotEmpty(spec.ClusterMachineConfigSha256)
			})

			requests := s.dryRunRequests()
			require.NotEmpty(t, requests)
			assert.Equal(t, machine.ApplyConfigurationRequest_AUTO, requests[0].GetMode())
		})
	})

	t.Run("try in flight", func(t *testing.T) {
		t.Parallel()

		withDryRunOnlyCluster(t, func(ctx context.Context, s dryRunTestSetup) {
			// the rollback of this try ends about 5 seconds from now
			startedAt := time.Now().Add(-configtry.Default.Timeout)

			rmock.Mock[*omni.ClusterMachineConfigStatus](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.ClusterMachineConfigStatus) error {
				res.TypedSpec().Value.ConfigTry = &specs.ConfigTryStatus{StartedAt: timestamppb.New(startedAt)}

				return nil
			}))

			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NOT_CHECKED, spec.Outcome)
				a.Equal(specs.DryRunStatusSpec_TRY_IN_FLIGHT, spec.NotCheckedReason)
			})

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
			})
		})
	})

	t.Run("maintenance", func(t *testing.T) {
		t.Parallel()

		withDryRunOnlyCluster(t, func(ctx context.Context, s dryRunTestSetup) {
			rmock.Mock[*omni.MachineStatus](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Maintenance = true

				return nil
			}))

			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NOT_CHECKED, spec.Outcome)
				a.Equal(specs.DryRunStatusSpec_MAINTENANCE, spec.NotCheckedReason)
			})
		})
	})

	t.Run("maintenance stage", func(t *testing.T) {
		t.Parallel()

		withDryRunOnlyCluster(t, func(ctx context.Context, s dryRunTestSetup) {
			rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.MachineStatusSnapshot) error {
				res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{Stage: machine.MachineStatusEvent_MAINTENANCE}

				return nil
			}))

			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NOT_CHECKED, spec.Outcome)
				a.Equal(specs.DryRunStatusSpec_MAINTENANCE, spec.NotCheckedReason, "the snapshot reports maintenance before MachineStatus does")
			})
		})
	})

	t.Run("not running", func(t *testing.T) {
		t.Parallel()

		withDryRunOnlyCluster(t, func(ctx context.Context, s dryRunTestSetup) {
			rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.MachineStatusSnapshot) error {
				res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{Stage: machine.MachineStatusEvent_REBOOTING}

				return nil
			}))

			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NOT_CHECKED, spec.Outcome)
				a.Equal(specs.DryRunStatusSpec_NOT_RUNNING, spec.NotCheckedReason)
			})
		})
	})

	t.Run("checked when reconnected", func(t *testing.T) {
		t.Parallel()

		withDryRunOnlyCluster(t, func(ctx context.Context, s dryRunTestSetup) {
			setConnected := func(connected bool) {
				rmock.Mock[*omni.MachineStatus](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.MachineStatus) error {
					res.TypedSpec().Value.Connected = connected

					return nil
				}))
			}

			setConnected(false)
			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_DISCONNECTED, spec.NotCheckedReason)
			})

			setConnected(true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
			})
		})
	})

	t.Run("check error", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		withDryRunOnlyCluster(t, func(ctx context.Context, s dryRunTestSetup) {
			s.onDryRun(func(*machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
				if calls.Add(1) == 2 {
					return nil, grpcstatus.Error(codes.FailedPrecondition, "another apply is in progress")
				}

				return &machine.ApplyConfigurationResponse{Messages: []*machine.ApplyConfiguration{{Mode: machine.ApplyConfigurationRequest_NO_REBOOT}}}, nil
			})

			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
			})

			s.setHostname(ctx, t, "dry-run-check-error")

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_CHECK_ERROR, spec.NotCheckedReason)
				a.Contains(spec.Error, "another apply is in progress")
				a.Empty(spec.ClusterMachineConfigSha256)
			})

			rmock.Mock[*omni.MachineStatus](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.TalosVersion = "1.99.0"

				return nil
			}))

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
				a.Empty(spec.Error)
			})
		})
	})

	t.Run("checked again after a reboot", func(t *testing.T) {
		t.Parallel()

		withDryRunOnlyCluster(t, func(ctx context.Context, s dryRunTestSetup) {
			setStage := func(stage machine.MachineStatusEvent_MachineStage) {
				rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.MachineStatusSnapshot) error {
					res.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{Stage: stage}

					return nil
				}))
			}

			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
			})

			setStage(machine.MachineStatusEvent_REBOOTING)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NOT_RUNNING, spec.NotCheckedReason)
			})

			setStage(machine.MachineStatusEvent_RUNNING)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
			})

			assert.Len(t, s.dryRunRequests(), 2)
		})
	})

	t.Run("failed", func(t *testing.T) {
		t.Parallel()

		withDryRunTestCluster(t, 0, func(ctx context.Context, s dryRunTestSetup) {
			s.onDryRun(func(*machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
				return nil, grpcstatus.Error(codes.InvalidArgument,
					"2 errors occurred:\n\t* .cluster.allowSchedulingOnControlPlanes is already set in v1alpha1 config\n\t* hostname is invalid\n\n")
			})

			s.lockCluster(ctx, t, true)
			s.setHostname(ctx, t, "dry-run-failed")

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_FAILED, spec.Outcome)
				a.Equal("2 errors occurred:\n\t* .cluster.allowSchedulingOnControlPlanes is already set in v1alpha1 config\n\t* hostname is invalid", spec.Error)
			})
		})
	})

	t.Run("reboot required", func(t *testing.T) {
		t.Parallel()

		withDryRunTestCluster(t, 0, func(ctx context.Context, s dryRunTestSetup) {
			s.onDryRun(func(*machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
				//nolint:staticcheck // Talos below 1.14 answers an AUTO apply with REBOOT. Drop when MinTalosVersion moves past 1.13.
				return &machine.ApplyConfigurationResponse{Messages: []*machine.ApplyConfiguration{{Mode: machine.ApplyConfigurationRequest_REBOOT}}}, nil
			})

			s.lockCluster(ctx, t, true)
			s.setHostname(ctx, t, "dry-run-reboot")

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
				a.True(spec.RebootRequired)
			})
		})
	})

	t.Run("no pending changes", func(t *testing.T) {
		t.Parallel()

		withDryRunTestCluster(t, 0, func(ctx context.Context, s dryRunTestSetup) {
			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NO_PENDING_CHANGES, spec.Outcome)
			})
		})
	})

	t.Run("generation error", func(t *testing.T) {
		t.Parallel()

		withDryRunTestCluster(t, 0, func(ctx context.Context, s dryRunTestSetup) {
			s.lockCluster(ctx, t, true)

			setGenerationError := func(generationError string) {
				rmock.Mock[*omni.ClusterMachineConfig](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.ClusterMachineConfig) error {
					res.TypedSpec().Value.GenerationError = generationError

					return nil
				}))
			}

			setGenerationError("failed to apply config patch")

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_FAILED, spec.Outcome)
				a.Equal("failed to apply config patch", spec.Error)
			})

			rtestutils.AssertResource(ctx, t, s.st, s.id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.Equal("failed to apply config patch", res.TypedSpec().Value.LastConfigError)
			})

			setGenerationError("")

			rtestutils.AssertResource(ctx, t, s.st, s.id, func(res *omni.ClusterMachineConfigStatus, a *assert.Assertions) {
				a.Empty(res.TypedSpec().Value.LastConfigError, "a fixed config clears the error while the cluster stays locked")
			})

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NO_PENDING_CHANGES, spec.Outcome)
			})
		})
	})

	t.Run("disconnected", func(t *testing.T) {
		t.Parallel()

		withDryRunTestCluster(t, 0, func(ctx context.Context, s dryRunTestSetup) {
			rmock.Mock[*omni.MachineStatus](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Connected = false

				return nil
			}))

			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NOT_CHECKED, spec.Outcome)
				a.Equal(specs.DryRunStatusSpec_DISCONNECTED, spec.NotCheckedReason)
			})
		})
	})

	t.Run("machine lock", func(t *testing.T) {
		t.Parallel()

		withDryRunTestCluster(t, 1, func(ctx context.Context, s dryRunTestSetup) {
			rmock.Mock[*omni.MachineSetNode](ctx, t, s.st, options.WithID(s.id), options.Modify(func(res *omni.MachineSetNode) error {
				res.Metadata().Annotations().Set(omni.MachineLocked, "")

				return nil
			}))

			s.setHostname(ctx, t, "dry-run-machine-lock")

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_PASSED, spec.Outcome)
			})
		})
	})

	t.Run("unlock", func(t *testing.T) {
		t.Parallel()

		withDryRunTestCluster(t, 0, func(ctx context.Context, s dryRunTestSetup) {
			s.lockCluster(ctx, t, true)

			s.assertOutcome(ctx, t, func(spec *specs.DryRunStatusSpec, a *assert.Assertions) {
				a.Equal(specs.DryRunStatusSpec_NO_PENDING_CHANGES, spec.Outcome)
			})

			s.lockCluster(ctx, t, false)

			rtestutils.AssertNoResource[*omni.DryRunStatus](ctx, t, s.st, s.id)
		})
	})
}
