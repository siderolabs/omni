// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos_test

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/cosi-project/runtime/pkg/state/registry"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	"github.com/siderolabs/omni/internal/pkg/testsecrets"
)

// sensitiveType is a sensitive resource type served by the fake Talos node.
const sensitiveType = "ApiCertificates.secrets.talos.dev"

// fakeTalos serves the COSI resource API of a Talos node over a unix socket, recording the Talos roles of each request.
type fakeTalos struct {
	address string
	roles   [][]string
	mu      sync.Mutex
}

func startFakeTalos(ctx context.Context, t *testing.T) *fakeTalos {
	// a short path, as unix socket paths are limited to 108 bytes
	dir, err := os.MkdirTemp("", "talos")
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })

	socketPath := filepath.Join(dir, "socket")

	listener, err := (&net.ListenConfig{}).Listen(ctx, "unix", socketPath)
	require.NoError(t, err)

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	require.NoError(t, registry.NewNamespaceRegistry(st).RegisterDefault(ctx))
	require.NoError(t, registry.NewResourceRegistry(st).RegisterDefault(ctx))

	// a sensitive resource type, as Talos defines the ones holding secrets
	sensitiveDefinition, err := meta.NewResourceDefinition(meta.ResourceDefinitionSpec{
		Type:             sensitiveType,
		DefaultNamespace: "secrets",
		Sensitivity:      meta.Sensitive,
	})
	require.NoError(t, err)
	require.NoError(t, st.Create(ctx, sensitiveDefinition))

	fake := &fakeTalos{address: "unix://" + socketPath}

	record := func(ctx context.Context) {
		md, _ := metadata.FromIncomingContext(ctx)

		fake.mu.Lock()
		defer fake.mu.Unlock()

		fake.roles = append(fake.roles, md.Get(talosconstants.APIAuthzRoleMetadataKey))
	}

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			record(ctx)

			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error { //nolint:contextcheck // a stream carries its own context
			record(ss.Context())

			return handler(srv, ss)
		}),
	)

	v1alpha1.RegisterStateServer(srv, server.NewState(st))

	errCh := make(chan error, 1)

	go func() { errCh <- srv.Serve(listener) }()

	t.Cleanup(func() {
		srv.Stop()

		if err := <-errCh; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			require.NoError(t, err)
		}
	})

	return fake
}

// lastRoles returns the Talos roles of the last request the fake node served.
func (f *fakeTalos) lastRoles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.roles) == 0 {
		return nil
	}

	return f.roles[len(f.roles)-1]
}

// requests returns the number of requests the fake node served.
func (f *fakeTalos) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.roles)
}

// callerContext returns the context of a request signed by a caller with the given Omni role.
func callerContext(ctx context.Context, r role.Role) context.Context {
	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: true})
	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: "user@example.org"})

	return ctxstore.WithValue(ctx, auth.RoleContextKey{Role: r})
}

// TestRuntimeCallerRoles checks that the resource requests made on behalf of a caller are authorized against the
// cluster of their target, and pass no Talos role: the role is the os:reader of the certificate, whatever the caller's
// Omni role, so the sensitive resources stay unreadable.
func TestRuntimeCallerRoles(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		st := testContext.State
		fake := startFakeTalos(ctx, t)

		clientFactory := talos.NewClientFactory(st, zap.NewNop())
		talosRuntime := talos.New(clientFactory, testContext.Logger, "omni", "https://omni.example.org")

		cluster := omni.NewCluster("alpha")
		cluster.TypedSpec().Value.TalosVersion = "1.11.0"
		require.NoError(t, st.Create(ctx, cluster))

		clusterEndpoint := omni.NewClusterEndpoint("alpha")
		clusterEndpoint.TypedSpec().Value.ManagementAddresses = []string{fake.address}
		require.NoError(t, st.Create(ctx, clusterEndpoint))

		clusterStatus := omni.NewClusterStatus("alpha")
		clusterStatus.TypedSpec().Value.Available = true
		require.NoError(t, st.Create(ctx, clusterStatus))

		member := omni.NewMachineStatus("member")
		member.TypedSpec().Value.Cluster = "alpha"
		member.TypedSpec().Value.ManagementAddress = fake.address
		member.TypedSpec().Value.TalosVersion = "v1.11.0"
		require.NoError(t, st.Create(ctx, member))

		createMaintenanceMachine(ctx, t, st, "unallocated")

		listDefinitions := func(ctx context.Context, target runtime.QueryOption) error {
			_, err := talosRuntime.List(ctx,
				target,
				runtime.WithNamespace(meta.NamespaceName),
				runtime.WithResource(meta.ResourceDefinitionType),
			)

			return err
		}

		for _, tt := range []struct {
			target   runtime.QueryOption
			name     string
			role     role.Role
			wantCode codes.Code
		}{
			{name: "reader lists cluster", role: role.Reader, target: runtime.WithContext("alpha")},
			{name: "operator lists cluster", role: role.Operator, target: runtime.WithContext("alpha")},
			{name: "admin lists cluster", role: role.Admin, target: runtime.WithContext("alpha")},
			{name: "reader lists machine", role: role.Reader, target: runtime.WithMachines("member")},
			{name: "admin lists machine", role: role.Admin, target: runtime.WithMachines("member")},
			{name: "caller without a role is denied the cluster", role: role.None, target: runtime.WithContext("alpha"), wantCode: codes.PermissionDenied},
			{name: "caller without a role is denied the machine", role: role.None, target: runtime.WithMachines("member"), wantCode: codes.PermissionDenied},
			{name: "reader is denied a machine outside of a cluster", role: role.Reader, target: runtime.WithMachines("unallocated"), wantCode: codes.PermissionDenied},
		} {
			t.Run(tt.name, func(t *testing.T) {
				requestsBefore := fake.requests()

				err := listDefinitions(callerContext(ctx, tt.role), tt.target)

				if tt.wantCode != codes.OK {
					require.Error(t, err)
					assert.Equal(t, tt.wantCode, status.Code(err))

					// a denied request never reaches Talos
					assert.Equal(t, requestsBefore, fake.requests())

					return
				}

				require.NoError(t, err)
				assert.Empty(t, fake.lastRoles(), "the role comes from the certificate, not the metadata")
			})
		}
	})
}

