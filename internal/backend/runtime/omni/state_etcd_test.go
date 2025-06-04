// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/keystorage"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/keyprovider"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
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
			err := omniruntime.BuildEtcdPersistentState(t.Context(), &config.Params{
				Account: config.Account{
					Name: "instance-name",
				},
				Storage: config.Storage{
					Default: config.StorageDefault{
						Etcd: config.EtcdParams{
							Embedded:         true,
							EmbeddedDBPath:   etcdDir,
							PrivateKeySource: step.args.privateKeySource,
							PublicKeyFiles:   step.args.publicKeyFiles,
							Endpoints:        []string{"http://localhost:0"},
						},
					},
				},
			}, zaptest.NewLogger(t), func(context.Context, namespaced.StateBuilder) error {
				return nil
			})

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

	original := omni.NewCluster(resources.DefaultNamespace, "clusterID")

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
			err := omniruntime.BuildEtcdPersistentState(t.Context(), &config.Params{
				Account: config.Account{
					Name: "instance-name",
				},
				Storage: config.Storage{
					Default: config.StorageDefault{
						Etcd: config.EtcdParams{
							Embedded:         true,
							EmbeddedDBPath:   etcdDir,
							PrivateKeySource: step.args.privateKeySource,
							PublicKeyFiles:   step.args.publicKeyFiles,
							Endpoints:        []string{"http://localhost:0"},
						},
					},
				},
			}, zaptest.NewLogger(t),
				func(ctx context.Context, stateBuilder namespaced.StateBuilder) error {
					coreState := stateBuilder(resources.DefaultNamespace)
					if step.beforeGet != nil {
						step.beforeGet(t, coreState)
					}

					got, err := coreState.Get(ctx, original.Metadata())
					require.NoError(t, err)

					require.True(t, resource.Equal(original, got))

					return nil
				},
			)

			require.NoError(t, err)
		})

		if !res {
			t.FailNow()
		}
	}
}
