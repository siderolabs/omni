// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

type MachineExtensionsSuite struct {
	OmniSuite
}

func TestMachineExtensionsReconcile(t *testing.T) {
	t.Parallel()

	setup := func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineExtensionsController()))
	}

	for _, tt := range []struct {
		labels           map[string]string
		machine          *omni.ClusterMachine
		name             string
		expectExtensions bool
	}{
		{
			name: "defined for a cluster",
			labels: map[string]string{
				omni.LabelCluster: "cluster",
			},
			expectExtensions: true,
		},
		{
			name: "defined for a different machine",
			labels: map[string]string{
				omni.LabelCluster:        "cluster",
				omni.LabelClusterMachine: "bbb",
			},
		},
		{
			name: "defined for this machine",
			labels: map[string]string{
				omni.LabelCluster:        "cluster",
				omni.LabelClusterMachine: "test",
			},
			expectExtensions: true,
		},
		{
			name: "defined for other machine set",
			labels: map[string]string{
				omni.LabelCluster:    "cluster",
				omni.LabelMachineSet: "aaa",
			},
		},
		{
			name: "defined for other this machine set",
			labels: map[string]string{
				omni.LabelCluster:    "cluster",
				omni.LabelMachineSet: "machineSet",
			},
			expectExtensions: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			machine := omni.NewClusterMachine("test")
			machine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
			machine.Metadata().Labels().Set(omni.LabelMachineSet, "machineSet")

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*3)
			defer cancel()

			testutils.WithRuntime(ctx, t, testutils.TestOptions{}, setup, func(ctx context.Context, testContext testutils.TestContext) {
				machineStatus := omni.NewMachineStatus(machine.Metadata().ID())

				conf := omni.NewExtensionsConfiguration("aaa")

				conf.Metadata().Labels().Do(func(tmp kvutils.TempKV) {
					for key, value := range tt.labels {
						tmp.Set(key, value)
					}
				})

				extensions := []string{"zzzz"}

				conf.TypedSpec().Value.Extensions = extensions

				require := require.New(t)

				require.NoError(testContext.State.Create(ctx, machineStatus))
				require.NoError(testContext.State.Create(ctx, machine))
				require.NoError(testContext.State.Create(ctx, conf))

				rtestutils.AssertResource(ctx, t, testContext.State, conf.Metadata().ID(), func(r *omni.ExtensionsConfiguration, assertion *assert.Assertions) {
					assertion.True(r.Metadata().Finalizers().Has(omnictrl.MachineExtensionsControllerName))
				})

				if tt.expectExtensions {
					rtestutils.AssertResources(ctx, t, testContext.State, []string{machine.Metadata().ID()}, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
						assertion.Equal(extensions, r.TypedSpec().Value.Extensions)
					})
				} else {
					rtestutils.AssertNoResource[*omni.MachineExtensions](ctx, t, testContext.State, machine.Metadata().ID())
				}
			})
		})
	}
}

func TestMachineExtensionsPriority(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		clusterLevelConfig := omni.NewExtensionsConfiguration("cluster-level")
		clusterLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterLevelConfig.TypedSpec().Value.Extensions = []string{"cluster-level"}

		machineSetLevelConfig := omni.NewExtensionsConfiguration("machine-set-level")
		machineSetLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		machineSetLevelConfig.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")
		machineSetLevelConfig.TypedSpec().Value.Extensions = []string{"machine-set-level"}

		clusterMachineLevel1 := omni.NewExtensionsConfiguration("cluster-machine-level-1")
		clusterMachineLevel1.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachineLevel1.Metadata().Labels().Set(omni.LabelClusterMachine, "cluster-machine")
		clusterMachineLevel1.TypedSpec().Value.Extensions = []string{"cluster-machine-level-1"}

		clusterMachineLevel2 := omni.NewExtensionsConfiguration("cluster-machine-level-2")
		clusterMachineLevel2.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachineLevel2.Metadata().Labels().Set(omni.LabelClusterMachine, "cluster-machine")
		clusterMachineLevel2.TypedSpec().Value.Extensions = []string{"cluster-machine-level-2"}

		st := testContext.State

		require.NoError(t, st.Create(ctx, clusterLevelConfig))
		require.NoError(t, st.Create(ctx, machineSetLevelConfig))
		require.NoError(t, st.Create(ctx, clusterMachineLevel1))
		require.NoError(t, st.Create(ctx, clusterMachineLevel2))

		clusterMachine := omni.NewClusterMachine("cluster-machine")
		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")

		require.NoError(t, st.Create(ctx, clusterMachine))

		controller := omnictrl.NewMachineExtensionsController()
		require.NoError(t, testContext.Runtime.RegisterQController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		rtestutils.AssertResource(ctx, t, testContext.State, "cluster-machine", func(res *omni.MachineExtensions, assertion *assert.Assertions) {
			assertion.Equal([]string{"cluster-machine-level-2"}, res.TypedSpec().Value.Extensions)
		})
	})
}

