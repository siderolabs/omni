// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"crypto/tls"
	stdx509 "crypto/x509"
	_ "embed"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/keystorage"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/state-etcd/pkg/state/impl/etcd"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap/zaptest"

	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/keyprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
)

//go:embed testdata/pgp/new_key.private
var oldKey string

func ExampleGetIdentityString() {
	idStr, err := keyprovider.GetIdentityString(oldKey)
	if err != nil {
		panic(err)
	}

	fmt.Println(idStr)

	// Output:
	// Another Key <another_key@siderolabs.com> <584aaf056e1a1278c47845008d8ce8435b748200>
}

func TestEtcdInitialization(t *testing.T) {
	tempDir := t.TempDir()
	etcdDir := filepath.Join(tempDir, "etcd")

	t.Cleanup(func() {
		err := os.RemoveAll(tempDir)
		require.NoError(t, err)
	})

	type args struct {
		privateKeySource string
		publicKeyFiles   []string
	}

	steps := []struct { //nolint:govet
		name        string
		args        args
		expectedErr check.Check
	}{
		{
			name: "empty private key source",
			args: args{
				privateKeySource: "",
				publicKeyFiles:   nil,
			},
			expectedErr: check.ErrorContains("private key source is not set"),
		},
		{
			name: "private key without public keys",
			args: args{
				privateKeySource: "file://testdata/pgp/old_key.private",
				publicKeyFiles:   nil,
			},
			expectedErr: check.NoError(),
		},
		{
			name: "incorrect public key file name",
			args: args{
				privateKeySource: "file://testdata/pgp/old_key.private",
				publicKeyFiles:   []string{"testdata/pgp/not_exists.pgp"},
			},
			expectedErr: check.ErrorContains("failed to read public key file"),
		},
		{
			name: "invalid private key slot",
			args: args{
				privateKeySource: "file://testdata/pgp/new_key.private",
				publicKeyFiles:   nil,
			},
			expectedErr: check.ErrorTagIs[keystorage.SlotNotFoundTag](),
		},
		{
			name: "add new public key",
			args: args{
				privateKeySource: "file://testdata/pgp/old_key.private",
				publicKeyFiles:   []string{"testdata/pgp/new_key.public"},
			},
			expectedErr: check.NoError(),
		},
		{
			name: "new public key with old private key should be ok",
			args: args{
				privateKeySource: "file://testdata/pgp/old_key.private",
				publicKeyFiles:   []string{"testdata/pgp/new_key.public"},
			},
			expectedErr: check.NoError(),
		},
		{
			name: "use new private key",
			args: args{
				privateKeySource: "file://testdata/pgp/new_key.private",
				publicKeyFiles:   nil,
			},
			expectedErr: check.NoError(),
		},
	}

	for _, step := range steps {
		res := t.Run(step.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)

			defer cancel()

			state, err := omni.NewEtcdPersistentState(
				ctx,
				&config.Params{
					Account: config.Account{
						Name: new("instance-name"),
					},
					Storage: config.Storage{
						Default: config.StorageDefault{
							Etcd: config.EtcdParams{
								Embedded:         new(true),
								EmbeddedDBPath:   new(etcdDir),
								PrivateKeySource: new(step.args.privateKeySource),
								PublicKeyFiles:   step.args.publicKeyFiles,
								Endpoints:        []string{"http://localhost:0"},
							},
						},
					},
				},
				zaptest.NewLogger(t),
			)

			if state != nil {
				require.NoError(t, state.Close())
			}

			step.expectedErr(t, err)
		})

		if !res {
			t.FailNow()
		}
	}
}

