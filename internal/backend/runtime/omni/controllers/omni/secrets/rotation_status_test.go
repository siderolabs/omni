// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets_test

import (
	"context"
	stdx509 "crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	secretsctrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
	"github.com/siderolabs/omni/internal/pkg/constants"
)

//nolint:maintidx,gocognit
func Test_TalosCARotation(t *testing.T) {
	t.Parallel()

	//nolint:dupl
	t.Run("no rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, testContext.State, cluster.Metadata().ID())
				require.NoError(t, err)

				secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
				require.NoError(t, err)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
					assertion.Empty(res.TypedSpec().Value.Status)
					assertion.Empty(res.TypedSpec().Value.Step)
					assertion.Empty(res.TypedSpec().Value.Error)
				})

				rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.ClusterMachineSecrets, assertion *assert.Assertions) {
					assertion.Nil(res.TypedSpec().Value.GetRotation())
					assertion.Equal(string(clusterSecrets.TypedSpec().Value.Data), string(res.TypedSpec().Value.Data))
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.SecretRotation, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
					assertion.NotNil(res.TypedSpec().Value.GetCerts())
					assertion.NotNil(res.TypedSpec().Value.GetExtraCerts())
					assertion.Empty(res.TypedSpec().Value.GetBackupCertsOs())
					assertion.Equal(secretsBundle.Certs.OS.Crt, res.TypedSpec().Value.Certs.Os.Crt)
					assertion.Equal(secretsBundle.Certs.OS.Key, res.TypedSpec().Value.Certs.Os.Key)
					assertion.Nil(res.TypedSpec().Value.ExtraCerts.GetOs())
				})
			},
		)
	})

	t.Run("rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				cps := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isCP := m.Metadata().Labels().Get(omni.LabelControlPlaneRole)

					return isCP
				})

				cpIDs := xslices.Map(cps, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					if slices.Contains(cpIDs, m.ID()) {
						certSAN := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
						certSAN.TypedSpec().IPs = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
						certSAN.TypedSpec().DNSNames = []string{m.Address()}
						certSAN.TypedSpec().FQDN = m.ID()

						err := m.State.Create(ctx, certSAN)
						require.NoError(t, err)
					}
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				eventCh := make(chan safe.WrappedStateEvent[*omni.SecretRotation])

				var event safe.WrappedStateEvent[*omni.SecretRotation]

				require.NoError(t,
					safe.StateWatchKind(
						ctx,
						testContext.State,
						resource.NewMetadata(resources.DefaultNamespace, omni.SecretRotationType, "", resource.VersionUndefined),
						eventCh,
					))

				currentPhase := specs.SecretRotationSpec_OK
				nextPhase := func(phase specs.SecretRotationSpec_Phase) specs.SecretRotationSpec_Phase {
					switch phase {
					case specs.SecretRotationSpec_OK:
						return specs.SecretRotationSpec_PRE_ROTATE
					case specs.SecretRotationSpec_PRE_ROTATE:
						return specs.SecretRotationSpec_ROTATE
					case specs.SecretRotationSpec_ROTATE:
						return specs.SecretRotationSpec_POST_ROTATE
					case specs.SecretRotationSpec_POST_ROTATE:
						return specs.SecretRotationSpec_OK
					}

					t.Fatalf("unexpected phase: %s", phase.String())

					return specs.SecretRotationSpec_OK
				}

				// trigger rotation
				rmock.Mock[*omni.RotateTalosCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				processesStages := 0
				expectedStages := 4

				for {
					select {
					case event = <-eventCh:
					case <-ctx.Done():
						t.Fatal("timeout")
					}

					require.NoError(t, event.Error(), "error received in secret rotation event")

					rotation, err := event.Resource()
					require.NoError(t, err)

					if rotation.TypedSpec().Value.Phase == currentPhase {
						processesStages++
						currentPhase = nextPhase(currentPhase)
					}

					if processesStages == expectedStages {
						break
					}
				}

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.SecretRotation, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
					assertion.NotEmpty(res.TypedSpec().Value.GetBackupCertsOs())
					assertion.Equal(1, len(res.TypedSpec().Value.GetBackupCertsOs()))
				})
			},
		)
	})

	t.Run("cluster locked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				cps := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isCP := m.Metadata().Labels().Get(omni.LabelControlPlaneRole)

					return isCP
				})

				cpIDs := xslices.Map(cps, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					if slices.Contains(cpIDs, m.ID()) {
						certSAN := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
						certSAN.TypedSpec().IPs = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
						certSAN.TypedSpec().DNSNames = []string{m.Address()}
						certSAN.TypedSpec().FQDN = m.ID()

						err := m.State.Create(ctx, certSAN)
						require.NoError(t, err)
					}
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterStatus](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.Metadata().Annotations().Set(omni.ClusterLocked, "")

						return nil
					}),
				)

				// trigger rotation
				rmock.Mock[*omni.RotateTalosCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Contains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
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
					assertions.NotContains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
					assertions.NotEqual("waiting for the cluster to be unlocked", res.TypedSpec().Value.Step)
				})
			},
		)
	})

	t.Run("cluster unhealthy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				cps := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isCP := m.Metadata().Labels().Get(omni.LabelControlPlaneRole)

					return isCP
				})

				cpIDs := xslices.Map(cps, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					if slices.Contains(cpIDs, m.ID()) {
						certSAN := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
						certSAN.TypedSpec().IPs = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
						certSAN.TypedSpec().DNSNames = []string{m.Address()}
						certSAN.TypedSpec().FQDN = m.ID()

						err := m.State.Create(ctx, certSAN)
						require.NoError(t, err)
					}
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterStatus](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.TypedSpec().Value.Ready = false
						res.TypedSpec().Value.Phase = specs.ClusterStatusSpec_SCALING_UP

						return nil
					}),
				)

				// trigger rotation
				rmock.Mock[*omni.RotateTalosCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.SecretRotationSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.SecretRotationSpec_TALOS_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
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
					assertions.NotContains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
					assertions.NotEqual("waiting for the cluster to become ready", res.TypedSpec().Value.Step)
				})
			},
		)
	})

	t.Run("machine locked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				controlPlanes := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isCP := m.Metadata().Labels().Get(omni.LabelControlPlaneRole)

					return isCP
				})
				workers := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isWorker := m.Metadata().Labels().Get(omni.LabelWorkerRole)

					return isWorker
				})

				cpIDs := xslices.Map(controlPlanes, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})
				machineID := workers[0].Metadata().ID()

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					if slices.Contains(cpIDs, m.ID()) {
						certSAN := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
						certSAN.TypedSpec().IPs = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
						certSAN.TypedSpec().DNSNames = []string{m.Address()}
						certSAN.TypedSpec().FQDN = m.ID()

						err := m.State.Create(ctx, certSAN)
						require.NoError(t, err)
					}
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.Metadata().Annotations().Set(omni.MachineLocked, "")

						return nil
					}),
				)

				// trigger rotation
				rmock.Mock[*omni.RotateTalosCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.SecretRotationSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.SecretRotationSpec_TALOS_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
					assertions.Contains(res.TypedSpec().Value.Step, "waiting for machines")
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.Metadata().Annotations().Delete(omni.MachineLocked)

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.NotContains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
				})
			},
		)
	})

	t.Run("machine unhealthy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				controlPlanes := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isCP := m.Metadata().Labels().Get(omni.LabelControlPlaneRole)

					return isCP
				})
				workers := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isWorker := m.Metadata().Labels().Get(omni.LabelWorkerRole)

					return isWorker
				})

				cpIDs := xslices.Map(controlPlanes, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})
				machineID := workers[0].Metadata().ID()

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					if slices.Contains(cpIDs, m.ID()) {
						certSAN := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
						certSAN.TypedSpec().IPs = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
						certSAN.TypedSpec().DNSNames = []string{m.Address()}
						certSAN.TypedSpec().FQDN = m.ID()

						err := m.State.Create(ctx, certSAN)
						require.NoError(t, err)
					}
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.TypedSpec().Value.Ready = false

						return nil
					}),
				)

				// trigger rotation
				rmock.Mock[*omni.RotateTalosCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.SecretRotationSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.SecretRotationSpec_TALOS_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
					assertions.Contains(res.TypedSpec().Value.Step, "waiting for machines")
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.TypedSpec().Value.Ready = true

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.NotContains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
				})
			},
		)
	})

	t.Run("rotation ongoing", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				cps := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isCP := m.Metadata().Labels().Get(omni.LabelControlPlaneRole)

					return isCP
				})

				cpIDs := xslices.Map(cps, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})
				machineID := machines[0].Metadata().ID()

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					if slices.Contains(cpIDs, m.ID()) {
						certSAN := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
						certSAN.TypedSpec().IPs = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
						certSAN.TypedSpec().DNSNames = []string{m.Address()}
						certSAN.TypedSpec().FQDN = m.ID()

						err := m.State.Create(ctx, certSAN)
						require.NoError(t, err)
					}
				})

				machineServices.Get(machineID).SetVersionHandler(
					func(ctx context.Context, _ *emptypb.Empty) (*machine.VersionResponse, error) {
						return nil, fmt.Errorf("failed to get version")
					},
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				// trigger rotation
				rmock.Mock[*omni.RotateTalosCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.SecretRotationSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.SecretRotationSpec_TALOS_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Step, "rotating secret for machines: [node-cp-0]")
				})

				machineServices.Get(machineID).SetVersionHandler(nil)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Empty(res.TypedSpec().Value.Error)
				})
			},
		)
	})
}

