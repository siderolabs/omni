// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	stateserver "github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omnictl"
)

func TestServiceAccountKeyTTL(t *testing.T) {
	t.Parallel()

	if os.Getenv("OMNI_TEST_SERVICE_ACCOUNT_TTL") == "1" {
		omnictl.RootCmd.SetArgs(os.Args[slices.Index(os.Args, "--")+1:])

		if err := omnictl.Execute(); err != nil {
			os.Exit(1)
		}

		os.Exit(0)
	}

	for _, command := range [][]string{
		{"serviceaccount", "create"},
		{"serviceaccount", "renew"},
		{"infraprovider", "create"},
		{"infraprovider", "renewkey"},
	} {
		for _, ttl := range []string{"", "24h"} {
			t.Run(strings.Join(command, " ")+"/ttl="+ttl, func(t *testing.T) {
				t.Parallel()

				st := state.WrapCore(namespaced.NewState(inmem.Build))
				version := system.NewSysVersion(system.SysVersionID)
				version.TypedSpec().Value.BackendVersion = "1.12.4"
				require.NoError(t, st.Create(t.Context(), version))

				var listenConfig net.ListenConfig

				listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
				require.NoError(t, err)

				server := grpc.NewServer()
				t.Cleanup(server.Stop)
				v1alpha1.RegisterStateServer(server, stateserver.NewState(st))

				keys := make(chan string, 1)
				management.RegisterManagementServiceServer(server, &ttlManagementServer{keys: keys})

				go func() {
					if serveErr := server.Serve(listener); serveErr != nil {
						t.Error(serveErr)
					}
				}()

				configPath := filepath.Join(t.TempDir(), "omniconfig")
				contents := fmt.Sprintf("context: test\ncontexts:\n  test:\n    url: grpc://%s\n", listener.Addr())
				require.NoError(t, os.WriteFile(configPath, []byte(contents), 0o600))

				args := []string{"-test.run=^TestServiceAccountKeyTTL$", "--", "--omniconfig", configPath}
				args = append(args, command...)
				args = append(args, "ttl-repro")

				if ttl != "" {
					args = append(args, "--ttl", ttl)
				}

				ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				t.Cleanup(cancel)

				cmd := exec.CommandContext(ctx, os.Args[0], args...)
				cmd.Env = slices.DeleteFunc(os.Environ(), func(s string) bool {
					return strings.HasPrefix(s, "OMNI_ENDPOINT=") || strings.HasPrefix(s, "OMNI_SERVICE_ACCOUNT_KEY=") || strings.HasPrefix(s, "SIDERO_SERVICE_ACCOUNT_KEY=")
				})
				cmd.Env = append(cmd.Env, "OMNI_TEST_SERVICE_ACCOUNT_TTL=1")

				// stdout contains a private key on success, so only stderr goes into failure messages.
				_, err = cmd.Output()

				if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
					err = fmt.Errorf("%w: %s", err, exitErr.Stderr)
				}

				require.NoError(t, err)

				var armored string
				select {
				case armored = <-keys:
				case <-ctx.Done():
					t.Fatal("command did not send a public key")
				}

				validated, err := access.ValidatePGPPublicKey([]byte(armored), pgp.WithMaxAllowedLifetime(365*24*time.Hour))
				require.NoError(t, err)
				key, err := pgpcrypto.NewKeyFromArmored(armored)
				require.NoError(t, err)

				want := 365 * 24 * time.Hour
				if ttl != "" {
					want = 24 * time.Hour
				}

				actual := validated.Expiration.Sub(key.GetEntity().PrimaryKey.CreationTime)
				t.Logf("public key sent by %s --ttl %q: lifetime=%s", strings.Join(command, " "), ttl, actual)
				require.Equal(t, want, actual)
			})
		}
	}
}

type ttlManagementServer struct {
	management.UnimplementedManagementServiceServer

	keys chan<- string
}

func (s *ttlManagementServer) CreateServiceAccount(_ context.Context, req *management.CreateServiceAccountRequest) (*management.CreateServiceAccountResponse, error) {
	s.keys <- req.ArmoredPgpPublicKey

	return &management.CreateServiceAccountResponse{PublicKeyId: "test-key"}, nil
}

func (s *ttlManagementServer) RenewServiceAccount(_ context.Context, req *management.RenewServiceAccountRequest) (*management.RenewServiceAccountResponse, error) {
	s.keys <- req.ArmoredPgpPublicKey

	return &management.RenewServiceAccountResponse{PublicKeyId: "test-key"}, nil
}