func TestEncryptDecrypt(t *testing.T) {
	tempDir := t.TempDir()
	etcdDir := filepath.Join(tempDir, "etcd")

	t.Cleanup(func() {
		err := os.RemoveAll(tempDir)
		require.NoError(t, err)
	})

	original := omnires.NewCluster("clusterID")

	type args struct {
		privateKeySource string
		publicKeyFiles   []string
	}

	steps := []struct {
		beforeGet func(t *testing.T, state state.CoreState)
		name      string
		args      args
	}{
		{
			name: "encrypt-decrypt using 'new_key' slot",
			args: args{
				privateKeySource: "file://testdata/pgp/new_key.private",
				publicKeyFiles:   []string{"testdata/pgp/old_key.public"},
			},
			beforeGet: func(t *testing.T, state state.CoreState) {
				err := state.Create(t.Context(), original)
				require.NoError(t, err)
			},
		},
		{
			name: "decrypt using 'old_key' slot",
			args: args{
				privateKeySource: "file://testdata/pgp/old_key.private",
				publicKeyFiles:   nil,
			},
		},
	}

	for _, step := range steps {
		res := t.Run(step.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
			defer cancel()

			state, err := omni.NewEtcdPersistentState(
				ctx,
				&config.Params{
					Account: config.Account{
						Name: new("instance-name"),
					},
					Storage: config.Storage{
						Default: config.StorageDefault{
							Etcd: config.EtcdParams{
								Embedded:         new(true),
								EmbeddedDBPath:   new(etcdDir),
								PrivateKeySource: new(step.args.privateKeySource),
								PublicKeyFiles:   step.args.publicKeyFiles,
								Endpoints:        []string{"http://localhost:0"},
							},
						},
					},
				}, zaptest.NewLogger(t),
			)

			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, state.Close())
			})

			if step.beforeGet != nil {
				step.beforeGet(t, state.State)
			}

			got, err := state.State.Get(ctx, original.Metadata())
			require.NoError(t, err)

			require.True(t, resource.Equal(original, got))
		})

		if !res {
			t.FailNow()
		}
	}
}

// TestEmbeddedEtcdInMemory also checks that canceling many watches does not break the new ones, see https://github.com/cosi-project/state-etcd/pull/156.
func TestEmbeddedEtcdInMemory(t *testing.T) {
	st, err := omni.GetEmbeddedEtcdClientWithServer(&config.EtcdParams{
		Embedded:       new(true),
		EmbeddedDBPath: new(t.TempDir()),
	}, zaptest.NewLogger(t))
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, st.Close()) })

	assert.Empty(t, omni.EmbeddedEtcdServer(st).Clients)
	assert.Empty(t, omni.EmbeddedEtcdServer(st).Peers)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	s := state.WrapCore(etcd.NewState(st.Client(), store.ProtobufMarshaler{}))
	res := conformance.NewPathResource("default", "path")

	canceledCtx, cancelWatches := context.WithCancel(ctx)

	for range 50 {
		require.NoError(t, s.Watch(canceledCtx, res.Metadata(), make(chan state.Event)))
	}

	cancelWatches()

	require.NoError(t, s.Create(ctx, res))
	require.NoError(t, s.Destroy(ctx, res.Metadata()))

	watchCh := make(chan state.Event)

	require.NoError(t, s.Watch(ctx, res.Metadata(), watchCh))

	select {
	case ev := <-watchCh:
		require.Equal(t, state.Destroyed, ev.Type)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for the initial event")
	}

	require.NoError(t, s.Create(ctx, res))

	select {
	case ev := <-watchCh:
		require.Equal(t, state.Created, ev.Type)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for the created event")
	}
}