func TestMachineExtensionsPriorityWithNotMatchingMachineSet(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		clusterLevelConfig := omni.NewExtensionsConfiguration("cluster-level")
		clusterLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterLevelConfig.TypedSpec().Value.Extensions = []string{"cluster-level"}

		otherMachineSetLevelConfig := omni.NewExtensionsConfiguration("other-machine-set-level")
		otherMachineSetLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		otherMachineSetLevelConfig.Metadata().Labels().Set(omni.LabelMachineSet, "other-machine-set")
		otherMachineSetLevelConfig.TypedSpec().Value.Extensions = []string{"other-machine-set-level"}

		st := testContext.State

		require.NoError(t, st.Create(ctx, clusterLevelConfig))
		require.NoError(t, st.Create(ctx, otherMachineSetLevelConfig))

		clusterMachine := omni.NewClusterMachine("cluster-machine")
		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")

		require.NoError(t, st.Create(ctx, clusterMachine))

		controller := omnictrl.NewMachineExtensionsController()
		require.NoError(t, testContext.Runtime.RegisterQController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		rtestutils.AssertResource(ctx, t, testContext.State, "cluster-machine", func(res *omni.MachineExtensions, assertion *assert.Assertions) {
			assertion.Equal([]string{"cluster-level"}, res.TypedSpec().Value.Extensions)
		})
	})
}

func TestPreserveLegacyOrder(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		clusterLevelConfig := omni.NewExtensionsConfiguration("cluster-level")
		clusterLevelConfig.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)
		clusterLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterLevelConfig.TypedSpec().Value.Extensions = []string{"cluster-level"}

		machineSetLevelConfig := omni.NewExtensionsConfiguration("machine-set-level")
		machineSetLevelConfig.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)
		machineSetLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		machineSetLevelConfig.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")
		machineSetLevelConfig.TypedSpec().Value.Extensions = []string{"machine-set-level"}

		clusterMachineLevel1 := omni.NewExtensionsConfiguration("cluster-machine-level-1")
		clusterMachineLevel1.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)
		clusterMachineLevel1.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachineLevel1.Metadata().Labels().Set(omni.LabelClusterMachine, "cluster-machine")
		clusterMachineLevel1.TypedSpec().Value.Extensions = []string{"cluster-machine-level-1"}

		clusterMachineLevel2 := omni.NewExtensionsConfiguration("cluster-machine-level-2")
		clusterMachineLevel2.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)
		clusterMachineLevel2.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachineLevel2.Metadata().Labels().Set(omni.LabelClusterMachine, "cluster-machine")
		clusterMachineLevel2.TypedSpec().Value.Extensions = []string{"cluster-machine-level-2"}

		st := testContext.State

		require.NoError(t, st.Create(ctx, clusterLevelConfig))
		require.NoError(t, st.Create(ctx, machineSetLevelConfig))
		require.NoError(t, st.Create(ctx, clusterMachineLevel1))
		require.NoError(t, st.Create(ctx, clusterMachineLevel2))

		clusterMachine := omni.NewClusterMachine("cluster-machine")
		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")

		require.NoError(t, st.Create(ctx, clusterMachine))

		// prepare a MachineExtensions with the wrong extension list - assume that it picked the cluster level extensions instead of the cluster machine level ones.
		machineExtensions := omni.NewMachineExtensions("cluster-machine")
		machineExtensions.TypedSpec().Value.Extensions = []string{"cluster-level"}

		require.NoError(t, st.Create(ctx, machineExtensions, state.WithCreateOwner(omnictrl.MachineExtensionsControllerName)))

		controller := omnictrl.NewMachineExtensionsController()
		require.NoError(t, testContext.Runtime.RegisterQController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		rtestutils.AssertResource(ctx, t, testContext.State, "cluster-machine", func(res *omni.MachineExtensions, assertion *assert.Assertions) {
			_, annotationOk := res.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
			assertion.True(annotationOk)

			assertion.Equal([]string{"cluster-level"}, res.TypedSpec().Value.Extensions)
		})
	})
}

