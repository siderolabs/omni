// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/md5"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-api-signature/pkg/client/interceptor"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/management"
	resapi "github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access"
	pkgaccess "github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/client"
	managementcli "github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/k8s"
	"github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/infraprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

// testClientFactory creates test clients with specific roles for authorization testing.
// It uses the root client (automation SA) to create new service accounts with the specified role,
// then returns a client authenticated as that SA. Clients are cached by role.
type testClientFactory struct {
	endpoint          string
	serviceAccountKey string
	rootCli           *client.Client

	mu      sync.Mutex
	clients map[role.Role]*client.Client
}

func newTestClientFactory(endpoint string, rootCli *client.Client) *testClientFactory {
	return &testClientFactory{
		endpoint: endpoint,
		rootCli:  rootCli,
		clients:  make(map[role.Role]*client.Client),
	}
}

func (f *testClientFactory) getClient(ctx context.Context, r role.Role) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if cli, ok := f.clients[r]; ok {
		return cli, nil
	}

	cli, err := f.createClientForRole(ctx, r)
	if err != nil {
		return nil, err
	}

	f.clients[r] = cli

	return cli, nil
}

func (f *testClientFactory) createClientForRole(ctx context.Context, r role.Role) (*client.Client, error) {
	name := fmt.Sprintf("%x", md5.Sum([]byte(string(r))))

	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	suffix := access.ServiceAccountNameSuffix
	if r == role.InfraProvider {
		suffix = access.InfraProviderServiceAccountNameSuffix
	}

	serviceAccountEmail := name + suffix

	key, err := pgp.GenerateKey(name, comment, serviceAccountEmail, auth.ServiceAccountMaxAllowedLifetime)
	if err != nil {
		return nil, err
	}

	if r == role.InfraProvider {
		name = access.InfraProviderServiceAccountPrefix + name
	}

	armoredPublicKey, err := key.ArmorPublic()
	if err != nil {
		return nil, err
	}

	serviceAccounts, err := f.rootCli.Management().ListServiceAccounts(ctx)
	if err != nil {
		return nil, err
	}

	if slices.IndexFunc(serviceAccounts, func(account *management.ListServiceAccountsResponse_ServiceAccount) bool {
		return account.Name == name
	}) != -1 {
		if err = f.rootCli.Management().DestroyServiceAccount(ctx, name); err != nil {
			return nil, err
		}
	}

	_, err = f.rootCli.Management().CreateServiceAccount(ctx, name, armoredPublicKey, string(r), false)
	if err != nil {
		return nil, err
	}

	encodedKey, err := serviceaccount.Encode(name, key)
	if err != nil {
		return nil, err
	}

	return client.New(f.endpoint, client.WithServiceAccount(encodedKey))
}

func (f *testClientFactory) close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var errs []error

	for _, cli := range f.clients {
		if err := cli.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// AssertAnonymousAuthentication tests the authentication without any credentials.
func AssertAnonymousAuthentication(testCtx context.Context, client *client.Client) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 300*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, interceptor.SkipInterceptorContextKey{}, struct{}{})

		_, err := client.Omni().State().List(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, "", resource.VersionUndefined))
		assert.Error(t, err)

		assert.Equalf(t, codes.Unauthenticated, status.Code(err), "%s != %s", codes.Unauthenticated, status.Code(err))
	}
}

// AssertUnauthenticatedLocalResourceServerAccess tests the authentication without any credentials.
func AssertUnauthenticatedLocalResourceServerAccess(testCtx context.Context) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 300*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, interceptor.SkipInterceptorContextKey{}, struct{}{})

		port := "8081" // todo: get this from the test args or embedded omni config

		client, err := client.New("http://" + net.JoinHostPort("127.0.0.1", port))
		require.NoError(t, err)

		defer client.Close() //nolint:errcheck

		_, err = client.Omni().State().List(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, "", resource.VersionUndefined))
		assert.NoError(t, err)

		_, err = client.Omni().State().List(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.MachineStatusType, "", resource.VersionUndefined))
		assert.NoError(t, err)
	}
}

// AssertAPIInvalidSignature tests the authentication with invalid credentials.
func AssertAPIInvalidSignature(testCtx context.Context, client *client.Client) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 300*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, interceptor.SkipInterceptorContextKey{}, struct{}{})

		ctx = metadata.AppendToOutgoingContext(ctx, "x-sidero-signature", "invalid")

		_, err := client.Omni().State().List(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, "", resource.VersionUndefined))
		assert.Error(t, err)

		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	}
}

// AssertPublicKeyWithoutLifetimeNotRegistered tests the registration of a public key without a lifetime.
func AssertPublicKeyWithoutLifetimeNotRegistered(testCtx context.Context, cli *client.Client) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, interceptor.SkipInterceptorContextKey{}, struct{}{})

		email := "test-user-invalid@siderolabs.com"

		key, err := pgpcrypto.GenerateKey("", email, "x25519", 0)
		require.NoError(t, err)

		armored, err := key.GetArmoredPublicKey()
		require.NoError(t, err)

		_, err = cli.Auth().RegisterPGPPublicKey(ctx, email, []byte(armored))
		assert.ErrorContains(t, err, "key does not contain a valid key lifetime")
	}
}

// AssertPublicKeyWithLongLifetimeNotRegistered tests the registration of a public key
// with a longer than maximum allowed lifetime.
func AssertPublicKeyWithLongLifetimeNotRegistered(testCtx context.Context, cli *client.Client) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, interceptor.SkipInterceptorContextKey{}, struct{}{})

		email := "test-user-invalid@siderolabs.com"

		key, err := pgp.GenerateKey("", "", email, 9*time.Hour)
		require.NoError(t, err)

		armored, err := key.ArmorPublic()
		require.NoError(t, err)

		_, err = cli.Auth().RegisterPGPPublicKey(ctx, email, []byte(armored))
		assert.ErrorContains(t, err, "key lifetime is too long")
	}
}

