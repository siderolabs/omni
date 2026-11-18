// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/access/role"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
)

// TestManagementStatusCodes checks the status code of the requests which ask for something that does not exist, or which are not valid.
func TestManagementStatusCodes(t *testing.T) {
	const cluster = "missing"

	for _, tt := range []struct {
		call     func(ctx context.Context, server *grpcomni.ManagementServer) error
		name     string
		backend  noBackend
		expected codes.Code
	}{
		{
			name:     "service account kubeconfig of a cluster which does not exist",
			expected: codes.NotFound,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.Kubeconfig(ctx, serviceAccountKubeconfigRequest)

				return err
			},
		},
		{
			name:     "break-glass kubeconfig of a cluster which does not exist",
			backend:  noBackend{configsNotFound: true},
			expected: codes.NotFound,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.Kubeconfig(ctx, &management.KubeconfigRequest{BreakGlass: true})

				return err
			},
		},
		{
			name:     "break-glass kubeconfig of a cluster which has no status yet",
			expected: codes.NotFound,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.Kubeconfig(ctx, &management.KubeconfigRequest{BreakGlass: true})

				return err
			},
		},
		{
			name:     "break-glass talosconfig of a cluster which does not exist",
			backend:  noBackend{configsNotFound: true},
			expected: codes.NotFound,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.Talosconfig(ctx, &management.TalosconfigRequest{BreakGlass: true})

				return err
			},
		},
		{
			name:     "break-glass talosconfig of a cluster which has no status yet",
			expected: codes.NotFound,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.Talosconfig(ctx, &management.TalosconfigRequest{BreakGlass: true})

				return err
			},
		},
		{
			name:     "Kubernetes upgrade pre-checks of a cluster which does not exist",
			expected: codes.NotFound,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.KubernetesUpgradePreChecks(ctx, &management.KubernetesUpgradePreChecksRequest{NewVersion: "1.37.2"})

				return err
			},
		},
		{
			name:     "renewal of a service account which does not exist",
			expected: codes.NotFound,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.RenewServiceAccount(ctx, &management.RenewServiceAccountRequest{Name: "missing", ArmoredPgpPublicKey: "invalid"})

				return err
			},
		},
		{
			name:     "renewal of a service account with a key which cannot be parsed",
			expected: codes.InvalidArgument,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.RenewServiceAccount(ctx, &management.RenewServiceAccountRequest{Name: "existing", ArmoredPgpPublicKey: "invalid"})

				return err
			},
		},
		{
			name:     "service account with a key which cannot be parsed",
			expected: codes.InvalidArgument,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				_, err := server.CreateServiceAccount(ctx, &management.CreateServiceAccountRequest{Name: "new", Role: string(role.Reader), ArmoredPgpPublicKey: "invalid"})

				return err
			},
		},
		{
			name:     "service account with a role which does not fit its name",
			expected: codes.InvalidArgument,
			call: func(ctx context.Context, server *grpcomni.ManagementServer) error {
				// the name of an existing service account, so that the request fails in another way when the role is not checked
				_, err := server.CreateServiceAccount(ctx, &management.CreateServiceAccountRequest{Name: "existing", Role: string(role.InfraProvider), ArmoredPgpPublicKey: "invalid"})

				return err
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := state.WrapCore(inmem.NewState())

			require.NoError(t, st.Create(t.Context(), authres.NewIdentity(access.ParseServiceAccountFromName("existing").FullID())))

			ctx := managementPowerTestContext(t.Context(), "admin@example.org", role.Admin)
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("cluster", cluster))

			server := grpcomni.NewManagementServer(st, nil, zaptest.NewLogger(t), true, tt.backend, tt.backend)

			err := tt.call(ctx, server)

			require.Equal(t, tt.expected, status.Code(err), "error: %v", err)
		})
	}
}

var errNoBackend = errors.New("the test server has no Kubernetes behind it")

// noBackend stands in for Talos and Kubernetes, which the management server does not have in the tests.
type noBackend struct {
	// configsNotFound makes the break-glass configs fail the way they do for a cluster which does not exist.
	configsNotFound bool
}

// breakGlassConfig returns the break-glass config of any cluster, or the error of the state for a missing resource.
func (b noBackend) breakGlassConfig(ctx context.Context) ([]byte, error) {
	if !b.configsNotFound {
		return []byte("config"), nil
	}

	_, err := inmem.NewState().Get(ctx, omni.NewClusterSecrets("missing").Metadata())

	return nil, err
}

func (noBackend) GetKubeconfig(context.Context, *common.Context) (*rest.Config, error) {
	return nil, errNoBackend
}

func (noBackend) GetOIDCKubeconfig(*common.Context, string, ...string) ([]byte, error) {
	return []byte("kubeconfig"), nil
}

func (b noBackend) BreakGlassKubeconfig(ctx context.Context, _ string) ([]byte, error) {
	return b.breakGlassConfig(ctx)
}

func (noBackend) GetClient(context.Context, string) (*kubernetes.Client, error) {
	return nil, errNoBackend
}

func (noBackend) RawTalosconfig(context.Context, string) ([]byte, error) {
	return []byte("talosconfig"), nil
}

func (b noBackend) OperatorTalosconfig(ctx context.Context, _ string) ([]byte, error) {
	return b.breakGlassConfig(ctx)
}

var serviceAccountKubeconfigRequest = &management.KubeconfigRequest{
	ServiceAccount:     true,
	ServiceAccountUser: "test",
	ServiceAccountTtl:  durationpb.New(time.Hour),
}