//nolint:maintidx,gocognit,gocyclo,cyclop
func Test_KubernetesCARotation(t *testing.T) {
	t.Parallel()

	ensureCerts := func(ctx context.Context, st state.State, machineID, path string) ([]byte, error) {
		secrets, err := safe.ReaderGetByID[*omni.ClusterMachineSecrets](ctx, st, machineID)
		if err != nil {
			return nil, fmt.Errorf("failed to get cluster machine secrets: %w", err)
		}

		secretsBundle, err := omni.ToSecretsBundle(secrets.TypedSpec().Value.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse secrets bundle: %w", err)
		}

		k8sCA := secretsBundle.Certs.K8s

		ca, err := talosx509.NewCertificateAuthorityFromCertificateAndKey(k8sCA)
		if err != nil {
			return nil, fmt.Errorf("failed to create CA from certificate and key: %w", err)
		}

		var certBytes []byte

		switch path {
		case filepath.Join(talosconstants.KubeletPKIDir, "kubelet-client-current.pem"):
			// Generate client certificate for kubelet
			clientKeyPair, certErr := talosx509.NewKeyPair(ca,
				talosx509.CommonName("system:node:"+machineID),
				talosx509.Organization("system:nodes"),
				talosx509.NotBefore(time.Now().Add(-10*time.Second)),
				talosx509.NotAfter(time.Now().Add(24*time.Hour)),
				talosx509.KeyUsage(stdx509.KeyUsageDigitalSignature|stdx509.KeyUsageKeyEncipherment),
				talosx509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageClientAuth}),
			)
			if certErr != nil {
				return nil, fmt.Errorf("failed to generate kubelet client certificate: %w", certErr)
			}

			certBytes = clientKeyPair.CrtPEM // talosx509.NewCertificateAndKeyFromKeyPair(clientKeyPair).Crt

		case filepath.Join(talosconstants.KubernetesAPIServerSecretsDir, "apiserver.crt"):
			// Generate server certificate for apiserver
			serverKeyPair, certErr := talosx509.NewKeyPair(ca,
				talosx509.CommonName("kube-apiserver"),
				talosx509.DNSNames([]string{"kubernetes", "kubernetes.default", "kubernetes.default.svc", "kubernetes.default.svc.cluster.local"}),
				talosx509.IPAddresses([]net.IP{net.ParseIP("127.0.0.1")}),
				talosx509.NotBefore(time.Now().Add(-10*time.Second)),
				talosx509.NotAfter(time.Now().Add(24*time.Hour)),
				talosx509.KeyUsage(stdx509.KeyUsageDigitalSignature|stdx509.KeyUsageKeyEncipherment),
				talosx509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
			)
			if certErr != nil {
				return nil, fmt.Errorf("failed to generate apiserver certificate: %w", certErr)
			}

			certBytes = serverKeyPair.CrtPEM // talosx509.NewCertificateAndKeyFromKeyPair(serverKeyPair).Crt

		default:
			return nil, fmt.Errorf("unexpected path requested: %s", path)
		}

		return certBytes, nil
	}

	//nolint:dupl
	t.Run("no rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)

				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-k8s-ca", 3, 2)
				ids := xslices.Map(machines, func(m *omni.ClusterMachine) string {
					return m.Metadata().ID()
				})

				clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, testContext.State, cluster.Metadata().ID())
				require.NoError(t, err)

				secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
				require.NoError(t, err)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
					assertion.Empty(res.TypedSpec().Value.Status)
					assertion.Empty(res.TypedSpec().Value.Step)
					assertion.Empty(res.TypedSpec().Value.Error)
				})

				rtestutils.AssertResources(ctx, t, testContext.State, ids, func(res *omni.ClusterMachineSecrets, assertion *assert.Assertions) {
					assertion.Nil(res.TypedSpec().Value.GetRotation())
					assertion.Equal(string(clusterSecrets.TypedSpec().Value.Data), string(res.TypedSpec().Value.Data))
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.SecretRotation, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
					assertion.NotNil(res.TypedSpec().Value.GetCerts())
					assertion.NotNil(res.TypedSpec().Value.GetExtraCerts())
					assertion.Empty(res.TypedSpec().Value.GetBackupCertsK8S())
					assertion.Equal(secretsBundle.Certs.K8s.Crt, res.TypedSpec().Value.Certs.K8S.Crt)
					assertion.Equal(secretsBundle.Certs.K8s.Key, res.TypedSpec().Value.Certs.K8S.Key)
					assertion.Nil(res.TypedSpec().Value.ExtraCerts.GetK8S())
				})
			},
		)
	})

	t.Run("rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-k8s-ca", 3, 2)

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.SetReadHandler(func(request *machine.ReadRequest, g grpc.ServerStreamingServer[common.Data]) error {
						cert, err := ensureCerts(ctx, testContext.State, m.ID(), request.Path)
						if err != nil {
							return err
						}

						return g.Send(&common.Data{Bytes: cert})
					})
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				eventCh := make(chan safe.WrappedStateEvent[*omni.SecretRotation])

				var event safe.WrappedStateEvent[*omni.SecretRotation]

				require.NoError(t,
					safe.StateWatchKind(
						ctx,
						testContext.State,
						resource.NewMetadata(resources.DefaultNamespace, omni.SecretRotationType, "", resource.VersionUndefined),
						eventCh,
					))

				currentPhase := specs.SecretRotationSpec_OK
				nextPhase := func(phase specs.SecretRotationSpec_Phase) specs.SecretRotationSpec_Phase {
					switch phase {
					case specs.SecretRotationSpec_OK:
						return specs.SecretRotationSpec_PRE_ROTATE
					case specs.SecretRotationSpec_PRE_ROTATE:
						return specs.SecretRotationSpec_ROTATE
					case specs.SecretRotationSpec_ROTATE:
						return specs.SecretRotationSpec_POST_ROTATE
					case specs.SecretRotationSpec_POST_ROTATE:
						return specs.SecretRotationSpec_OK
					}

					t.Fatalf("unexpected phase: %s", phase.String())

					return specs.SecretRotationSpec_OK
				}

				// trigger rotation
				rmock.Mock[*omni.RotateKubernetesCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				processesStages := 0
				expectedStages := 4

				for {
					select {
					case event = <-eventCh:
					case <-ctx.Done():
						t.Fatal("timeout")
					}

					require.NoError(t, event.Error(), "error received in secret rotation event")

					rotation, err := event.Resource()
					require.NoError(t, err)

					if rotation.TypedSpec().Value.Phase == currentPhase {
						processesStages++
						currentPhase = nextPhase(currentPhase)
					}

					if processesStages == expectedStages {
						break
					}
				}

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.SecretRotation, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
					assertion.NotEmpty(res.TypedSpec().Value.GetBackupCertsK8S())
					assertion.Equal(1, len(res.TypedSpec().Value.GetBackupCertsK8S()))
				})
			},
		)
	})

	t.Run("cluster locked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.SetReadHandler(func(request *machine.ReadRequest, g grpc.ServerStreamingServer[common.Data]) error {
						cert, err := ensureCerts(ctx, testContext.State, m.ID(), request.Path)
						if err != nil {
							return err
						}

						return g.Send(&common.Data{Bytes: cert})
					})
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterStatus](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.Metadata().Annotations().Set(omni.ClusterLocked, "")

						return nil
					}),
				)

				// trigger rotation
				rmock.Mock[*omni.RotateKubernetesCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Contains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
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
					assertions.NotContains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
					assertions.NotEqual("waiting for the cluster to be unlocked", res.TypedSpec().Value.Step)
				})
			},
		)
	})

	t.Run("cluster unhealthy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.SetReadHandler(func(request *machine.ReadRequest, g grpc.ServerStreamingServer[common.Data]) error {
						cert, err := ensureCerts(ctx, testContext.State, m.ID(), request.Path)
						if err != nil {
							return err
						}

						return g.Send(&common.Data{Bytes: cert})
					})
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterStatus](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterStatus) error {
						res.TypedSpec().Value.Ready = false
						res.TypedSpec().Value.Phase = specs.ClusterStatusSpec_SCALING_UP

						return nil
					}),
				)

				// trigger rotation
				rmock.Mock[*omni.RotateKubernetesCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.SecretRotationSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.SecretRotationSpec_KUBERNETES_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
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
					assertions.NotContains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
					assertions.NotEqual("waiting for the cluster to become ready", res.TypedSpec().Value.Step)
				})
			},
		)
	})

	t.Run("machine locked", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				workers := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isWorker := m.Metadata().Labels().Get(omni.LabelWorkerRole)

					return isWorker
				})

				machineID := workers[0].Metadata().ID()

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.SetReadHandler(func(request *machine.ReadRequest, g grpc.ServerStreamingServer[common.Data]) error {
						cert, err := ensureCerts(ctx, testContext.State, m.ID(), request.Path)
						if err != nil {
							return err
						}

						return g.Send(&common.Data{Bytes: cert})
					})
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.Metadata().Annotations().Set(omni.MachineLocked, "")

						return nil
					}),
				)

				// trigger rotation
				rmock.Mock[*omni.RotateKubernetesCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.SecretRotationSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.SecretRotationSpec_KUBERNETES_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
					assertions.Contains(res.TypedSpec().Value.Step, "waiting for machines")
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.Metadata().Annotations().Delete(omni.MachineLocked)

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.NotContains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
				})
			},
		)
	})

	t.Run("machine unhealthy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)
				workers := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
					_, isWorker := m.Metadata().Labels().Get(omni.LabelWorkerRole)

					return isWorker
				})

				machineID := workers[0].Metadata().ID()

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.SetReadHandler(func(request *machine.ReadRequest, g grpc.ServerStreamingServer[common.Data]) error {
						cert, err := ensureCerts(ctx, testContext.State, m.ID(), request.Path)
						if err != nil {
							return err
						}

						return g.Send(&common.Data{Bytes: cert})
					})
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.TypedSpec().Value.Ready = false

						return nil
					}),
				)

				// trigger rotation
				rmock.Mock[*omni.RotateKubernetesCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.SecretRotationSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.SecretRotationSpec_KUBERNETES_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
					assertions.Contains(res.TypedSpec().Value.Step, "waiting for machines")
				})

				rmock.Mock[*omni.ClusterMachineStatus](ctx, t, testContext.State,
					options.WithID(machineID),
					options.Modify(func(res *omni.ClusterMachineStatus) error {
						res.TypedSpec().Value.Ready = true

						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.NotContains(res.TypedSpec().Value.Status, secretsctrl.RotationPaused)
				})
			},
		)
	})

	t.Run("rotation ongoing", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(
					secretsctrl.NewSecretRotationStatusController(
						&fakeRemoteGeneratorFactory{testContext.State},
						&fakeKubernetesClientFactory{},
					)))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				machineServices := testutils.NewMachineServices(t, testContext.State)
				cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "rotate-talos-ca", 3, 2)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
					assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
					assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
				})

				// trigger rotation
				rmock.Mock[*omni.RotateKubernetesCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State,
					options.WithID(cluster.Metadata().ID()),
					options.Modify(func(res *omni.ClusterSecrets) error {
						return nil
					}),
				)

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Equal(specs.SecretRotationSpec_PRE_ROTATE, res.TypedSpec().Value.Phase)
					assertions.Equal(specs.SecretRotationSpec_KUBERNETES_CA, res.TypedSpec().Value.Component)
					assertions.Contains(res.TypedSpec().Value.Step, "rotating secret for machines: [node-cp-0]")
				})

				machineServices.ForEach(func(m *testutils.MachineServiceMock) {
					m.SetReadHandler(func(request *machine.ReadRequest, g grpc.ServerStreamingServer[common.Data]) error {
						cert, err := ensureCerts(ctx, testContext.State, m.ID(), request.Path)
						if err != nil {
							return err
						}

						return g.Send(&common.Data{Bytes: cert})
					})
				})

				rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertions *assert.Assertions) {
					assertions.Empty(res.TypedSpec().Value.Error)
				})
			},
		)
	})
}

