// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strings"
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
	authcli "github.com/siderolabs/go-api-signature/pkg/client/auth"
	"github.com/siderolabs/go-api-signature/pkg/client/interceptor"
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

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	pkgaccess "github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/client"
	managementcli "github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources/k8s"
	"github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/cmd/integration-test/pkg/clientconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
)

// AssertAnonymousAuthenication tests the authentication without any credentials.
func AssertAnonymousAuthenication(testCtx context.Context, client *client.Client) TestFunc {
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
func AssertAPIAuthz(rootCtx context.Context, rootCli *client.Client, clientConfig *clientconfig.ClientConfig, clusterName string) TestFunc {
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
				requiredRole:  role.Operator,
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
		}

		for _, tc := range testCases {
			// test each test case without signature
			t.Run(fmt.Sprintf("%s-no-signature", tc.namePrefix), func(t *testing.T) {
				scopedClient, testErr := clientConfig.GetClient()
				require.NoError(t, testErr)

				// skip signing the request
				ctx := context.WithValue(rootCtx, interceptor.SkipInterceptorContextKey{}, struct{}{})

				testErr = tc.fn(ctx, scopedClient)

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
				scopedClient, testErr := clientConfig.GetClient(
					authcli.WithRole(string(tc.requiredRole)),
					authcli.WithSkipUserRole(true),
				)
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
				scopedClient, testErr := clientConfig.GetClient(
					authcli.WithRole(string(failureRole)),
					authcli.WithSkipUserRole(true))
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
}

// AssertResourceAuthz tests the authorization checks of the resources (state).
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func AssertResourceAuthz(rootCtx context.Context, rootCli *client.Client, clientConfig *clientconfig.ClientConfig) TestFunc {
	return func(t *testing.T) {
		rootCtx = metadata.NewOutgoingContext(rootCtx, metadata.Pairs(grpcutil.LogLevelOverrideMetadataKey, zapcore.PanicLevel.String()))

		allRoles := []role.Role{role.None, role.Reader, role.Operator, role.Admin}
		allVerbs := []state.Verb{state.Get, state.List, state.Create, state.Update, state.Destroy}

		allVerbsSet := xslices.ToSet(allVerbs)
		readOnlyVerbSet := xslices.ToSet([]state.Verb{state.Get, state.List})

		// fully client-managed resources

		identity := authres.NewIdentity(resources.DefaultNamespace, uuid.New().String())
		accessPolicy := authres.NewAccessPolicy()
		samlLabelRule := authres.NewSAMLLabelRule(resources.DefaultNamespace, uuid.New().String())
		cluster := omni.NewCluster(resources.DefaultNamespace, uuid.New().String())
		cluster.TypedSpec().Value.TalosVersion = "1.2.2"
		configPatch := omni.NewConfigPatch(resources.DefaultNamespace, uuid.New().String())
		machineLabels := omni.NewMachineLabels(resources.DefaultNamespace, uuid.New().String())
		machineSet := omni.NewMachineSet(resources.DefaultNamespace, uuid.New().String())
		machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
		machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, uuid.New().String(), machineSet)
		machineClass := omni.NewMachineClass(resources.DefaultNamespace, uuid.New().String())

		extensionsConfiguration := omni.NewExtensionsConfiguration(resources.DefaultNamespace, uuid.New().String())
		extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

		machineExtensions := omni.NewMachineExtensions(resources.DefaultNamespace, uuid.New().String())
		machineExtensions.Metadata().Labels().Set(omni.LabelCluster, uuid.New().String())

		machineExtensionsStatus := omni.NewMachineExtensionsStatus(resources.DefaultNamespace, uuid.New().String())
		machineExtensionsStatus.Metadata().Labels().Set(omni.LabelCluster, uuid.New().String())

		testCases := []resourceAuthzTestCase{
			{
				resource:       identity,
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       authres.NewUser(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       accessPolicy,
				allowedVerbSet: allVerbsSet,
				isAdminOnly:    true,
			},
			{
				resource:       samlLabelRule,
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
				resource:       machineClass,
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
				resource:       omni.NewClusterBootstrapStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterDestroyStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterEndpoint(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterKubernetesNodes(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineIdentity(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachine(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterMachineTemplate(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterUUID(uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewClusterWorkloadProxyStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewKubernetesNodeAuditResult(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewEtcdBackup(uuid.New().String(), time.Now()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewControlPlaneStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewExposedService(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewFeaturesConfig(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewKubernetesStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       virtual.NewKubernetesUsage(resources.MetricsNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       virtual.NewLabelsCompletion(resources.MetricsNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewKubernetesUpgradeManifestStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewKubernetesUpgradeStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewLoadBalancerConfig(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewLoadBalancerStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachine(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineSetStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineStatusLink(resources.MetricsNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineStatusSnapshot(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineConfigGenOptions(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineClassStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewMachineSetRequiredMachines(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewOngoingTask(resources.DefaultNamespace, "res"),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              omni.NewKubernetesVersion(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              omni.NewTalosVersion(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewTalosUpgradeStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewInstallationMedia(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewRedactedClusterMachineConfig(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewImagePullRequest(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewImagePullStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       authres.NewAuthConfig(),
				allowedVerbSet: readOnlyVerbSet,
				isPublic:       true,
			},
			{
				resource:       siderolink.NewConnectionParams(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              system.NewSysVersion(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              virtual.NewCurrentUser(),
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
				resource:       omni.NewMachineSetDestroyStatus(resources.EphemeralNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewEtcdBackupStoreStatus(),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewSchematic(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewTalosExtensions(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewSchematicConfiguration(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:       omni.NewExtensionsConfigurationStatus(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
			{
				resource:              omni.NewMachineStatusMetrics(resources.EphemeralNamespace, uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:              omni.NewClusterStatusMetrics(resources.EphemeralNamespace, uuid.New().String()),
				allowedVerbSet:        readOnlyVerbSet,
				isSignatureSufficient: true,
			},
			{
				resource:       omni.NewClusterTaint(resources.DefaultNamespace, uuid.New().String()),
				allowedVerbSet: readOnlyVerbSet,
			},
		}...)

		// no access resources
		testCases = append(testCases, []resourceAuthzTestCase{
			{
				resource: omni.NewClusterConfigVersion(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: oidc.NewJWTPublicKey(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: system.NewDBVersion(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: system.NewCertRefreshTick(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: authres.NewPublicKey(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: omni.NewEtcdAuditResult(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: omni.NewKubeconfig(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: omni.NewTalosConfig(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: siderolink.NewConfig(resources.DefaultNamespace),
			},
			{
				resource: omni.NewClusterMachineConfig(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: omni.NewClusterSecrets(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: authres.NewSAMLAssertion(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: omni.NewClusterMachineEncryptionKey(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: omni.NewEtcdBackupEncryption(resources.DefaultNamespace, uuid.New().String()),
			},
			{
				resource: omni.NewBackupData(uuid.New().String()),
			},
		}...)

		// custom resources

		testCases = append(testCases, []resourceAuthzTestCase{
			{
				resource:       siderolink.NewLink(resources.DefaultNamespace, uuid.New().String(), &specs.SiderolinkSpec{}),
				allowedVerbSet: xslices.ToSet([]state.Verb{state.Get, state.List, state.Update, state.Destroy}),
			},
		}...)

		untestedResourceTypes := xslices.ToSetFunc(registry.Resources, func(rd generic.ResourceWithRD) resource.Type {
			return rd.ResourceDefinition().Type
		})

		// delete excluded resources from the untested set
		delete(untestedResourceTypes, k8s.KubernetesResourceType)
		delete(untestedResourceTypes, siderolink.DeprecatedLinkCounterType)

		// cloud provider resources have their custom authz logic, they are unit-tested in their package
		delete(untestedResourceTypes, cloud.MachineRequestType)
		delete(untestedResourceTypes, cloud.MachineRequestStatusType)

		for _, tc := range testCases {
			for _, testVerb := range allVerbs {
				for _, testRole := range allRoles {
					name := fmt.Sprintf("resource-authz-%s-%s-%s", testRole, tc.resource.Metadata().Type(), verbToString(testVerb))

					// delete the resource type from the untested set
					delete(untestedResourceTypes, tc.resource.Metadata().Type())

					t.Run(name, func(t *testing.T) {
						scopedCli, testErr := clientConfig.GetClient(
							authcli.WithRole(string(testRole)),
							authcli.WithSkipUserRole(true),
						)
						require.NoError(t, testErr)

						// ensure that scopedCli is operating with the correct role
						assertCurrentUserRole(rootCtx, t, scopedCli.Omni().State(), testRole)

						noSignatureCtx := context.WithValue(rootCtx, interceptor.SkipInterceptorContextKey{}, struct{}{})

						accessErr := accessResource(noSignatureCtx, t, rootCli, scopedCli, tc.resource, testVerb)

						if len(tc.allowedVerbSet) == 0 {
							assert.ErrorContains(t, accessErr, "no access is permitted")

							return
						}

						if !tc.isPublic {
							assert.ErrorContains(t, accessErr, "missing valid signature")

							// refresh the error but with a signature this time
							accessErr = accessResource(rootCtx, t, rootCli, scopedCli, tc.resource, testVerb)
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
									"NotFoundError":                 "doesn't exist",
									"ValidationError":               "failed to validate",
									"UnsupportedError":              "unsupported resource type",
									"AlreadyExists(AccessPolicy)":   "resource AccessPolicies.omni.sidero.dev(default/access-policy@undefined) already exists",
									"VersionConflict(AccessPolicy)": "failed to update: resource AccessPolicies.omni.sidero.dev(default/access-policy@1) update conflict: expected version",
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

// AssertResourceAuthzWithACL tests the authorization checks of with ACLs.
func AssertResourceAuthzWithACL(ctx context.Context, rootCli *client.Client, clientConfig *clientconfig.ClientConfig) TestFunc {
	return func(t *testing.T) {
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(grpcutil.LogLevelOverrideMetadataKey, zapcore.PanicLevel.String()))

		rootState := rootCli.Omni().State()

		testID := "acl-test-" + uuid.NewString()

		user := authres.NewUser(resources.DefaultNamespace, testID)

		user.TypedSpec().Value.Role = string(role.Reader)

		require.NoError(t, rootState.Create(ctx, user))

		t.Cleanup(func() { destroy(ctx, t, rootCli, user.Metadata()) })

		identity := authres.NewIdentity(resources.DefaultNamespace, fmt.Sprintf("user-%s@siderolabs.com", testID))

		identity.TypedSpec().Value.UserId = user.Metadata().ID()

		require.NoError(t, rootState.Create(ctx, identity))

		t.Cleanup(func() { destroy(ctx, t, rootCli, identity.Metadata()) })

		accessPolicy := authres.NewAccessPolicy()

		err := rootState.Destroy(ctx, accessPolicy.Metadata())
		require.Truef(t, err == nil || state.IsNotFoundError(err), "unexpected error: %v", err)

		clusterAuthorizedID := "authorized-" + testID

		accessPolicy.TypedSpec().Value.Rules = []*specs.AccessPolicyRule{
			{
				Users:    []string{identity.Metadata().ID()},
				Clusters: []string{clusterAuthorizedID},
				Role:     string(role.Operator),
			},
		}

		err = rootState.Create(ctx, accessPolicy)
		require.NoError(t, err)

		t.Cleanup(func() { destroy(ctx, t, rootCli, accessPolicy.Metadata()) })

		userCli, err := clientConfig.GetClientForEmail(identity.Metadata().ID())
		require.NoError(t, err)

		t.Cleanup(func() { userCli.Close() }) //nolint:errcheck

		clusterUnauthorized := omni.NewCluster(resources.DefaultNamespace, "unauthorized-"+testID)
		clusterUnauthorized.TypedSpec().Value.TalosVersion = constants.DefaultTalosVersion
		clusterUnauthorized.TypedSpec().Value.KubernetesVersion = "1.27.3"

		userState := userCli.Omni().State()

		// try to create an unauthorized cluster using the user client
		err = userState.Create(ctx, clusterUnauthorized)
		assert.ErrorContains(t, err, "insufficient role")

		// create it using the admin client
		require.NoError(t, rootState.Create(ctx, clusterUnauthorized))

		t.Cleanup(func() { destroy(ctx, t, rootCli, clusterUnauthorized.Metadata()) })

		// create a cluster that is authorized to the user by the ACL
		clusterAuthorized := omni.NewCluster(resources.DefaultNamespace, clusterAuthorizedID)
		clusterAuthorized.TypedSpec().Value.TalosVersion = constants.DefaultTalosVersion
		clusterAuthorized.TypedSpec().Value.KubernetesVersion = "1.27.3"

		err = userState.Create(ctx, clusterAuthorized)
		assert.NoError(t, err)

		t.Cleanup(func() { destroy(ctx, t, rootCli, clusterAuthorized.Metadata()) })

		// try to get the unauthorized cluster using the user client - should work, as the user has the Reader role
		_, err = userState.Get(ctx, clusterUnauthorized.Metadata())
		assert.NoError(t, err)

		//  try to modify the unauthorized cluster using the user client
		clusterUnauthorized.TypedSpec().Value.TalosVersion = "1.4.5"

		err = userState.Update(ctx, clusterUnauthorized)
		assert.ErrorContains(t, err, "insufficient role")

		// try to get the authorized cluster using the user client
		_, err = userState.Get(ctx, clusterAuthorized.Metadata())
		assert.NoError(t, err)

		// test the logic for a config patch without any cluster association
		configPatchUnauthorized := omni.NewConfigPatch(resources.DefaultNamespace, "unauthorized-"+testID)

		err = userState.Create(ctx, configPatchUnauthorized)
		assert.ErrorContains(t, err, "insufficient role")

		// test the logic for a config patch with an authorized cluster association
		configPatchAuthorized := omni.NewConfigPatch(resources.DefaultNamespace, "authorized"+testID)
		configPatchAuthorized.Metadata().Labels().Set(omni.LabelCluster, clusterAuthorized.Metadata().ID())

		configPatchAuthorized.TypedSpec().Value.Data = "debug: true"

		err = userState.Create(ctx, configPatchAuthorized)
		assert.NoError(t, err)

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

func destroy(ctx context.Context, t *testing.T, rootClient *client.Client, md *resource.Metadata) {
	t.Logf("destroying created resource %s", md.String())

	_, err := rootClient.Omni().State().Teardown(ctx, md)
	if state.IsNotFoundError(err) {
		return
	}

	require.NoError(t, err)

	err = retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(ctx, func(ctx context.Context) error {
		err = rootClient.Omni().State().Destroy(ctx, md)
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