// AssertRegisterPublicKeyWithUnknownEmail tests the registration of a public key with an unknown email.
// It should not fail explicitly to avoid leaking information about registered emails.
func AssertRegisterPublicKeyWithUnknownEmail(testCtx context.Context, cli *client.Client) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, interceptor.SkipInterceptorContextKey{}, struct{}{})

		email := "test-user-unknown@siderolabs.com"

		key, err := pgp.GenerateKey("", "", email, 4*time.Hour)
		require.NoError(t, err)

		armored, err := key.ArmorPublic()
		require.NoError(t, err)

		_, err = cli.Auth().RegisterPGPPublicKey(ctx, email, []byte(armored))

		// an explicit error must not be returned to avoid leaking information
		assert.NoError(t, err)
	}
}

// AssertServiceAccountAPIFlow tests the service account lifecycle and API calls using it.
func AssertServiceAccountAPIFlow(testCtx context.Context, cli *client.Client) TestFunc {
	return func(t *testing.T) {
		name := "test-" + uuid.NewString()

		saCli, armoredPublicKey, err := newServiceAccountClient(cli, name)
		require.NoError(t, err)

		defer saCli.Close() //nolint:errcheck

		// create service account with the generated key
		_, err = cli.Management().CreateServiceAccount(testCtx, name, armoredPublicKey, string(role.None), true)
		assert.NoError(t, err)

		// make an API call using the registered service account
		_, err = saCli.Omni().State().List(testCtx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, "", resource.VersionUndefined))
		assert.NoError(t, err)

		// renew service account
		renewedSACli, renewedArmoredPublicKey, err := newServiceAccountClient(cli, name)
		require.NoError(t, err)

		defer renewedSACli.Close() //nolint:errcheck

		_, err = cli.Management().RenewServiceAccount(testCtx, name, renewedArmoredPublicKey)
		assert.NoError(t, err)

		// make an API call using the renewed service account
		_, err = renewedSACli.Omni().State().List(testCtx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, "", resource.VersionUndefined))
		assert.NoError(t, err)

		rtestutils.AssertResources(testCtx, t, cli.Omni().State(), []string{
			name + pkgaccess.ServiceAccountNameSuffix,
		}, func(res *authres.ServiceAccountStatus, assert *assert.Assertions) {
			assert.Equal(string(role.Admin), res.TypedSpec().Value.Role)
			assert.Equal(2, len(res.TypedSpec().Value.PublicKeys))
		})

		// list service accounts and ensure that the created service account is present
		saList, err := cli.Management().ListServiceAccounts(testCtx)
		assert.NoError(t, err)

		filtered := xslices.Filter(saList, func(sa *management.ListServiceAccountsResponse_ServiceAccount) bool {
			return sa.Name == name
		})

		assert.Len(t, filtered, 1, "service account not found")

		// assert service account properties
		foundSA := filtered[0]

		// expect 2 keys: 1 from the creation and 1 from renewal
		require.Len(t, foundSA.PgpPublicKeys, 2, "unexpected number of PGP public keys")

		assertTime := func(t *testing.T, expected, actual time.Time, allowedSkew time.Duration) bool {
			return assert.True(t, actual.After(expected.Add(-allowedSkew)) && actual.Before(expected.Add(allowedSkew)))
		}

		for _, pgpPublicKey := range foundSA.PgpPublicKeys {
			assertTime(t, pgpPublicKey.Expiration.AsTime(), time.Now().Add(auth.ServiceAccountMaxAllowedLifetime), 1*time.Minute)
		}

		assert.Equal(t, string(role.Admin), foundSA.Role)

		// try to create a service account with the same name again
		_, err = cli.Management().CreateServiceAccount(testCtx, name, armoredPublicKey, string(role.None), true)
		assert.Equal(t, codes.AlreadyExists, status.Code(err), "error code should be codes.AlreadyExists")
		assert.ErrorContains(t, err, "service account")
		assert.ErrorContains(t, err, "already exists")

		// destroy service account
		err = cli.Management().DestroyServiceAccount(testCtx, name)
		assert.NoError(t, err)

		// list service accounts and ensure that the deleted service account is no more present
		saList, err = cli.Management().ListServiceAccounts(testCtx)
		assert.NoError(t, err)

		assert.False(t, slices.ContainsFunc(saList, func(sa *management.ListServiceAccountsResponse_ServiceAccount) bool {
			return sa.Name == name
		}))
	}
}

func newServiceAccountClient(cli *client.Client, name string) (*client.Client, string, error) {
	// generate a new PGP key with long lifetime
	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	serviceAccountEmail := name + pkgaccess.ServiceAccountNameSuffix

	key, err := pgp.GenerateKey(name, comment, serviceAccountEmail, auth.ServiceAccountMaxAllowedLifetime)
	if err != nil {
		return nil, "", err
	}

	armoredPublicKey, err := key.ArmorPublic()
	if err != nil {
		return nil, "", err
	}

	encodedServiceAccount, err := serviceaccount.Encode(name, key)
	if err != nil {
		return nil, "", err
	}

	interceptors := interceptor.New(interceptor.Options{
		ClientName:           "omni-test",
		ServiceAccountBase64: encodedServiceAccount,
	})

	// create a new API client with the service account PGP signing interceptors
	saCli, err := client.New(
		cli.Endpoint(),
		client.WithGrpcOpts(
			grpc.WithUnaryInterceptor(interceptors.Unary()),
			grpc.WithStreamInterceptor(interceptors.Stream()),
		),
	)
	if err != nil {
		return nil, "", err
	}

	return saCli, armoredPublicKey, nil
}

type apiAuthzTestCase struct {
	namePrefix    string
	fn            func(context.Context, *client.Client) error
	assertSuccess func(*testing.T, error)
	assertFailure func(*testing.T, error)
	requiredRole  role.Role
	isPublic      bool
}