func Test_ConcurrentRotationRejection(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(ctx context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(
				secretsctrl.NewSecretRotationStatusController(
					&fakeRemoteGeneratorFactory{testContext.State},
					&fakeKubernetesClientFactory{},
				)))
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
			require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			machineServices := testutils.NewMachineServices(t, testContext.State)
			cluster, machines := createCluster(ctx, t, testContext.State, machineServices, "concurrent-rotation", 3, 2)
			cps := xslices.Filter(machines, func(m *omni.ClusterMachine) bool {
				_, isCP := m.Metadata().Labels().Get(omni.LabelControlPlaneRole)

				return isCP
			})

			cpIDs := xslices.Map(cps, func(m *omni.ClusterMachine) string {
				return m.Metadata().ID()
			})

			machineServices.ForEach(func(m *testutils.MachineServiceMock) {
				if slices.Contains(cpIDs, m.ID()) {
					certSAN := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
					certSAN.TypedSpec().IPs = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
					certSAN.TypedSpec().DNSNames = []string{m.Address()}
					certSAN.TypedSpec().FQDN = m.ID()

					err := m.State.Create(ctx, certSAN)
					require.NoError(t, err)
				}
			})

			rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
				assertion.Equal(specs.SecretRotationSpec_NONE, res.TypedSpec().Value.Component)
			})

			// Trigger Talos CA rotation first
			rmock.Mock[*omni.RotateTalosCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

			// Wait for Talos CA rotation to start (phase changes from OK)
			rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.SecretRotationSpec_TALOS_CA, res.TypedSpec().Value.Component)
				assertion.NotEqual(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
			})

			// Trigger Kubernetes CA rotation while Talos CA rotation is in progress
			rmock.Mock[*omni.RotateKubernetesCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

			rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.SecretRotationSpec_TALOS_CA, res.TypedSpec().Value.Component)
			})
		},
	)
}

