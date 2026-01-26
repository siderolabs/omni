// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets_test

import (
	"bytes"
	"context"
	stdx509 "crypto/x509"
	"encoding/pem"
	"fmt"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/secretrotation"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	secretsctrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
	"github.com/siderolabs/omni/internal/pkg/constants"
	"github.com/siderolabs/omni/internal/pkg/siderolink/trustd"
)

type fakeRemoteGeneratorFactory struct {
	omniState state.State
}

type remoteGenerator struct {
	omniState state.State
}

func (r remoteGenerator) IdentityContext(ctx context.Context, csr *x509.CertificateSigningRequest) (ca, crt []byte, err error) {
	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r.omniState, csr.X509CertificateRequest.Subject.CommonName)
	if err != nil {
		return nil, nil, err
	}

	clusterID, _ := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)

	clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, r.omniState, clusterID)
	if err != nil {
		return nil, nil, err
	}

	secretsBundle, err := omni.ToSecretsBundle(clusterSecrets.TypedSpec().Value.Data)
	if err != nil {
		return nil, nil, err
	}

	issuingCA, acceptedCAs, err := trustd.GetIssuingAndAcceptedCAs(ctx, r.omniState, secretsBundle, clusterSecrets.Metadata().ID())
	if err != nil {
		return nil, nil, err
	}

	csrPemBlock, _ := pem.Decode(csr.X509CertificateRequestPEM)
	if csrPemBlock == nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "failed to decode CSR")
	}

	x509Opts := []x509.Option{
		x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
	}

	signed, err := x509.NewCertificateFromCSRBytes(
		issuingCA.Crt,
		issuingCA.Key,
		csr.X509CertificateRequestPEM,
		x509Opts...,
	)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed to sign CSR: %s", err)
	}

	return bytes.Join(
		xslices.Map(
			acceptedCAs,
			func(cert *x509.PEMEncodedCertificate) []byte {
				return cert.Crt
			},
		),
		nil,
	), signed.X509CertificatePEM, nil
}

func (r remoteGenerator) Close() error {
	return nil
}

func (f *fakeRemoteGeneratorFactory) NewRemoteGenerator(string, []string, []*x509.PEMEncodedCertificate) (secretrotation.RemoteGenerator, error) {
	return remoteGenerator{omniState: f.omniState}, nil
}

//nolint:maintidx,gocognit
func Test_TalosCARotation(t *testing.T) {
	t.Parallel()

	t.Run("no rotation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{},
			func(ctx context.Context, testContext testutils.TestContext) {
				require.NoError(t, testContext.Runtime.RegisterQController(secretsctrl.NewSecretRotationStatusController(&fakeRemoteGeneratorFactory{testContext.State})))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil)))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory")))
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
				require.NoError(t, testContext.Runtime.RegisterQController(secretsctrl.NewSecretRotationStatusController(&fakeRemoteGeneratorFactory{testContext.State})))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil)))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory")))
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
				require.NoError(t, testContext.Runtime.RegisterQController(secretsctrl.NewSecretRotationStatusController(&fakeRemoteGeneratorFactory{testContext.State})))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil)))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory")))
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
					assertions.Equal(secretsctrl.RotationPaused, res.TypedSpec().Value.Status)
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
					assertions.NotEqual(secretsctrl.RotationPaused, res.TypedSpec().Value.Status)
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
				require.NoError(t, testContext.Runtime.RegisterQController(secretsctrl.NewSecretRotationStatusController(&fakeRemoteGeneratorFactory{testContext.State})))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil)))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory")))
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
					assertions.Equal(secretsctrl.RotationPaused, res.TypedSpec().Value.Status)
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
					assertions.NotEqual(secretsctrl.RotationPaused, res.TypedSpec().Value.Status)
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
				require.NoError(t, testContext.Runtime.RegisterQController(secretsctrl.NewSecretRotationStatusController(&fakeRemoteGeneratorFactory{testContext.State})))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil)))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory")))
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
					assertions.Equal(secretsctrl.RotationPaused, res.TypedSpec().Value.Status)
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
					assertions.NotEqual(secretsctrl.RotationPaused, res.TypedSpec().Value.Status)
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
				require.NoError(t, testContext.Runtime.RegisterQController(secretsctrl.NewSecretRotationStatusController(&fakeRemoteGeneratorFactory{testContext.State})))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil)))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory")))
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
					assertions.Equal(secretsctrl.RotationPaused, res.TypedSpec().Value.Status)
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
					assertions.NotEqual(secretsctrl.RotationPaused, res.TypedSpec().Value.Status)
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
				require.NoError(t, testContext.Runtime.RegisterQController(secretsctrl.NewSecretRotationStatusController(&fakeRemoteGeneratorFactory{testContext.State})))
				require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewClusterMachineConfigController("test.factory", nil)))
				require.NoError(t, testContext.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController("test.factory")))
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
	rmock.Mock[*omni.LoadBalancerConfig](ctx, t, st, options.SameID(cluster))

	cpMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.ControlPlanesResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelControlPlaneRole),
	)
	rmock.Mock[*omni.MachineSetStatus](ctx, t, st, options.SameID(cpMachineSet))

	workersMachineSet := rmock.Mock[*omni.MachineSet](ctx, t, st,
		options.WithID(omni.WorkersResourceID(clusterName)),
		options.LabelCluster(cluster),
		options.EmptyLabel(omni.LabelWorkerRole),
	)
	rmock.Mock[*omni.MachineSetStatus](ctx, t, st, options.SameID(workersMachineSet))

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