func TestEmbeddedEtcdTLSListener(t *testing.T) {
	ip := x509.IPAddresses([]net.IP{net.ParseIP("127.0.0.1")})

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	serverCert, err := x509.NewKeyPair(ca, x509.ECDSA(true), ip)
	require.NoError(t, err)

	clientCert, err := x509.NewKeyPair(ca, x509.ECDSA(true))
	require.NoError(t, err)

	selfSigned, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true), ip)
	require.NoError(t, err)

	selfSignedCert, err := tls.X509KeyPair(selfSigned.CrtPEM, selfSigned.KeyPEM)
	require.NoError(t, err)

	for _, tt := range []struct { //nolint:govet
		name           string
		cert, key, ca  []byte
		serverCA       []byte
		acceptedClient tls.Certificate
		rejectedClient tls.Certificate
	}{
		{
			name:           "with CA",
			cert:           serverCert.CrtPEM,
			key:            serverCert.KeyPEM,
			ca:             ca.CrtPEM,
			serverCA:       ca.CrtPEM,
			acceptedClient: *clientCert.Certificate,
			rejectedClient: selfSignedCert,
		},
		{
			name:           "self-signed without CA",
			cert:           selfSigned.CrtPEM,
			key:            selfSigned.KeyPEM,
			serverCA:       selfSigned.CrtPEM,
			acceptedClient: selfSignedCert,
			rejectedClient: *clientCert.Certificate,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			params := &config.EtcdParams{
				Embedded:         new(true),
				EmbeddedDBPath:   new(filepath.Join(dir, "etcd")),
				Endpoints:        []string{"http://127.0.0.1:0"},
				EmbeddedCertFile: new(filepath.Join(dir, "server.crt")),
				EmbeddedKeyFile:  new(filepath.Join(dir, "server.key")),
			}

			require.NoError(t, os.WriteFile(params.GetEmbeddedCertFile(), tt.cert, 0o600))
			require.NoError(t, os.WriteFile(params.GetEmbeddedKeyFile(), tt.key, 0o600))

			if tt.ca != nil {
				params.EmbeddedCaFile = new(filepath.Join(dir, "ca.crt"))

				require.NoError(t, os.WriteFile(params.GetEmbeddedCaFile(), tt.ca, 0o600))
			}

			st, err := omni.GetEmbeddedEtcdClientWithServer(params, zaptest.NewLogger(t))
			require.NoError(t, err)

			t.Cleanup(func() { require.NoError(t, st.Close()) })

			listeners := omni.EmbeddedEtcdServer(st).Clients
			require.Len(t, listeners, 1)

			rootCAs := stdx509.NewCertPool()
			rootCAs.AppendCertsFromPEM(tt.serverCA)

			put := func(certs ...tls.Certificate) error {
				cli, cliErr := clientv3.New(clientv3.Config{
					Endpoints:   []string{listeners[0].Addr().String()},
					DialTimeout: 2 * time.Second,
					TLS:         &tls.Config{RootCAs: rootCAs, Certificates: certs},
				})
				require.NoError(t, cliErr)

				defer cli.Close() //nolint:errcheck

				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				defer cancel()

				_, cliErr = cli.Put(ctx, "key", "value")

				return cliErr
			}

			require.NoError(t, put(tt.acceptedClient))
			require.Error(t, put(tt.rejectedClient))
			require.Error(t, put())
		})
	}
}

func TestEmbeddedEtcdTLSChainWithoutCA(t *testing.T) {
	dir := t.TempDir()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	serverCert, err := x509.NewKeyPair(ca, x509.ECDSA(true))
	require.NoError(t, err)

	params := &config.EtcdParams{
		Embedded:         new(true),
		EmbeddedDBPath:   new(filepath.Join(dir, "etcd")),
		EmbeddedCertFile: new(filepath.Join(dir, "server.crt")),
		EmbeddedKeyFile:  new(filepath.Join(dir, "server.key")),
	}

	require.NoError(t, os.WriteFile(params.GetEmbeddedCertFile(), append(serverCert.CrtPEM, ca.CrtPEM...), 0o600))
	require.NoError(t, os.WriteFile(params.GetEmbeddedKeyFile(), serverCert.KeyPEM, 0o600))

	_, err = omni.GetEmbeddedEtcdClientWithServer(params, zaptest.NewLogger(t))
	require.ErrorContains(t, err, "must contain a single certificate when no CA file is set, found 2")
}
