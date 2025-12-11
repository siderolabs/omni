// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

//nolint:maintidx
func Test_TalosCARotation(t *testing.T) {
	t.Parallel()

	addControllers := func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(secrets.NewSecretRotationStatusController()))
	}

	t.Run("no rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, res.TypedSpec().Value.Component)
				})

				for _, id := range ids {
					rtestutils.AssertNoResource[*omni.ClusterMachineSecretsRotation](ctx, t, testContext.State, id)
				}
			},
		)
	})

	t.Run("rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, res.TypedSpec().Value.Component)
				})

				rotateSecretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				rotateData, err := json.Marshal(rotateSecretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.RotateData = rotateData
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, res.TypedSpec().Value.Component)
				})

				rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.ClusterMachineSecretsRotation, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterMachineSecretsRotationSpec_IDLE, res.TypedSpec().Value.Status)
				})

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.Data = rotateData
						res.TypedSpec().Value.RotateData = data
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_ROTATE

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, res.TypedSpec().Value.Component)
				})

				rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.ClusterMachineSecretsRotation, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterMachineSecretsRotationSpec_IDLE, res.TypedSpec().Value.Status)
				})

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_POST_ROTATE

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_POST_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, res.TypedSpec().Value.Component)
				})

				rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.ClusterMachineSecretsRotation, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterMachineSecretsRotationSpec_IDLE, res.TypedSpec().Value.Status)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_POST_ROTATE, res.TypedSpec().Value.Phase)
				})

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.RotateData = nil
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_NONE
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_OK

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, res.TypedSpec().Value.Component)
				})

				rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.ClusterMachineSecretsRotation, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterMachineSecretsRotationSpec_IDLE, res.TypedSpec().Value.Status)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
				})
			},
		)
	})

	t.Run("cluster locked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 1, 1)

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, res.TypedSpec().Value.Component)
				})

				rotateSecretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				rotateData, err := json.Marshal(rotateSecretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.RotateData = rotateData
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterStatus](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.Metadata().Annotations().Set(omni.ClusterLocked, "")

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(secrets.RotationPaused, res.TypedSpec().Value.Status)
					assertions.Equal("waiting for the cluster to be unlocked", res.TypedSpec().Value.Step)
				})

				rmock.Mock[*omni.ClusterStatus](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.Metadata().Annotations().Delete(omni.ClusterLocked)

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.NotEqual(secrets.RotationPaused, res.TypedSpec().Value.Status)
					assertions.NotEqual("waiting for the cluster to be unlocked", res.TypedSpec().Value.Step)
				})
			},
		)
	})

	t.Run("cluster unhealthy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 1, 1)

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, res.TypedSpec().Value.Component)
				})

				rotateSecretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				rotateData, err := json.Marshal(rotateSecretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterStatus](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.TypedSpec().Value.Ready = false
						res.TypedSpec().Value.Phase = specs.ClusterStatusSpec_SCALING_UP

						return nil
					}),
				)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.RotateData = rotateData
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, res.TypedSpec().Value.Component)
					assertions.Equal(secrets.RotationPaused, res.TypedSpec().Value.Status)
					assertions.Equal("waiting for the cluster to become ready", res.TypedSpec().Value.Step)
				})

				rmock.Mock[*omni.ClusterStatus](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.TypedSpec().Value.Ready = true
						res.TypedSpec().Value.Phase = specs.ClusterStatusSpec_RUNNING

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.NotEqual(secrets.RotationPaused, res.TypedSpec().Value.Status)
					assertions.NotEqual("waiting for the cluster to become ready", res.TypedSpec().Value.Step)
				})
			},
		)
	})

	t.Run("machine locked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 1, 1)

				machineID := machines[0].Metadata().ID()

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, res.TypedSpec().Value.Component)
				})

				rotateSecretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				rotateData, err := json.Marshal(rotateSecretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.Metadata().Annotations().Set(omni.MachineLocked, "")

						return nil
					}),
				)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.RotateData = rotateData
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, res.TypedSpec().Value.Component)
					assertions.Equal(secrets.RotationPaused, res.TypedSpec().Value.Status)
					assertions.Contains(res.TypedSpec().Value.Step, "waiting for a machine to be unlocked")
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.Metadata().Annotations().Delete(omni.MachineLocked)

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.NotEqual(secrets.RotationPaused, res.TypedSpec().Value.Status)
				})
			},
		)
	})

	t.Run("machine unhealthy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 1, 1)

				machineID := machines[0].Metadata().ID()

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, res.TypedSpec().Value.Component)
				})

				rotateSecretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				rotateData, err := json.Marshal(rotateSecretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.TypedSpec().Value.Ready = false

						return nil
					}),
				)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.RotateData = rotateData
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, res.TypedSpec().Value.Component)
					assertions.Equal(secrets.RotationPaused, res.TypedSpec().Value.Status)
					assertions.Contains(res.TypedSpec().Value.Step, "waiting for a machine to become ready")
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.TypedSpec().Value.Ready = true

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.NotEqual(secrets.RotationPaused, res.TypedSpec().Value.Status)
				})
			},
		)
	})

	t.Run("rotation ongoing", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 1, 1)

				machineID := machines[0].Metadata().ID()

				machineServices.Get(machineID).SetVersionHandler(
					func(ctx context.Context, _ *emptypb.Empty) (*machine.VersionResponse, error) {
						return nil, fmt.Errorf("failed to get version")
					},
				)

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, options.SameID(cluster), options.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_OK, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_NONE, res.TypedSpec().Value.Component)
				})

				rotateSecretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				rotateData, err := json.Marshal(rotateSecretsBundle)
				require.NoError(t, err)

				machineServices.Get(machineID).SetVersionHandler(
					func(ctx context.Context, _ *emptypb.Empty) (*machine.VersionResponse, error) {
						return nil, fmt.Errorf("failed to get version")
					},
				)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						res.TypedSpec().Value.RotateData = rotateData
						res.TypedSpec().Value.ComponentInRotation = specs.ClusterSecretsRotationStatusSpec_TALOS_CA
						res.TypedSpec().Value.RotationPhase = specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.ClusterSecretsRotationStatusSpec_TALOS_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Step, "rotating secret for machine: node-cp-0")
					assertions.Contains(res.TypedSpec().Value.Error, "failed to get version")
				})

				machineServices.Get(machineID).SetVersionHandler(nil)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Empty(res.TypedSpec().Value.Error)
				})
			},
		)
	})
}