func Test_ComponentIsolationDuringRotation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*15)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(ctx context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(
				secretsctrl.NewSecretRotationStatusController(
					&fakeRemoteGeneratorFactory{testContext.State},
					&fakeKubernetesClientFactory{},
				)))
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil, "ghcr.io/siderolabs/installer")))
			require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory", "ghcr.io/siderolabs/installer")))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			machineServices := testutils.NewMachineServices(t, testContext.State)
			cluster, _ := createCluster(ctx, t, testContext.State, machineServices, "component-isolation", 3, 2)

			rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.ClusterSecretsRotationStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.SecretRotationSpec_OK, res.TypedSpec().Value.Phase)
			})

			var initialTalosOsCrt, initialTalosOsKey []byte

			rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.SecretRotation, assertion *assert.Assertions) {
				assertion.NotNil(res.TypedSpec().Value.GetCerts())
				assertion.NotNil(res.TypedSpec().Value.Certs.GetOs())
				initialTalosOsCrt = res.TypedSpec().Value.Certs.Os.Crt
				initialTalosOsKey = res.TypedSpec().Value.Certs.Os.Key
			})

			machineServices.ForEach(func(m *testutils.MachineServiceMock) {
				m.SetReadHandler(func(request *machine.ReadRequest, g grpc.ServerStreamingServer[common.Data]) error {
					// Return dummy cert data - the actual validation is mocked
					return g.Send(&common.Data{Bytes: []byte("mock-cert-data")})
				})
			})

			// Trigger Kubernetes CA rotation
			rmock.Mock[*omni.RotateKubernetesCA](ctx, t, testContext.State, options.WithID(cluster.Metadata().ID()))

			rtestutils.AssertResource(ctx, t, testContext.State, cluster.Metadata().ID(), func(res *omni.SecretRotation, assertion *assert.Assertions) {
				if res.TypedSpec().Value.Component != specs.SecretRotationSpec_KUBERNETES_CA {
					return
				}

				assertion.Equal(initialTalosOsCrt, res.TypedSpec().Value.Certs.Os.Crt, "Talos CA cert should be preserved during Kubernetes CA rotation")
				assertion.Equal(initialTalosOsKey, res.TypedSpec().Value.Certs.Os.Key, "Talos CA key should be preserved during Kubernetes CA rotation")

				if res.TypedSpec().Value.ExtraCerts != nil {
					assertion.Nil(res.TypedSpec().Value.ExtraCerts.GetOs(), "Talos CA extra certs should be nil during Kubernetes CA rotation")
				}
			})
		},
	)
}

