// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
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
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/client"
	managementcli "github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/integration/authz"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

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

		key, err := pgp.GenerateKey("", "", email, 0)
		require.NoError(t, err)

		armored, err := key.ArmorPublic()
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
		originalKeyID, err := cli.Management().CreateServiceAccount(testCtx, name, armoredPublicKey, string(role.None), true)
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
			name + access.ServiceAccountNameSuffix,
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

		// revoke the original key, leaving only the renewed one
		err = cli.Management().RevokeServiceAccountKey(testCtx, name, originalKeyID)
		require.NoError(t, err)

		rtestutils.AssertResources(testCtx, t, cli.Omni().State(), []string{
			name + access.ServiceAccountNameSuffix,
		}, func(res *authres.ServiceAccountStatus, assert *assert.Assertions) {
			assert.Len(res.TypedSpec().Value.PublicKeys, 1)
		})

		// the original key must not be usable anymore
		_, err = saCli.Omni().State().List(testCtx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, "", resource.VersionUndefined))
		assert.Error(t, err)

		// the renewed key must still be usable
		_, err = renewedSACli.Omni().State().List(testCtx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, "", resource.VersionUndefined))
		assert.NoError(t, err)

		// revoking an unknown key must fail
		err = cli.Management().RevokeServiceAccountKey(testCtx, name, originalKeyID)
		assert.Equal(t, codes.NotFound, status.Code(err))

		// revoking the last key must fail
		saList, err = cli.Management().ListServiceAccounts(testCtx)
		require.NoError(t, err)

		filtered = xslices.Filter(saList, func(sa *management.ListServiceAccountsResponse_ServiceAccount) bool {
			return sa.Name == name
		})
		require.Len(t, filtered, 1, "service account not found")
		require.Len(t, filtered[0].PgpPublicKeys, 1)

		err = cli.Management().RevokeServiceAccountKey(testCtx, name, filtered[0].PgpPublicKeys[0].Id)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))

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

	serviceAccountEmail := name + access.ServiceAccountNameSuffix

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
func AssertAPIAuthz(rootCtx context.Context, rootCli *client.Client, clientFactory *authz.ClientFactory, clusterName string) TestFunc {
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

	// the audit log is gated by exact role rather than by rank, so a denied actor is not "insufficient":
	// an Operator is rejected while outranking the allowed Auditor.
	assertDisallowedRoleFailure := func(t *testing.T, err error) {
		assert.Equalf(t, codes.PermissionDenied, status.Code(err), "unexpected error: %v", err)
	}

	readAuditLog := func(ctx context.Context, cli *client.Client) error {
		for _, err := range cli.Management().ReadAuditLog(ctx, nil) {
			if err != nil {
				return err
			}
		}

		return nil
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

					// the call passes an intentionally invalid armored PGP key, which fails parsing after the
					// authz check with codes.Unknown. Authz failures surface as codes.PermissionDenied, so treat
					// the parse error as success here instead of matching the library error text.
					if status.Code(err) == codes.Unknown {
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
				namePrefix:    "mgmt-revoke-service-account-key",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					err := cli.Management().RevokeServiceAccountKey(ctx, "doesntmatter", "doesntmatter")

					// ignore "identity not found" error
					if status.Code(err) == codes.NotFound {
						return nil
					}

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
				namePrefix:    "mgmt-create-user",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().CreateUser(ctx, "doesntmatter@example.com", string(role.None))

					// ignore "already exists" error from a previous successful run
					if err != nil && strings.Contains(err.Error(), "already exists") {
						return nil
					}

					if err == nil {
						// clean up the created user
						_ = cli.Management().DestroyUser(ctx, "doesntmatter@example.com") //nolint:errcheck
					}

					return err
				},
			},
			{
				namePrefix:    "mgmt-list-users",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					_, err := cli.Management().ListUsers(ctx)

					return err
				},
			},
			{
				namePrefix:    "mgmt-update-user",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					err := cli.Management().UpdateUser(ctx, "doesntmatter@example.com", string(role.None))

					// ignore "not found" error
					if status.Code(err) == codes.NotFound {
						return nil
					}

					return err
				},
			},
			{
				namePrefix:    "mgmt-destroy-user",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertMissingRoleFailure,
				fn: func(ctx context.Context, cli *client.Client) error {
					err := cli.Management().DestroyUser(ctx, "doesntmatter@example.com")

					// ignore "not found" error
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
			// Audit log, allowed for an Admin and denied for the Operator right below it.
			{
				namePrefix:    "audit-logs",
				requiredRole:  role.Admin,
				assertSuccess: assertSuccess,
				assertFailure: assertDisallowedRoleFailure,
				fn:            readAuditLog,
			},
			// The same, for the Auditor the role exists to serve: allowed even though it does not
			// outrank the rejected Operator above, and denied for the Reader right below it.
			{
				namePrefix:    "audit-logs-auditor",
				requiredRole:  role.Auditor,
				assertSuccess: assertSuccess,
				assertFailure: assertDisallowedRoleFailure,
				fn:            readAuditLog,
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
				scopedClient, testErr := clientFactory.Get(rootCtx, tc.requiredRole)
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
				scopedClient, testErr := clientFactory.Get(rootCtx, failureRole)
				require.NoError(t, testErr)

				assertCurrentUserRole(rootCtx, t, scopedClient.Omni().State(), failureRole)

				testErr = tc.fn(rootCtx, scopedClient)

				tc.assertFailure(t, testErr)
			})
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

// AssertSeveralNodesDenied checks that a request naming several nodes is refused by the access check, rather than for
// being unsupported, when the caller may not reach them.
//
//nolint:gocognit,maintidx
func AssertSeveralNodesDenied(testCtx context.Context, rootCli *client.Client, clientFactory *authz.ClientFactory, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute)
		defer cancel()

		machines, err := safe.ReaderListAll[*omni.ClusterMachineStatus](
			ctx, rootCli.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		require.NoError(t, err)
		require.Positive(t, machines.Len())

		existing := machines.Get(0).Metadata().ID()

		cli, err := clientFactory.Get(ctx, role.None)
		require.NoError(t, err)

		conn := cli.Talos().WithCluster(clusterName).WithNodes(existing, existing)

		_, err = resapi.NewResourceServiceClient(conn).Get(ctx, &resapi.GetRequest{
			Namespace: "network",
			Type:      "HostnameStatuses.net.talos.dev",
			Id:        "hostname",
		})

		require.Error(t, err)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
}

// AssertResolutionErrorsAuthz checks that the Talos API answers a caller without access the same way whichever target
// it names, so the answer does not depend on the target.
func AssertResolutionErrorsAuthz(testCtx context.Context, rootCli *client.Client, clientFactory *authz.ClientFactory, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute)
		defer cancel()

		machines, err := safe.ReaderListAll[*omni.ClusterMachineStatus](
			ctx, rootCli.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		require.NoError(t, err)
		require.Positive(t, machines.Len())

		existing := machines.Get(0).Metadata().ID()

		// signed, so the comparison is about access and not about the signature
		cli, err := clientFactory.Get(ctx, role.None)
		require.NoError(t, err)

		t.Run("Node", func(t *testing.T) {
			_, knownErr := cli.Talos().WithCluster(clusterName).WithNodes(existing).Version(ctx, &emptypb.Empty{})
			_, unknownErr := cli.Talos().WithCluster(clusterName).WithNodes(uuid.NewString()).Version(ctx, &emptypb.Empty{})

			require.Error(t, knownErr)
			require.Error(t, unknownErr)
			require.Equal(t, knownErr.Error(), unknownErr.Error())
		})

		t.Run("Cluster", func(t *testing.T) {
			_, knownErr := cli.Talos().WithCluster(clusterName).Version(ctx, &emptypb.Empty{})
			_, unknownErr := cli.Talos().WithCluster("no-such-cluster-"+uuid.NewString()).Version(ctx, &emptypb.Empty{})

			require.Error(t, knownErr)
			require.Error(t, unknownErr)
			require.Equal(t, knownErr.Error(), unknownErr.Error())
		})
	}
}