// AssertAPIAuthz tests the authorization checks of the API endpoints.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func AssertAPIAuthz(rootCtx context.Context, rootCli *client.Client, clientFactory *testClientFactory, clusterName string) TestFunc {
	rootCtx = metadata.NewOutgoingContext(rootCtx, metadata.Pairs(grpcutil.LogLevelOverrideMetadataKey, zapcore.PanicLevel.String()))

	assertSuccess := func(t *testing.T, err error) {
		expectedConditions := err == nil ||
			state.IsNotFoundError(err) ||
			state.IsConflictError(err) ||
			validated.IsValidationError(err)

		assert.Truef(t, expectedConditions, "unexpected error: %v", err)
	}

	assertMissingRoleFailure := func(t *testing.T, err error) {
		correctErrorType := status.Code(err) == codes.PermissionDenied || state.IsOwnerConflictError(err)

		assert.Truef(t, correctErrorType, "unexpected error: %v", err)
		assert.ErrorContainsf(t, err, "insufficient role", "unexpected error: %v", err)
	}

	return func(t *testing.T) {
		testCases := []apiAuthzTestCase{
			// Management API tests - global
			{
				namePrefix:    "mgmt-talosconfig",
				requiredRole:  role.Reader,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().Talosconfig(ctx)

					return err
				},
			},
			{
				namePrefix:    "mgmt-create-service-account",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().CreateServiceAccount(ctx, "doesntmatter", "doesntmatter", string(role.None), true)

					// ignore the armored pgp key parse error
					if err != nil && strings.Contains(err.Error(), "no armored data found") {
						return nil
					}

					return err
				},
			},
			{
				namePrefix:    "mgmt-renew-service-account",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().RenewServiceAccount(ctx, "doesntmatter", "doesntmatter")

					// ignore "identity not found" error
					if err != nil && strings.Contains(err.Error(), "doesn't exist") {
						return nil
					}

					return err
				},
			},
			{
				namePrefix:    "mgmt-list-service-accounts",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().ListServiceAccounts(ctx)

					return err
				},
			},
			{
				namePrefix:    "mgmt-destroy-service-account",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					err := cli.Management().DestroyServiceAccount(ctx, "doesntmatter")

					// ignore "service account not found" error
					if status.Code(err) == codes.NotFound {
						return nil
					}

					return err
				},
			},
			{
				namePrefix:    "mgmt-logs",
				requiredRole:  role.Reader,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					machineIDs := rtestutils.ResourceIDs[*omni.Machine](rootCtx, t, rootCli.Omni().State())

					require.NotEmpty(t, machineIDs)

					randomMachineID := machineIDs[0]

					reader, err := cli.Management().LogsReader(ctx, randomMachineID, false, 0)
					if err != nil {
						return err
					}

					buffer := make([]byte, 1)
					_, err = reader.Read(buffer)

					if status.Code(err) != codes.NotFound && !errors.Is(err, io.EOF) {
						return err
					}

					return nil
				},
			},
			// Management API tests - cluster-specific
			{
				namePrefix:    "mgmt-cluster-kubeconfig-user",
				requiredRole:  role.Reader,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().WithCluster(clusterName).Kubeconfig(ctx)

					return err
				},
			},
			{
				namePrefix:    "mgmt-cluster-kubeconfig-service-account",
				requiredRole:  role.Operator,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().WithCluster(clusterName).Kubeconfig(
						ctx,
						managementcli.WithServiceAccount(24*time.Hour, "authz-integration-test", constants.DefaultAccessGroup),
					)

					return err
				},
			},
			{
				namePrefix:    "mgmt-cluster-talosconfig",
				requiredRole:  role.Reader,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().WithCluster(clusterName).Talosconfig(ctx)

					return err
				},
			},
			{
				namePrefix:    "talos-version",
				requiredRole:  role.Reader,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Talos().WithCluster(clusterName).Version(ctx, &emptypb.Empty{})

					return err
				},
			},
			{
				namePrefix:    "talos-etcd-status",
				requiredRole:  role.Reader,
				assertSuccess: assertSuccess,
				assertFailure: func(t *testing.T, err error) {
					assert.Truef(t, status.Code(err) == codes.PermissionDenied, "unexpected error: %v", err)
				},
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Talos().WithCluster(clusterName).EtcdStatus(ctx, &emptypb.Empty{})

					return err
				},
			},
			// OIDC API tests
			{
				namePrefix:    "oidc-authenticate",
				requiredRole:  role.None,
				assertSuccess: assertSuccess,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.OIDC().Authenticate(ctx, "test-token")

					// silence the error on 'token not found'
					if errStatus, ok := status.FromError(err); ok {
						if errStatus.Code() == codes.PermissionDenied && errStatus.Message() == "failed to authenticate request: request not found" {
							return nil
						}
					}

					return err
				},
			},
			// Audit log
			{
				namePrefix:    "audit-logs",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					for _, err := range cli.Management().ReadAuditLog(ctx, "", "") {
						if err != nil {
							return err
						}
					}

					return nil
				},
			},
		}

		for _, tc := range testCases {
			// test each test case without signature
			t.Run(fmt.Sprintf("%s-no-signature", tc.namePrefix), func(t *testing.T) {
				// skip signing the request
				ctx := context.WithValue(rootCtx, interceptor.SkipInterceptorContextKey{}, struct{}{})

				testErr := tc.fn(ctx, rootCli)

				// public resources will either succeed or fail with a permission denied if they are read-only resources
				if tc.isPublic {
					expectedCondition := testErr == nil || strings.Contains(testErr.Error(), "only read access is permitted")

					assert.Truef(t, expectedCondition, "error did not meet condition: %v", testErr)

					return
				}

				// protected resources must always fail with unauthenticated

				expectedCondition := testErr != nil && strings.Contains(testErr.Error(), "Unauthenticated")

				assert.Truef(t, expectedCondition, "error did not meet condition: %v", testErr)
			})

			// public resources are not subject to role restrictions
			if tc.isPublic {
				continue
			}

			// test with the role which should succeed
			t.Run(fmt.Sprintf("%s-success", tc.namePrefix), func(t *testing.T) {
				scopedClient, testErr := clientFactory.getClient(rootCtx, tc.requiredRole)
				require.NoError(t, testErr)

				assertCurrentUserRole(rootCtx, t, scopedClient.Omni().State(), tc.requiredRole)

				testErr = tc.fn(rootCtx, scopedClient)

				tc.assertSuccess(t, testErr)
			})

			if tc.assertFailure == nil || tc.requiredRole == role.None {
				continue
			}

			// test with the role which should fail

			var err error

			// one less than the required role
			failureRole, err := tc.requiredRole.Previous()
			require.NoError(t, err)

			t.Run(fmt.Sprintf("%s-failure", tc.namePrefix), func(t *testing.T) {
				scopedClient, testErr := clientFactory.getClient(rootCtx, failureRole)
				require.NoError(t, testErr)

				assertCurrentUserRole(rootCtx, t, scopedClient.Omni().State(), failureRole)

				testErr = tc.fn(rootCtx, scopedClient)

				tc.assertFailure(t, testErr)
			})
		}
	}
}