func Test_BackupCertificateLimitConstant(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 5, secretsctrl.BackedUpRotatedSecretsLimit,
		"BackedUpRotatedSecretsLimit should be 5 to store historical CA certificates for rollback purposes")

	assert.Greater(t, secretsctrl.BackedUpRotatedSecretsLimit, 0,
		"BackedUpRotatedSecretsLimit must be positive")
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

	rmock.Mock[*omni.ClusterConfigVersion](ctx, t, st, options.SameID(cluster), options.Modify(func(res *omni.ClusterConfigVersion) error {
		res.TypedSpec().Value.Version = "v" + constants.DefaultTalosVersion

		return nil
	}))
	rmock.Mock[*omni.ClusterSecrets](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.TalosConfig](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.ClusterStatus](ctx, t, st, options.SameID(cluster))
	rmock.Mock[*omni.LoadBalancerConfig](ctx, t, st, options.SameID(cluster), options.Modify(func(res *omni.LoadBalancerConfig) error {
		res.TypedSpec().Value.SiderolinkEndpoint = "https://[fdae:41e4:649b:9303::1]:10000"

		return nil
	}))

	cpMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.ControlPlanesResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelControlPlaneRole),
	)
	rmock.Mock[*omni.MachineSetStatus](ctx, t, st, options.SameID(cpMachineSet))
	rmock.Mock[*omni.MachineSetConfigStatus](ctx, t, st, options.SameID(cpMachineSet))

	workersMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.WorkersResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelWorkerRole),
	)
	rmock.Mock[*omni.MachineSetStatus](ctx, t, st, options.SameID(workersMachineSet))
	rmock.Mock[*omni.MachineSetConfigStatus](ctx, t, st, options.SameID(workersMachineSet))

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
				res.TypedSpec().Value.Config = &specs.JoinConfig{
					Config: `
apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: grpc://omni.localhost:8090?jointoken=test-token
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092`,
				}

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
				service := machineServices.Create(ctx, res.Metadata().ID())
				res.TypedSpec().Value.Maintenance = false
				res.TypedSpec().Value.ManagementAddress = service.SocketConnectionString

				return nil
			}),
		)

		rmock.Mock[*omni.Machine](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.ClusterMachineStatus](ctx, t, st, options.SameID(machine),
			options.Modify(func(res *omni.ClusterMachineStatus) error {
				service := machineServices.Get(res.Metadata().ID())
				helpers.CopyLabels(machine, res, omni.LabelCluster, omni.LabelMachineSet, omni.LabelControlPlaneRole, omni.LabelWorkerRole)
				res.Metadata().Labels().Set(omni.LabelHostname, res.Metadata().ID())
				res.TypedSpec().Value.ManagementAddress = service.SocketConnectionString
				res.TypedSpec().Value.Ready = true

				return nil
			}))
		rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.ClusterMachineConfigPatches](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.MachineStatusSnapshot](ctx, t, st, options.SameID(machine))
		rmock.Mock[*omni.MachineConfigGenOptions](ctx, t, st, options.SameID(machine))
	}

	return cluster, machines
}