// TestReaderCredentials checks that the clients for callers authenticate with a certificate which only has the os:reader
// role, so that Talos lets them only read the non-sensitive resources whatever metadata they pass.
func TestReaderCredentials(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clientFactory := talos.NewClientFactory(testContext.State, zap.NewNop())

		_, err := clientFactory.ReaderCertificate(ctx, "alpha")
		require.True(t, talos.IsClientNotReadyError(err))

		secretsData, err := testsecrets.BundleData(nil)
		require.NoError(t, err)

		clusterSecrets := omni.NewClusterSecrets("alpha")
		clusterSecrets.TypedSpec().Value.Data = secretsData
		require.NoError(t, testContext.State.Create(ctx, clusterSecrets))

		encoded, err := clientFactory.ReaderCertificate(ctx, "alpha")
		require.NoError(t, err)

		certPEM, err := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, err)

		block, _ := pem.Decode(certPEM)
		require.NotNil(t, block)

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		assert.Equal(t, []string{string(talosrole.Reader)}, cert.Subject.Organization)
	})
}

// TestReaderClientCache checks that the admin and the reader clients of a machine are cached apart, and that evicting
// the machine drops both.
func TestReaderClientCache(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		fake := startFakeTalos(ctx, t)
		clientFactory := talos.NewClientFactory(testContext.State, zap.NewNop())

		createClusterMachine(ctx, t, testContext.State, "member", "alpha", fake.address)

		admin, err := clientFactory.GetForMachine(ctx, "member")
		require.NoError(t, err)
		require.NoError(t, admin.Close())

		reader, err := clientFactory.GetReaderForMachine(ctx, "member")
		require.NoError(t, err)
		require.NoError(t, reader.Close())

		assert.NotSame(t, admin.Client, reader.Client)
		assert.Equal(t, 2, clientFactory.CacheLen())
		assert.Equal(t, 1, clientFactory.ActiveClients("machine"))
		assert.Equal(t, 1, clientFactory.ActiveClients("machine-reader"))

		clientFactory.ReleaseForMachine("alpha", "member")

		assert.Equal(t, 0, clientFactory.CacheLen())
	})
}

// TestRuntimeSensitivity checks that the sensitive resources stay unreadable on Omni's side, whatever the node enforces.
//
// The maintenance API of Talos grants every SideroLink peer, Omni included, all the roles, so this is the only check
// for a machine in maintenance mode; for the others it backs the os:reader certificate.
func TestRuntimeSensitivity(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		st := testContext.State
		fake := startFakeTalos(ctx, t)

		clientFactory := talos.NewClientFactory(st, zap.NewNop())
		talosRuntime := talos.New(clientFactory, testContext.Logger, "omni", "https://omni.example.org")

		// allocated to the cluster, but still in maintenance mode, so it is reached over the maintenance API
		machineStatus := omni.NewMachineStatus("allocated")
		machineStatus.TypedSpec().Value.Cluster = "alpha"
		machineStatus.TypedSpec().Value.Maintenance = true
		machineStatus.TypedSpec().Value.ManagementAddress = fake.address
		require.NoError(t, st.Create(ctx, machineStatus))

		machine := omni.NewMachine("allocated")
		machine.TypedSpec().Value.Connected = true
		require.NoError(t, st.Create(ctx, machine))

		list := func(ctx context.Context, namespace, resourceType string) error {
			_, err := talosRuntime.List(ctx,
				runtime.WithMachines("allocated"),
				runtime.WithNamespace(namespace),
				runtime.WithResource(resourceType),
			)

			return err
		}

		// the non-sensitive resources are readable as before
		require.NoError(t, list(callerContext(ctx, role.Reader), meta.NamespaceName, meta.ResourceDefinitionType))

		for _, r := range []role.Role{role.Reader, role.Operator, role.Admin} {
			err := list(callerContext(ctx, r), "secrets", sensitiveType)
			require.Error(t, err, "role %s", r)
			assert.Equal(t, codes.PermissionDenied, status.Code(err), "role %s: %v", r, err)
		}

		err := list(callerContext(ctx, role.Admin), "secrets", "Unknowns.secrets.talos.dev")
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}

// createClusterMachine creates the status of a machine allocated to the cluster and reachable over the given address.
//
// The address must be a unix socket, as a cluster client over any other address needs the cluster credentials.
func createClusterMachine(ctx context.Context, t *testing.T, st state.State, id, clusterName, address string) {
	ms := omni.NewMachineStatus(id)
	ms.TypedSpec().Value.ManagementAddress = address
	ms.TypedSpec().Value.Cluster = clusterName
	require.NoError(t, st.Create(ctx, ms))
}

// createMaintenanceMachine creates the status of a machine in maintenance mode. Its client needs no credentials.
func createMaintenanceMachine(ctx context.Context, t *testing.T, st state.State, id string) {
	ms := omni.NewMachineStatus(id)
	ms.TypedSpec().Value.ManagementAddress = "127.0.0.1"
	ms.TypedSpec().Value.Maintenance = true
	require.NoError(t, st.Create(ctx, ms))
}
