// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets_test

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	talosx509 "github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	testoptions "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

//nolint:maintidx,dupl
func Test_Talosconfig(t *testing.T) {
	t.Parallel()

	addControllers := func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(secrets.NewTalosConfigController(2*time.Second)))
	}

	t.Run("reconcile", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "test-reconcile"

				cluster := omni.NewCluster(clusterName)
				cluster.TypedSpec().Value.TalosVersion = constants.DefaultTalosVersion
				require.NoError(t, st.Create(ctx, cluster))

				machineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(cluster.Metadata().ID()))

				machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
				machineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)

				require.NoError(t, st.Create(ctx, machineSet))

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				var firstCrt string

				config := omni.NewTalosConfig(clusterName)
				rtestutils.AssertResource(ctx, t, st, config.Metadata().ID(),
					func(res *omni.TalosConfig, _ *assert.Assertions) {
						spec := res.TypedSpec().Value

						cfg := &clientconfig.Config{
							Context: clusterName,
							Contexts: map[string]*clientconfig.Context{
								clusterName: {
									Endpoints: []string{"127.0.0.1"},
									CA:        spec.Ca,
									Crt:       spec.Crt,
									Key:       spec.Key,
								},
							},
						}
						_, err := client.New(ctx, client.WithConfig(cfg))
						require.NoError(t, err)

						firstCrt = spec.Crt
					},
				)

				// wait 1 second so that the certificate is 50% stale
				time.Sleep(1 * time.Second)

				// issue a refresh tick
				require.NoError(t, st.Create(ctx, system.NewCertRefreshTick("refresh")))

				rtestutils.AssertResource(ctx, t, st, config.Metadata().ID(),
					func(res *omni.TalosConfig, assertions *assert.Assertions) {
						spec := res.TypedSpec().Value

						// cert should be refreshed
						assertions.NotEqual(firstCrt, spec.Crt)
					},
				)

				rtestutils.Destroy[*omni.Cluster](ctx, t, st, []resource.ID{cluster.Metadata().ID()})
				rmock.Destroy[*omni.ClusterSecrets](ctx, t, st, []resource.ID{cluster.Metadata().ID()})
				rtestutils.AssertNoResource[*omni.TalosConfig](ctx, t, st, config.Metadata().ID())
			},
		)
	})

	t.Run("rotate secret", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		t.Cleanup(cancel)

		testutils.WithRuntime(ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				st := testContext.State
				clusterName := "test-rotate-secret"

				cluster := omni.NewCluster(clusterName)
				cluster.TypedSpec().Value.TalosVersion = constants.DefaultTalosVersion
				require.NoError(t, st.Create(ctx, cluster))

				machineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(cluster.Metadata().ID()))

				machineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
				machineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)

				require.NoError(t, st.Create(ctx, machineSet))

				secretsBundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
				require.NoError(t, err)
				data, err := json.Marshal(secretsBundle)
				require.NoError(t, err)

				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data
					res.TypedSpec().Value.ExtraCerts = nil

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosConfig, assertions *assert.Assertions) {
						spec := res.TypedSpec().Value

						assertions.NotEmpty(spec.Ca)
						assertions.Equal(base64.StdEncoding.EncodeToString(secretsBundle.Certs.OS.Crt), spec.Ca)

						cfg := &clientconfig.Config{
							Context: clusterName,
							Contexts: map[string]*clientconfig.Context{
								clusterName: {
									Endpoints: []string{"127.0.0.1"},
									CA:        spec.Ca,
									Crt:       spec.Crt,
									Key:       spec.Key,
								},
							},
						}
						_, err = client.New(ctx, client.WithConfig(cfg))
						require.NoError(t, err)
					},
				)

				newTalosCA, err := talossecrets.NewTalosCA(time.Now())
				require.NoError(t, err)

				oldTalosCA := &talosx509.PEMEncodedCertificateAndKey{
					Crt: secretsBundle.Certs.OS.Crt,
					Key: secretsBundle.Certs.OS.Key,
				}

				// pre-rotate
				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data
					res.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{
						Os: &specs.ClusterSecretsSpec_Certs_CA{
							Crt: newTalosCA.CrtPEM,
							Key: newTalosCA.CrtPEM,
						},
					}

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosConfig, assertions *assert.Assertions) {
						acceptedCAs := []*talosx509.PEMEncodedCertificate{
							{Crt: oldTalosCA.Crt},
							{Crt: newTalosCA.CrtPEM},
						}

						spec := res.TypedSpec().Value

						assertions.NotEmpty(spec.Ca)
						assertions.NotEqual(base64.StdEncoding.EncodeToString(oldTalosCA.Crt), spec.Ca)
						assertions.NotEqual(base64.StdEncoding.EncodeToString(newTalosCA.CrtPEM), spec.Ca)
						assertions.Equal(base64.StdEncoding.EncodeToString(bytes.Join(
							xslices.Map(
								acceptedCAs,
								func(cert *talosx509.PEMEncodedCertificate) []byte {
									return cert.Crt
								},
							),
							nil,
						)), spec.Ca)

						assertions.NoError(checkSignature(oldTalosCA.Crt, spec.Crt))
						assertions.Error(checkSignature(newTalosCA.CrtPEM, spec.Crt))

						cfg := &clientconfig.Config{
							Context: clusterName,
							Contexts: map[string]*clientconfig.Context{
								clusterName: {
									Endpoints: []string{"127.0.0.1"},
									CA:        spec.Ca,
									Crt:       spec.Crt,
									Key:       spec.Key,
								},
							},
						}
						_, err = client.New(ctx, client.WithConfig(cfg))
						require.NoError(t, err)
					},
				)

				secretsBundle.Certs.OS = &talosx509.PEMEncodedCertificateAndKey{
					Crt: newTalosCA.CrtPEM,
					Key: newTalosCA.KeyPEM,
				}
				data, err = json.Marshal(secretsBundle)
				require.NoError(t, err)

				// rotate
				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data
					res.TypedSpec().Value.ExtraCerts = &specs.ClusterSecretsSpec_Certs{
						Os: &specs.ClusterSecretsSpec_Certs_CA{
							Crt: oldTalosCA.Crt,
							Key: oldTalosCA.Crt,
						},
					}

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosConfig, assertions *assert.Assertions) {
						acceptedCAs := []*talosx509.PEMEncodedCertificate{
							{Crt: newTalosCA.CrtPEM},
							{Crt: oldTalosCA.Crt},
						}

						spec := res.TypedSpec().Value

						assertions.NotEmpty(spec.Ca)
						assertions.NotEqual(base64.StdEncoding.EncodeToString(newTalosCA.CrtPEM), spec.Ca)
						assertions.NotEqual(base64.StdEncoding.EncodeToString(oldTalosCA.Crt), spec.Ca)
						assertions.Equal(base64.StdEncoding.EncodeToString(bytes.Join(
							xslices.Map(
								acceptedCAs,
								func(cert *talosx509.PEMEncodedCertificate) []byte {
									return cert.Crt
								},
							),
							nil,
						)), spec.Ca)

						assertions.Error(checkSignature(oldTalosCA.Crt, spec.Crt))
						assertions.NoError(checkSignature(newTalosCA.CrtPEM, spec.Crt))

						cfg := &clientconfig.Config{
							Context: clusterName,
							Contexts: map[string]*clientconfig.Context{
								clusterName: {
									Endpoints: []string{"127.0.0.1"},
									CA:        spec.Ca,
									Crt:       spec.Crt,
									Key:       spec.Key,
								},
							},
						}
						_, err = client.New(ctx, client.WithConfig(cfg))
						assert.NoError(t, err)
					},
				)

				// post-rotate
				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosConfig, assertions *assert.Assertions) {
						acceptedCAs := []*talosx509.PEMEncodedCertificate{
							{Crt: newTalosCA.CrtPEM},
							{Crt: oldTalosCA.Crt},
						}

						spec := res.TypedSpec().Value

						assertions.NotEmpty(spec.Ca)
						assertions.NotEqual(base64.StdEncoding.EncodeToString(newTalosCA.CrtPEM), spec.Ca)
						assertions.NotEqual(base64.StdEncoding.EncodeToString(oldTalosCA.Crt), spec.Ca)

						assertions.Equal(base64.StdEncoding.EncodeToString(bytes.Join(
							xslices.Map(
								acceptedCAs,
								func(cert *talosx509.PEMEncodedCertificate) []byte {
									return cert.Crt
								},
							),
							nil,
						)), spec.Ca)

						assertions.Error(checkSignature(oldTalosCA.Crt, spec.Crt))
						assertions.NoError(checkSignature(newTalosCA.CrtPEM, spec.Crt))

						cfg := &clientconfig.Config{
							Context: clusterName,
							Contexts: map[string]*clientconfig.Context{
								clusterName: {
									Endpoints: []string{"127.0.0.1"},
									CA:        spec.Ca,
									Crt:       spec.Crt,
									Key:       spec.Key,
								},
							},
						}
						_, err = client.New(ctx, client.WithConfig(cfg))
						assert.NoError(t, err)
					},
				)

				// ok
				rmock.Mock[*omni.ClusterSecrets](ctx, t, testContext.State, testoptions.SameID(cluster), testoptions.Modify(func(res *omni.ClusterSecrets) error {
					res.TypedSpec().Value.Data = data
					res.TypedSpec().Value.ExtraCerts = nil

					return nil
				}))

				rtestutils.AssertResource(ctx, t, st, clusterName,
					func(res *omni.TalosConfig, assertions *assert.Assertions) {
						spec := res.TypedSpec().Value

						assertions.NotEmpty(spec.Ca)
						assertions.NotEqual(base64.StdEncoding.EncodeToString(oldTalosCA.Crt), spec.Ca)
						assertions.Equal(base64.StdEncoding.EncodeToString(newTalosCA.CrtPEM), spec.Ca)

						assertions.Error(checkSignature(oldTalosCA.Crt, spec.Crt))
						assertions.NoError(checkSignature(newTalosCA.CrtPEM, spec.Crt))

						cfg := &clientconfig.Config{
							Context: clusterName,
							Contexts: map[string]*clientconfig.Context{
								clusterName: {
									Endpoints: []string{"127.0.0.1"},
									CA:        spec.Ca,
									Crt:       spec.Crt,
									Key:       spec.Key,
								},
							},
						}
						_, err = client.New(ctx, client.WithConfig(cfg))
						assert.NoError(t, err)
					},
				)

				rtestutils.Destroy[*omni.Cluster](ctx, t, st, []resource.ID{clusterName})
				rmock.Destroy[*omni.ClusterSecrets](ctx, t, st, []resource.ID{clusterName})
				rtestutils.AssertNoResource[*omni.TalosConfig](ctx, t, st, clusterName)
			},
		)
	})
}

func checkSignature(caCertBytes []byte, clientCert string) error {
	block, _ := pem.Decode(caCertBytes)

	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	clientCertBytes, err := base64.StdEncoding.DecodeString(clientCert)
	if err != nil {
		return err
	}

	block, _ = pem.Decode(clientCertBytes)

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	return cert.CheckSignatureFrom(ca)
}