//nolint:unparam
func createCluster(
	ctx context.Context,
	t *testing.T,
	st state.State,
	machineServices *testutils.MachineServices,
	clusterName string,
	controlPlanes, workers int,
	opts ...options.MockOption,
) (*omni.Cluster, []*omni.ClusterMachine) {
	clusterOptions := append([]options.MockOption{
		options.WithID(clusterName),
	}, opts...)

	cluster := rmock.Mock[*omni.Cluster](ctx, t, st,
		clusterOptions...,
	)

	rmock.Mock[*omni.ClusterConfigVersion](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.ClusterSecrets](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.TalosConfig](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.ClusterStatus](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.LoadBalancerConfig](ctx, t, st, options.SameID(cluster))

	cpMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.ControlPlanesResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelControlPlaneRole),
	)

	workersMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.WorkersResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelWorkerRole),
	)

	getIDs := func(machineType string, count int) []string {
		res := make([]string, 0, count)

		for i := range count {
			res = append(res, fmt.Sprintf("node-%s-%d", machineType, i))
		}

		return res
	}

	// create control planes
	rmock.MockList[*omni.MachineSetNode](ctx, t, st,
		options.IDs(getIDs("cp", controlPlanes)),
		options.ItemOptions(
			options.LabelCluster(cluster),
			options.LabelMachineSet(cpMachineSet),
			options.EmptyLabel(omni.LabelControlPlaneRole),
		),
	)

	if workers > 0 {
		// create workers
		rmock.MockList[*omni.MachineSetNode](ctx, t, st,
			options.IDs(getIDs("w", workers)),
			options.ItemOptions(
				options.LabelCluster(cluster),
				options.LabelMachineSet(workersMachineSet),
				options.EmptyLabel(omni.LabelWorkerRole),
			),
		)
	}

	cpMachines := rmock.MockList[*omni.ClusterMachine](ctx, t, st,
		options.QueryIDs[*omni.MachineSetNode](resource.LabelEqual(omni.LabelMachineSet, cpMachineSet.Metadata().ID())),
		options.ItemOptions(
			options.LabelCluster(cluster),
			options.LabelMachineSet(cpMachineSet),
			options.Modify(
				func(res *omni.ClusterMachine) error {
					res.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

					return nil
				},
			),
		),
	)

	workerMachines := rmock.MockList[*omni.ClusterMachine](ctx, t, st,
		options.QueryIDs[*omni.MachineSetNode](resource.LabelEqual(omni.LabelMachineSet, workersMachineSet.Metadata().ID())),
		options.ItemOptions(
			options.LabelCluster(cluster),
			options.LabelMachineSet(workersMachineSet),
			options.Modify(
				func(res *omni.ClusterMachine) error {
					res.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion

					return nil
				},
			),
		),
	)

	machines := slices.Concat(cpMachines, workerMachines)

	for _, machine := range machines {
		rmock.Mock[*siderolink.MachineJoinConfig](ctx, t, st, options.SameID(machine),
			options.Modify(func(res *siderolink.MachineJoinConfig) error {
				res.TypedSpec().Value.Config = &specs.JoinConfig{}

				return nil
			}),
		)

		rmock.Mock[*siderolink.Link](ctx, t, st, options.SameID(machine),
			options.Modify(func(res *siderolink.Link) error {
				res.TypedSpec().Value.Connected = true

				return nil
			}),
		)

		rmock.Mock[*omni.MachineStatus](ctx, t, st, options.SameID(machine),
			options.Modify(func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Maintenance = false

				return nil
			}),
		)

		rmock.Mock[*omni.Machine](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st, options.SameID(machine),
			options.Modify(func(res *omni.ClusterMachineStatus) error {
				service := machineServices.Create(ctx, res.Metadata().ID())
				helpers.CopyLabels(machine, res, omni.LabelCluster, omni.LabelMachineSet)
				res.Metadata().Labels().Set(omni.LabelHostname, res.Metadata().ID())
				res.TypedSpec().Value.ManagementAddress = service.SocketConnectionString
				res.TypedSpec().Value.Ready = true

				return nil
			}))
		rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.ClusterMachineConfigPatches](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.ClusterMachineConfig](ctx, t, st, options.SameID(machine))
	}

	return cluster, machines
}