//nolint:gocognit,maintidx
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

			payloadJSON, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				return fmt.Errorf("failed to encode payload: %w", marshalErr)
			}

			request.Header.Set(grpcMetadataPrefix+message.PayloadHeaderKey, string(payloadJSON))

			signature, signErr := key.Sign(payloadJSON)
			if signErr != nil {
				return fmt.Errorf("failed to sign: %w", signErr)
			}

			signatureBase64 := base64.StdEncoding.EncodeToString(signature)

			request.Header.Set(grpcMetadataPrefix+message.SignatureHeaderKey,
				fmt.Sprintf("%s %s %s %s", message.SignatureVersionV1, email, key.Fingerprint(), signatureBase64))

			return nil
		}

		nodes, err := safe.ReaderListAll[*omni.ClusterMachineStatus](
			ctx, rootCli.Omni().State(),
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
			method         string
			requestBody    []byte
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

					request, err := http.NewRequestWithContext(
						ctx, http.MethodPost,
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
					case common.Runtime_Omni:
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

// AssertUserManagementAPIFlow tests the user lifecycle through the management gRPC endpoints.
// These endpoints pass through ValidatedState(), so role validation applies.
func AssertUserManagementAPIFlow(testCtx context.Context, cli *client.Client) TestFunc {
	return func(t *testing.T) {
		email := fmt.Sprintf("test-user-%s@example.com", uuid.NewString())

		// Create a user with a valid role.
		userID, err := cli.Management().CreateUser(testCtx, email, string(role.Operator))
		require.NoError(t, err)
		require.NotEmpty(t, userID)

		// List users and verify the created user is present with the correct role.
		users, err := cli.Management().ListUsers(testCtx)
		require.NoError(t, err)

		found := slices.ContainsFunc(users, func(u *management.ListUsersResponse_User) bool {
			return u.Email == email && u.Role == string(role.Operator) && u.Id == userID
		})
		assert.True(t, found, "created user not found in list")

		// Update the user's role.
		require.NoError(t, cli.Management().UpdateUser(testCtx, email, string(role.Admin)))

		users, err = cli.Management().ListUsers(testCtx)
		require.NoError(t, err)

		found = slices.ContainsFunc(users, func(u *management.ListUsersResponse_User) bool {
			return u.Email == email && u.Role == string(role.Admin)
		})
		assert.True(t, found, "updated user not found with new role")

		// Create with invalid role should fail.
		_, err = cli.Management().CreateUser(testCtx, "invalid-role@example.com", "SuperAdmin")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unknown role")

		// Update with invalid role should fail.
		err = cli.Management().UpdateUser(testCtx, email, "MegaAdmin")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unknown role")

		// Create duplicate user should fail.
		_, err = cli.Management().CreateUser(testCtx, email, string(role.Reader))
		assert.Error(t, err)
		assert.ErrorContains(t, err, "already exists")

		// Destroy the user.
		require.NoError(t, cli.Management().DestroyUser(testCtx, email))

		// Verify user is gone from the list.
		users, err = cli.Management().ListUsers(testCtx)
		require.NoError(t, err)

		found = slices.ContainsFunc(users, func(u *management.ListUsersResponse_User) bool {
			return u.Email == email
		})
		assert.False(t, found, "destroyed user should not be in list")

		// Destroy non-existent user should fail with NotFound.
		err = cli.Management().DestroyUser(testCtx, "nonexistent@example.com")
		assert.Equal(t, codes.NotFound, status.Code(err))
	}
}