type resourceAuthzTestCase struct {
	resource              resource.Resource
	allowedVerbSet        map[state.Verb]struct{}
	isAdminOnly           bool
	isSignatureSufficient bool
	isPublic              bool
	isDestroyNotAllowed   bool
}

// AssertResourceAuthz tests the authorization checks of the resources (state).
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func AssertResourceAuthz(rootCtx context.Context, rootCli *client.Client, clientFactory *testClientFactory) TestFunc {
	rootCtx = metadata.NewOutgoingContext(rootCtx, metadata.Pairs(grpcutil.LogLevelOverrideMetadataKey, zapcore.PanicLevel.String()))

	return func(t *testing.T) {
		allRoles := []role.Role{role.None, role.Reader, role.Operator, role.Admin}
		allVerbs := []state.Verb{state.Get, state.List, state.Create, state.Update, state.Destroy}

		allVerbsSet := xslices.ToSet(allVerbs)
		readOnlyVerbSet := xslices.ToSet([]state.Verb{state.Get, state.List})

		// fully client-managed resources

		identity := authres.NewIdentity(uuid.New().String())
		accessPolicy := authres.NewAccessPolicy()
		samlLabelRule := authres.NewSAMLLabelRule(uuid.New().String())
		cluster := omni.NewCluster(uuid.New().String())
		cluster.TypedSpec().Value.TalosVersion = "1.2.2"
		configPatch := omni.NewConfigPatch(uuid.New().String())
		machineLabels := omni.NewMachineLabels(uuid.New().String())
		machineSet := omni.NewMachineSet(uuid.New().String())
		machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
		machineSetNode := omni.NewMachineSetNode(uuid.New().String(), machineSet)
		machineClass := omni.NewMachineClass(uuid.New().String())
		machineRequestSet := omni.NewMachineRequestSet(uuid.New().String())
		infraMachineConfig := omni.NewInfraMachineConfig(uuid.New().String())
		grpcTunnelConfig := siderolink.NewGRPCTunnelConfig(uuid.New().String())
		installationMediaConfig := omni.NewInstallationMediaConfig(uuid.NewString())

		extensionsConfiguration := omni.NewExtensionsConfiguration(uuid.New().String())
		extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

		machineExtensions := omni.NewMachineExtensions(uuid.New().String())
		machineExtensions.Metadata().Labels().Set(omni.LabelCluster, uuid.New().String())

		machineExtensionsStatus := omni.NewMachineExtensionsStatus(uuid.New().String())
		machineExtensionsStatus.Metadata().Labels().Set(omni.LabelCluster, uuid.New().String())

		kernelArgs := omni.NewKernelArgs(uuid.New().String())
		kernelArgsStatus := omni.NewKernelArgsStatus(uuid.New().String())

		joinToken := siderolink.NewJoinToken(uuid.New().String())

		defaultJoinToken, err := safe.StateGetByID[*siderolink.DefaultJoinToken](rootCtx, rootCli.Omni().State(), siderolink.DefaultJoinTokenID)

		require.NoError(t, err)

		importedClusterSecret := omni.NewImportedClusterSecrets(cluster.Metadata().ID())

		testCases := []resourceAuthzTestCase{
			{
				resource:       identity,
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       authres.NewUser(uuid.New().String()),
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       accessPolicy,
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       joinToken,
				allowedVerbSet: xslices.ToSet([]state.Verb{state.Get, state.List, state.Update, state.Destroy}),
				isAdminOnly:    true,
			},
			{
				resource:            defaultJoinToken,
				allowedVerbSet:      allVerbsSet,
				isAdminOnly:         true,
				isDestroyNotAllowed: true,
			},
			{
				resource:       samlLabelRule,
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       omni.NewInfraMachineBMCConfig(uuid.New().String()),
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       cluster,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       configPatch,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       machineLabels,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       machineSet,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       machineSetNode,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       installationMediaConfig,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       omni.NewNodeForceDestroyRequest(uuid.New().String()),
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       machineClass,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       machineRequestSet,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       infraMachineConfig,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       omni.NewEtcdManualBackup(uuid.New().String()),
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       omni.NewEtcdBackupS3Conf(),
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       extensionsConfiguration,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       machineExtensions,
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       machineExtensionsStatus,
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       kernelArgs,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       kernelArgsStatus,
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       importedClusterSecret,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       grpcTunnelConfig,
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       omni.NewRotateTalosCA(cluster.Metadata().ID()),
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       omni.NewRotateKubernetesCA(cluster.Metadata().ID()),
				allowedVerbSet: allVerbsSet,
			},
		}

		// read-only resources

		resourceDefinition, err := meta.NewResourceDefinition(meta.ResourceDefinitionSpec{
			Type: "Tests.cosi.dev",
		})
		require.NoError(t, err)

		testCases = append(testCases, []resourceAuthzTestCase{
			{
				resource:              resourceDefinition,
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              meta.NewNamespace("test", meta.NamespaceSpec{}),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewClusterBootstrapStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterConfigVersion(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterDestroyStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterEndpoint(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterKubernetesNodes(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineIdentity(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachinePendingUpdates(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineConfigPatches(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineTalosVersion(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachine(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineConfigStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineRequestStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterDiagnostics(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterUUID(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterWorkloadProxyStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewKubernetesNodeAuditResult(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewEtcdBackup(uuid.New().String(), time.Now()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewControlPlaneStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewExposedService(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewFeaturesConfig(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewKubernetesStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       virtual.NewKubernetesUsage(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       virtual.NewSBCConfig(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       virtual.NewCloudPlatformConfig(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       virtual.NewMetalPlatformConfig(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       virtual.NewLabelsCompletion(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewKubernetesUpgradeManifestStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewKubernetesUpgradeStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewLoadBalancerConfig(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewLoadBalancerStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachine(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineSetStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineStatusLink(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineStatusSnapshot(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineConfigGenOptions(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewOngoingTask("res"),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              omni.NewKubernetesVersion(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              omni.NewTalosVersion(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewTalosUpgradeStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewInstallationMedia(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewRedactedClusterMachineConfig(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineConfigDiff(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewImagePullRequest(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewImagePullStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       siderolink.NewConnectionParams(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              system.NewSysVersion(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              virtual.NewCurrentUser(),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              virtual.NewAdvertisedEndpoints(),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              virtual.NewClusterPermissions(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              virtual.NewPermissions(),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewEtcdBackupStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewEtcdBackupOverallStatus(),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineSetDestroyStatus(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewEtcdBackupStoreStatus(),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewSchematic(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewTalosExtensions(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewSchematicConfiguration(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewMachineStatusMetrics(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              omni.NewClusterMetrics(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              omni.NewClusterStatusMetrics(uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewClusterTaint(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       system.NewResourceLabels[*omni.MachineStatus](uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       siderolink.NewLinkStatus(siderolink.NewLink(uuid.NewString(), nil)),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineRequestSetStatus(uuid.New().String()),
				allowedVerbSet: allVerbsSet,
			},
			{
				resource:       omni.NewMaintenanceConfigStatus(uuid.NewString()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineUpgradeStatus(uuid.NewString()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewDiscoveryAffiliateDeleteTask(uuid.NewString()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       authres.NewServiceAccountStatus(uuid.NewString()),
				allowedVerbSet: readOnlyVerbSet,
				isAdminOnly:    true,
			},
			{
				resource:       omni.NewInfraProviderCombinedStatus(uuid.NewString()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       siderolink.NewAPIConfig(),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       siderolink.NewJoinTokenStatus(uuid.NewString()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       siderolink.NewNodeUniqueTokenStatus(uuid.NewString()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterSecretsRotationStatus(uuid.NewString()),
				allowedVerbSet: readOnlyVerbSet,
			},
		}...)

		// no access resources
		testCases = append(testCases, []resourceAuthzTestCase{
			{
				resource: oidc.NewJWTPublicKey(uuid.New().String()),
			},
			{
				resource: system.NewDBVersion(uuid.New().String()),
			},
			{
				resource: system.NewCertRefreshTick(uuid.New().String()),
			},
			{
				resource: authres.NewPublicKey(uuid.New().String()),
			},
			{
				resource: omni.NewEtcdAuditResult(uuid.New().String()),
			},
			{
				resource: omni.NewKubeconfig(uuid.New().String()),
			},
			{
				resource: omni.NewTalosConfig(uuid.New().String()),
			},
			{
				resource: siderolink.NewConfig(),
			},
			{
				resource: omni.NewClusterMachineConfig(uuid.New().String()),
			},
			{
				resource: omni.NewClusterSecrets(uuid.New().String()),
			},
			{
				resource: authres.NewSAMLAssertion(uuid.New().String()),
			},
			{
				resource: omni.NewClusterMachineEncryptionKey(uuid.New().String()),
			},
			{
				resource: omni.NewEtcdBackupEncryption(uuid.New().String()),
			},
			{
				resource: omni.NewBackupData(uuid.New().String()),
			},
			{
				resource: siderolink.NewPendingMachineStatus(uuid.NewString()),
			},
			{
				resource: siderolink.NewMachineJoinConfig(uuid.NewString()),
			},
			{
				resource: siderolink.NewJoinTokenUsage(uuid.NewString()),
			},
			{
				resource: siderolink.NewNodeUniqueToken(uuid.NewString()),
			},
			{
				resource: omni.NewClusterMachineSecrets(uuid.NewString()),
			},
			{
				resource: omni.NewSecretRotation(uuid.NewString()),
			},
			{
				resource: omni.NewMachineSetConfigStatus(uuid.NewString()),
			},
		}...)

		// custom resources

		testCases = append(testCases, []resourceAuthzTestCase{
			{
				resource:       siderolink.NewLink(uuid.New().String(), &specs.SiderolinkSpec{}),
				allowedVerbSet: xslices.ToSet([]state.Verb{state.Get, state.List, state.Update, state.Destroy}),
			},
			{
				resource:       siderolink.NewPendingMachine(uuid.New().String(), &specs.SiderolinkSpec{}),
				allowedVerbSet: xslices.ToSet([]state.Verb{state.Get, state.List, state.Update, state.Destroy}),
			},
		}...)

		untestedResourceTypes := xslices.ToSetFunc(registry.Resources, func(rd generic.ResourceWithRD) resource.Type {
			return rd.ResourceDefinition().Type
		})

		// infra provider resources have their custom authz logic, they are unit-tested in their package, exclude them
		untestedResourceTypes = maps.Filter(untestedResourceTypes, func(resourceType resource.Type, _ struct{}) bool {
			return !infraprovider.IsInfraProviderResource(resources.InfraProviderNamespace, resourceType)
		})

		// delete excluded resources from the untested set
		delete(untestedResourceTypes, k8s.KubernetesResourceType)
		delete(untestedResourceTypes, authres.AuthConfigType)

		for _, tc := range testCases {
			for _, testVerb := range allVerbs {
				for _, testRole := range allRoles {
					name := fmt.Sprintf("resource-authz-%s-%s-%s", testRole, tc.resource.Metadata().Type(), verbToString(testVerb))

					// delete the resource type from the untested set
					delete(untestedResourceTypes, tc.resource.Metadata().Type())

					t.Run(name, func(t *testing.T) {
						scopedCli, testErr := clientFactory.getClient(rootCtx, testRole)
						require.NoError(t, testErr)

						// ensure that scopedCli is operating with the correct role
						assertCurrentUserRole(rootCtx, t, scopedCli.Omni().State(), testRole)

						noSignatureCtx := context.WithValue(rootCtx, interceptor.SkipInterceptorContextKey{}, struct{}{})

						accessErr := accessResource(noSignatureCtx, t, rootCli, scopedCli, tc.resource, testVerb)

						if !tc.isPublic {
							assert.ErrorContains(t, accessErr, "invalid signature")

							// refresh the error but with a signature this time
							accessErr = accessResource(rootCtx, t, rootCli, scopedCli, tc.resource, testVerb)
						}

						if len(tc.allowedVerbSet) == 0 {
							assert.ErrorContains(t, accessErr, "no access is permitted")

							return
						}

						isVerbError := accessErr != nil && strings.Contains(accessErr.Error(), "only") && strings.Contains(accessErr.Error(), "access is permitted")
						isRoleError := accessErr != nil && strings.Contains(accessErr.Error(), "insufficient role:")

						// assert the error

						isReader := testRole.Check(role.Reader) == nil
						isOperator := testRole.Check(role.Operator) == nil
						isAdmin := testRole.Check(role.Admin) == nil
						_, verbAllowed := tc.allowedVerbSet[testVerb]
						sufficientRole := true

						if tc.isAdminOnly {
							if !isAdmin {
								sufficientRole = false
							}
						} else {
							if testVerb.Readonly() {
								if !isReader {
									sufficientRole = false
								}
							} else {
								if !isOperator {
									sufficientRole = false
								}
							}
						}

						if tc.isSignatureSufficient || tc.isPublic {
							sufficientRole = true
						}

						switch {
						case !verbAllowed && !sufficientRole:
							// we check for either of these errors because their order is not guaranteed
							assert.Truef(t, isVerbError || isRoleError, "expected verb not allowed or insufficient role error, got: %v", accessErr)
						case !verbAllowed:
							assert.Truef(t, isVerbError, "expected verb not allowed error, got: %v", accessErr)
						case !sufficientRole:
							assert.Truef(t, isRoleError, "expected insufficient role error, got: %v", accessErr)
						default:
							if accessErr != nil {
								toleratedErrors := map[string]string{
									"NotFoundError":                   "doesn't exist",
									"ValidationError":                 "failed to validate",
									"UnsupportedError":                "unsupported resource type",
									"AlreadyExists(DefaultJoinToken)": "resource DefaultJoinTokens.omni.sidero.dev(default/default@1) already exists",
									"AlreadyExists(AccessPolicy)":     "resource AccessPolicies.omni.sidero.dev(default/access-policy@undefined) already exists",
									"VersionConflict(AccessPolicy)":   "failed to update: resource AccessPolicies.omni.sidero.dev(default/access-policy@1) update conflict: expected version",
								}

								isExpectedError := false

								for _, toleratedErrorSubstring := range toleratedErrors {
									if strings.Contains(accessErr.Error(), toleratedErrorSubstring) {
										isExpectedError = true

										break
									}
								}

								assert.Truef(t, isExpectedError, "expected one of: %q, got: %v", maps.Keys(toleratedErrors), accessErr)
							}
						}
					})
				}
			}
		}

		// ensure that all resources are tested
		for untestedResourceType := range untestedResourceTypes {
			assert.Failf(t, "resource-authz", "resource type %s is not tested", untestedResourceType)
		}
	}
}

var (
	//go:embed testdata/requests/config-patch.json
	configPatch []byte

	//go:embed testdata/requests/members-request.json
	membersRequest []byte

	//go:embed testdata/requests/machines-request.json
	machinesRequest []byte

	nodesRequest = []byte(`{"type": "nodes.v1"}`)

	emptyJSON = []byte("{}")

	mockResource = configPatch

	//go:embed testdata/requests/delete-config-patch.json
	deleteConfigPatchRequest []byte

	//go:embed testdata/requests/get-machine-status.json
	getMachineStatusRequest []byte

	//go:embed testdata/requests/get-current-user.json
	getCurrentUserRequest []byte
)

const grpcMetadataPrefix = "Grpc-Metadata-"

func AssertFrontendResourceAPI(ctx context.Context, rootCli *client.Client, serviceAccountKey, httpEndpoint, clusterName string) TestFunc {
	return func(t *testing.T) {
		sa, err := serviceaccount.Decode(serviceAccountKey)
		require.NoError(t, err)

		key := sa.Key
		email := sa.Name + access.ServiceAccountNameSuffix

		// do the same flow for the signature as in the JS code
		signRequest := func(request *http.Request) error {
			request.Header.Set(grpcMetadataPrefix+message.TimestampHeaderKey, strconv.FormatInt(time.Now().Unix(), 10))

			md := metadata.MD{}

			for key, values := range request.Header {
				md.Set(strings.ToLower(key)[len(grpcMetadataPrefix):], values...)
			}

			payload := message.GRPCPayload{
				Headers: md,
				Method:  request.URL.Path[len("/api"):],
			}

			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("failed to encode payload: %w", err)
			}

			request.Header.Set(grpcMetadataPrefix+message.PayloadHeaderKey, string(payloadJSON))

			signature, err := key.Sign(payloadJSON)
			if err != nil {
				return fmt.Errorf("failed to sign: %w", err)
			}

			signatureBase64 := base64.StdEncoding.EncodeToString(signature)

			//nolint:canonicalheader
			request.Header.Set(grpcMetadataPrefix+message.SignatureHeaderKey,
				fmt.Sprintf("%s %s %s %s", message.SignatureVersionV1, email, key.Fingerprint(), signatureBase64))

			return nil
		}

		nodes, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, rootCli.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		require.NoError(t, err)

		require.Greater(t, nodes.Len(), 0)

		hostname, ok := nodes.Get(0).Metadata().Labels().Get(omni.LabelHostname)

		require.True(t, ok)

		getNodeRequest, err := json.Marshal(map[string]string{
			"type": "nodes.v1",
			"id":   hostname,
		})

		require.NoError(t, err)

		for _, tt := range []struct {
			requestBody    []byte
			method         string
			expectedStatus int
			backend        common.Runtime
		}{
			{
				backend:        common.Runtime_Kubernetes,
				method:         resapi.ResourceService_Create_FullMethodName,
				requestBody:    emptyJSON,
				expectedStatus: http.StatusBadRequest,
			},
			{
				backend:        common.Runtime_Kubernetes,
				method:         resapi.ResourceService_Create_FullMethodName,
				requestBody:    mockResource,
				expectedStatus: http.StatusNotImplemented,
			},
			{
				backend:        common.Runtime_Talos,
				method:         resapi.ResourceService_Create_FullMethodName,
				requestBody:    emptyJSON,
				expectedStatus: http.StatusBadRequest,
			},
			{
				backend:        common.Runtime_Talos,
				method:         resapi.ResourceService_Create_FullMethodName,
				requestBody:    mockResource,
				expectedStatus: http.StatusNotImplemented,
			},
			{
				backend:        common.Runtime_Omni,
				method:         resapi.ResourceService_Create_FullMethodName,
				requestBody:    configPatch,
				expectedStatus: http.StatusBadRequest,
			},
			{
				backend:        common.Runtime_Kubernetes,
				method:         resapi.ResourceService_Watch_FullMethodName,
				requestBody:    nodesRequest,
				expectedStatus: http.StatusOK,
			},
			{
				backend:        common.Runtime_Talos,
				method:         resapi.ResourceService_Watch_FullMethodName,
				requestBody:    membersRequest,
				expectedStatus: http.StatusOK,
			},
			{
				backend:        common.Runtime_Omni,
				method:         resapi.ResourceService_Watch_FullMethodName,
				requestBody:    machinesRequest,
				expectedStatus: http.StatusOK,
			},
			{
				backend:        common.Runtime_Kubernetes,
				method:         resapi.ResourceService_List_FullMethodName,
				requestBody:    nodesRequest,
				expectedStatus: http.StatusOK,
			},
			{
				backend:        common.Runtime_Talos,
				method:         resapi.ResourceService_List_FullMethodName,
				requestBody:    membersRequest,
				expectedStatus: http.StatusOK,
			},
			{
				backend:        common.Runtime_Omni,
				method:         resapi.ResourceService_List_FullMethodName,
				requestBody:    machinesRequest,
				expectedStatus: http.StatusOK,
			},
			{
				backend:        common.Runtime_Kubernetes,
				method:         resapi.ResourceService_Update_FullMethodName,
				requestBody:    configPatch,
				expectedStatus: http.StatusNotImplemented,
			},
			{
				backend:        common.Runtime_Talos,
				method:         resapi.ResourceService_Update_FullMethodName,
				requestBody:    configPatch,
				expectedStatus: http.StatusNotImplemented,
			},
			{
				backend:        common.Runtime_Omni,
				method:         resapi.ResourceService_Update_FullMethodName,
				requestBody:    configPatch,
				expectedStatus: http.StatusNotFound,
			},
			{
				backend:        common.Runtime_Kubernetes,
				method:         resapi.ResourceService_Delete_FullMethodName,
				requestBody:    deleteConfigPatchRequest,
				expectedStatus: http.StatusNotImplemented,
			},
			{
				backend:        common.Runtime_Talos,
				method:         resapi.ResourceService_Delete_FullMethodName,
				requestBody:    deleteConfigPatchRequest,
				expectedStatus: http.StatusNotImplemented,
			},
			{
				backend:        common.Runtime_Omni,
				method:         resapi.ResourceService_Delete_FullMethodName,
				requestBody:    deleteConfigPatchRequest,
				expectedStatus: http.StatusNotFound,
			},
			{
				backend:        common.Runtime_Kubernetes,
				method:         resapi.ResourceService_Teardown_FullMethodName,
				requestBody:    deleteConfigPatchRequest,
				expectedStatus: http.StatusNotImplemented,
			},
			{
				backend:        common.Runtime_Talos,
				method:         resapi.ResourceService_Teardown_FullMethodName,
				requestBody:    deleteConfigPatchRequest,
				expectedStatus: http.StatusNotImplemented,
			},
			{
				backend:        common.Runtime_Omni,
				method:         resapi.ResourceService_Teardown_FullMethodName,
				requestBody:    deleteConfigPatchRequest,
				expectedStatus: http.StatusNotFound,
			},
			{
				backend:        common.Runtime_Kubernetes,
				method:         resapi.ResourceService_Get_FullMethodName,
				requestBody:    getNodeRequest,
				expectedStatus: http.StatusOK,
			},
			{
				backend:        common.Runtime_Talos,
				method:         resapi.ResourceService_Get_FullMethodName,
				requestBody:    getMachineStatusRequest,
				expectedStatus: http.StatusOK,
			},
			{
				backend:        common.Runtime_Omni,
				method:         resapi.ResourceService_Get_FullMethodName,
				requestBody:    getCurrentUserRequest,
				expectedStatus: http.StatusOK,
			},
		} {
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			for _, sign := range []bool{true, false} {
				nameSuffix := "no_signature"
				if sign {
					nameSuffix = "signed"
				}

				t.Run(tt.method+"_"+tt.backend.String()+"_"+nameSuffix, func(t *testing.T) {
					fullURL, err := url.JoinPath(httpEndpoint, "api", tt.method)
					require.NoError(t, err)

					request, err := http.NewRequestWithContext(ctx, "POST",
						fullURL, bytes.NewBuffer(tt.requestBody),
					)
					require.NoError(t, err)
					request.Header.Set(grpcMetadataPrefix+"Runtime", tt.backend.String())

					switch tt.backend {
					case common.Runtime_Talos:
						request.Header.Set(grpcMetadataPrefix+"Nodes", nodes.Get(0).Metadata().ID())
						request.Header.Set(grpcMetadataPrefix+"Cluster", clusterName)
					case common.Runtime_Kubernetes:
						request.Header.Set(grpcMetadataPrefix+"Cluster", clusterName)
					}

					if sign {
						require.NoError(t, signRequest(request))
					}

					var resp *http.Response

					resp, err = client.Do(request)
					require.NoError(t, err)

					t.Cleanup(func() {
						require.NoError(t, resp.Body.Close())
					})

					decoder := json.NewDecoder(resp.Body)

					var data any

					require.NoError(t, decoder.Decode(&data))

					if !sign {
						require.Equal(t, http.StatusUnauthorized, resp.StatusCode, data)

						return
					}

					require.Equal(t, tt.expectedStatus, resp.StatusCode, data)
				})
			}
		}
	}
}

// AssertResourceAuthzWithACL tests the authorization checks of with ACLs.
func AssertResourceAuthzWithACL(ctx context.Context, rootCli *client.Client) TestFunc {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(grpcutil.LogLevelOverrideMetadataKey, zapcore.PanicLevel.String()))

	return func(t *testing.T) {
		rootState := rootCli.Omni().State()

		testID := "acl-test-" + uuid.NewString()

		saName := "service-account-" + testID

		key := createServiceAccount(ctx, t, rootCli, saName, role.Reader)

		identity := saName + access.ServiceAccountNameSuffix

		t.Cleanup(func() {
			require.NoError(t, rootCli.Management().DestroyServiceAccount(ctx, saName))
		})

		accessPolicy := authres.NewAccessPolicy()

		err := rootState.Destroy(ctx, accessPolicy.Metadata())
		require.Truef(t, err == nil || state.IsNotFoundError(err), "unexpected error: %v", err)

		clusterAuthorizedID := "authorized-" + testID

		accessPolicy.TypedSpec().Value.Rules = []*specs.AccessPolicyRule{
			{
				Users:    []string{identity},
				Clusters: []string{clusterAuthorizedID},
				Role:     string(role.Operator),
			},
		}

		err = rootState.Create(ctx, accessPolicy)
		require.NoError(t, err)

		t.Cleanup(func() { destroy(ctx, t, rootCli, accessPolicy.Metadata()) })

		userCli, err := client.New(rootCli.Endpoint(), client.WithServiceAccount(key))
		require.NoError(t, err)

		t.Cleanup(func() { userCli.Close() }) //nolint:errcheck

		clusterUnauthorized := omni.NewCluster("unauthorized-" + testID)
		clusterUnauthorized.TypedSpec().Value.TalosVersion = constants.DefaultTalosVersion
		clusterUnauthorized.TypedSpec().Value.KubernetesVersion = "1.32.7"

		userState := userCli.Omni().State()

		// try to create an unauthorized cluster using the user client
		err = userState.Create(ctx, clusterUnauthorized)
		assert.ErrorContains(t, err, "insufficient role")

		// create it using the admin client
		require.NoError(t, rootState.Create(ctx, clusterUnauthorized))

		t.Cleanup(func() { destroy(ctx, t, rootCli, clusterUnauthorized.Metadata()) })

		// create a cluster that is authorized to the user by the ACL
		clusterAuthorized := omni.NewCluster(clusterAuthorizedID)
		clusterAuthorized.TypedSpec().Value.TalosVersion = constants.DefaultTalosVersion
		clusterAuthorized.TypedSpec().Value.KubernetesVersion = "1.32.7"

		err = userState.Create(ctx, clusterAuthorized)
		require.NoError(t, err)

		t.Cleanup(func() { destroy(ctx, t, rootCli, clusterAuthorized.Metadata()) })

		// try to get the unauthorized cluster using the user client - should work, as the user has the Reader role
		_, err = userState.Get(ctx, clusterUnauthorized.Metadata())
		require.NoError(t, err)

		//  try to modify the unauthorized cluster using the user client
		clusterUnauthorized.TypedSpec().Value.TalosVersion = "1.4.5"

		err = userState.Update(ctx, clusterUnauthorized)
		assert.ErrorContains(t, err, "insufficient role")

		// try to get the authorized cluster using the user client
		_, err = userState.Get(ctx, clusterAuthorized.Metadata())
		require.NoError(t, err)

		// test the logic for a config patch without any cluster association
		configPatchUnauthorized := omni.NewConfigPatch("unauthorized-" + testID)

		err = userState.Create(ctx, configPatchUnauthorized)
		assert.ErrorContains(t, err, "insufficient role")

		// test the logic for a config patch with an authorized cluster association
		configPatchAuthorized := omni.NewConfigPatch("authorized" + testID)
		configPatchAuthorized.Metadata().Labels().Set(omni.LabelCluster, clusterAuthorized.Metadata().ID())

		err = configPatchAuthorized.TypedSpec().Value.SetUncompressedData([]byte("debug: true"))
		require.NoError(t, err)

		err = userState.Create(ctx, configPatchAuthorized)
		require.NoError(t, err)

		t.Cleanup(func() { destroy(ctx, t, rootCli, configPatchAuthorized.Metadata()) })
	}
}

func accessResource(ctx context.Context, t *testing.T, rootCli *client.Client, cli *client.Client, res resource.Resource, verb state.Verb) error {
	var err error

	md := res.Metadata()

	switch verb { //nolint:exhaustive
	case state.List:
		_, err = cli.Omni().State().List(ctx, md)
	case state.Get:
		_, err = cli.Omni().State().Get(ctx, md)
	case state.Destroy:
		err = cli.Omni().State().Destroy(ctx, md)
	case state.Create:
		err = cli.Omni().State().Create(ctx, res)
		if err == nil {
			defer destroy(ctx, t, rootCli, md)
		}
	case state.Update:
		err = cli.Omni().State().Update(ctx, res)
	default:
		assert.Fail(t, "unsupported test verb: %s", verbToString(verb))
	}

	return err
}

func assertCurrentUserRole(ctx context.Context, t *testing.T, st state.State, expected role.Role) {
	currentUser, currentUserErr := safe.StateGet[*virtual.CurrentUser](ctx, st, virtual.NewCurrentUser().Metadata())
	require.NoError(t, currentUserErr)

	assert.Equal(t, string(expected), currentUser.TypedSpec().Value.GetRole(), "invalid role on current user virtual resource: %v", currentUser.TypedSpec().Value.GetRole())
}

func destroy(ctx context.Context, t *testing.T, omniClient *client.Client, md *resource.Metadata) {
	t.Logf("destroying created resource %s", md.String())

	_, err := omniClient.Omni().State().Teardown(ctx, md)
	if state.IsNotFoundError(err) {
		return
	}

	require.NoError(t, err)

	err = retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(ctx, func(ctx context.Context) error {
		err = omniClient.Omni().State().Destroy(ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return retry.ExpectedError(err)
		}

		return nil
	})
	require.NoError(t, err)
}

func verbToString(verb state.Verb) string {
	switch verb {
	case state.List:
		return "list"
	case state.Get:
		return "get"
	case state.Create:
		return "create"
	case state.Update:
		return "update"
	case state.Destroy:
		return "destroy"
	case state.Watch:
		return "watch"
	default:
		return "unknown"
	}
}