func TestMachineExtensionsDeleteOverride(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineExtensionsController()))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		st := testContext.State
		require := require.New(t)

		clusterMachine := omni.NewClusterMachine("machine")
		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")
		require.NoError(st.Create(ctx, clusterMachine))

		clusterLevel := omni.NewExtensionsConfiguration("cluster-level")
		clusterLevel.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterLevel.TypedSpec().Value.Extensions = []string{"a"}
		require.NoError(st.Create(ctx, clusterLevel))

		machineLevel := omni.NewExtensionsConfiguration("machine-level")
		machineLevel.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		machineLevel.Metadata().Labels().Set(omni.LabelClusterMachine, "machine")
		machineLevel.TypedSpec().Value.Extensions = []string{"b"}
		require.NoError(st.Create(ctx, machineLevel))

		rtestutils.AssertResource(ctx, t, st, machineLevel.Metadata().ID(), func(r *omni.ExtensionsConfiguration, assertion *assert.Assertions) {
			assertion.True(r.Metadata().Finalizers().Has(omnictrl.MachineExtensionsControllerName))
		})

		_, err := safe.StateUpdateWithConflicts(ctx, st, clusterLevel.Metadata(), func(r *omni.ExtensionsConfiguration) error {
			r.TypedSpec().Value.Extensions = []string{"a", "c"}

			return nil
		})
		require.NoError(err)

		var created time.Time

		rtestutils.AssertResources(ctx, t, st, []string{"machine"}, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
			assertion.Equal([]string{"b"}, r.TypedSpec().Value.Extensions)

			source, _ := r.Metadata().Labels().Get(omni.ExtensionsConfigurationLabel)
			assertion.Equal("machine-level", source)

			created = r.Metadata().Created()
		})

		rtestutils.Destroy[*omni.ExtensionsConfiguration](ctx, t, st, []string{"machine-level"})

		rtestutils.AssertResources(ctx, t, st, []string{"machine"}, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
			assertion.Equal([]string{"a", "c"}, r.TypedSpec().Value.Extensions)

			source, _ := r.Metadata().Labels().Get(omni.ExtensionsConfigurationLabel)
			assertion.Equal("cluster-level", source)

			assertion.Equal(created, r.Metadata().Created())
		})
	})
}

// machineExtensionsHolder keeps a finalizer on every MachineExtensions until released, like the schematic controller does.
type machineExtensionsHolder struct {
	release chan struct{}
}

func (h *machineExtensionsHolder) Name() string { return "MachineExtensionsHolder" }

func (h *machineExtensionsHolder) Inputs() []controller.Input {
	return []controller.Input{{Namespace: resources.DefaultNamespace, Type: omni.MachineExtensionsType, Kind: controller.InputStrong}}
}

func (h *machineExtensionsHolder) Outputs() []controller.Output { return nil }

func (h *machineExtensionsHolder) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	released := false

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-h.release:
			released = true
		}

		list, err := safe.ReaderListAll[*omni.MachineExtensions](ctx, r)
		if err != nil {
			return err
		}

		for me := range list.All() {
			switch {
			case me.Metadata().Phase() == resource.PhaseTearingDown && released:
				err = r.RemoveFinalizer(ctx, me.Metadata(), h.Name())
			case me.Metadata().Phase() == resource.PhaseRunning && !me.Metadata().Finalizers().Has(h.Name()):
				err = r.AddFinalizer(ctx, me.Metadata(), h.Name())
			}

			if err != nil {
				return err
			}
		}
	}
}

func TestMachineExtensionsDeleteWithHeldOutput(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	holder := &machineExtensionsHolder{release: make(chan struct{})}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineExtensionsController()))
		require.NoError(t, testContext.Runtime.RegisterController(holder))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		st := testContext.State
		require := require.New(t)

		clusterMachine := omni.NewClusterMachine("machine")
		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")
		require.NoError(st.Create(ctx, clusterMachine))

		machineSetLevel := omni.NewExtensionsConfiguration("machine-set-level")
		machineSetLevel.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		machineSetLevel.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")
		machineSetLevel.TypedSpec().Value.Extensions = []string{"a"}
		require.NoError(st.Create(ctx, machineSetLevel))

		rtestutils.AssertResources(ctx, t, st, []string{"machine"}, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
			assertion.Equal([]string{"a"}, r.TypedSpec().Value.Extensions)
			assertion.True(r.Metadata().Finalizers().Has(holder.Name()))
		})

		rtestutils.AssertResource(ctx, t, st, machineSetLevel.Metadata().ID(), func(r *omni.ExtensionsConfiguration, assertion *assert.Assertions) {
			assertion.True(r.Metadata().Finalizers().Has(omnictrl.MachineExtensionsControllerName))
		})

		// delete the only configuration, the output is torn down but held
		_, err := st.Teardown(ctx, machineSetLevel.Metadata())
		require.NoError(err)

		rtestutils.AssertResources(ctx, t, st, []string{"machine"}, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
			assertion.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		})

		// a new configuration appears while the output is held
		clusterLevel := omni.NewExtensionsConfiguration("cluster-level")
		clusterLevel.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterLevel.TypedSpec().Value.Extensions = []string{"b"}
		require.NoError(st.Create(ctx, clusterLevel))

		rtestutils.AssertResource(ctx, t, st, clusterLevel.Metadata().ID(), func(r *omni.ExtensionsConfiguration, assertion *assert.Assertions) {
			assertion.True(r.Metadata().Finalizers().Has(omnictrl.MachineExtensionsControllerName))
		})

		close(holder.release)

		rtestutils.Destroy[*omni.ExtensionsConfiguration](ctx, t, st, []string{machineSetLevel.Metadata().ID()})

		rtestutils.AssertResources(ctx, t, st, []string{"machine"}, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
			assertion.Equal(resource.PhaseRunning, r.Metadata().Phase())
			assertion.Equal([]string{"b"}, r.TypedSpec().Value.Extensions)
		})
	})
}
