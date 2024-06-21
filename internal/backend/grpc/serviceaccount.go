// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	pkgaccess "github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func (s *managementServer) CreateServiceAccount(ctx context.Context, req *management.CreateServiceAccountRequest) (*management.CreateServiceAccountResponse, error) {
	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin)); err != nil {
		return nil, err
	}

	sa := pkgaccess.ParseServiceAccountFromName(req.Name)
	saRole := role.Admin

	if req.UseUserRole && sa.IsCloudProvider {
		return nil, fmt.Errorf("cloud provider service accounts must have the role %q, but use-user-role was requested", role.CloudProvider)
	}

	if !req.UseUserRole {
		var err error

		if saRole, err = role.Parse(req.GetRole()); err != nil {
			return nil, err
		}

		if sa.IsCloudProvider && saRole != role.CloudProvider {
			return nil, fmt.Errorf("cloud-provider service accounts must have the role %q", role.CloudProvider)
		}

		if saRole == role.CloudProvider && !sa.IsCloudProvider {
			return nil, fmt.Errorf("service accounts with role %q must be prefixed with %q", role.CloudProvider, pkgaccess.CloudProviderServiceAccountPrefix)
		}
	}

	id := sa.FullID()

	ctx = actor.MarkContextAsInternalActor(ctx)

	_, err := s.omniState.Get(ctx, authres.NewIdentity(resources.DefaultNamespace, id).Metadata())
	if err == nil {
		return nil, fmt.Errorf("service account %q already exists", id)
	}

	if !state.IsNotFoundError(err) { // the identity must not exist
		return nil, err
	}

	key, err := validatePGPPublicKey(
		[]byte(req.GetArmoredPgpPublicKey()),
		pgp.WithMaxAllowedLifetime(auth.ServiceAccountMaxAllowedLifetime),
	)
	if err != nil {
		return nil, err
	}

	newUserID := uuid.NewString()

	publicKeyResource := authres.NewPublicKey(resources.DefaultNamespace, key.id)
	publicKeyResource.Metadata().Labels().Set(authres.LabelPublicKeyUserID, newUserID)

	if sa.IsCloudProvider {
		publicKeyResource.Metadata().Labels().Set(authres.LabelCloudProvider, "")
	}

	publicKeyResource.TypedSpec().Value.PublicKey = key.data
	publicKeyResource.TypedSpec().Value.Expiration = timestamppb.New(key.expiration)
	publicKeyResource.TypedSpec().Value.Role = string(saRole)

	// register the public key of the service account as "confirmed" because we are already authenticated
	publicKeyResource.TypedSpec().Value.Confirmed = true

	publicKeyResource.TypedSpec().Value.Identity = &specs.Identity{
		Email: id,
	}

	err = s.omniState.Create(ctx, publicKeyResource)
	if err != nil {
		return nil, err
	}

	// create the user resource representing the service account with the same scopes as the public key
	user := authres.NewUser(resources.DefaultNamespace, newUserID)
	user.TypedSpec().Value.Role = publicKeyResource.TypedSpec().Value.GetRole()

	if sa.IsCloudProvider {
		user.Metadata().Labels().Set(authres.LabelCloudProvider, "")
	}

	err = s.omniState.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// create the identity resource representing the service account
	identity := authres.NewIdentity(resources.DefaultNamespace, id)
	identity.TypedSpec().Value.UserId = user.Metadata().ID()
	identity.Metadata().Labels().Set(authres.LabelIdentityUserID, newUserID)
	identity.Metadata().Labels().Set(authres.LabelIdentityTypeServiceAccount, "")

	if sa.IsCloudProvider {
		identity.Metadata().Labels().Set(authres.LabelCloudProvider, "")
	}

	err = s.omniState.Create(ctx, identity)
	if err != nil {
		return nil, err
	}

	return &management.CreateServiceAccountResponse{PublicKeyId: key.id}, nil
}

// RenewServiceAccount registers a new public key to the service account, effectively renewing it.
func (s *managementServer) RenewServiceAccount(ctx context.Context, req *management.RenewServiceAccountRequest) (*management.RenewServiceAccountResponse, error) {
	_, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	sa := pkgaccess.ParseServiceAccountFromName(req.Name)
	id := sa.FullID()

	identity, err := safe.StateGet[*authres.Identity](ctx, s.omniState, authres.NewIdentity(resources.DefaultNamespace, id).Metadata())
	if err != nil {
		return nil, err
	}

	user, err := safe.StateGet[*authres.User](ctx, s.omniState, authres.NewUser(resources.DefaultNamespace, identity.TypedSpec().Value.UserId).Metadata())
	if err != nil {
		return nil, err
	}

	key, err := validatePGPPublicKey(
		[]byte(req.GetArmoredPgpPublicKey()),
		pgp.WithMaxAllowedLifetime(auth.ServiceAccountMaxAllowedLifetime),
	)
	if err != nil {
		return nil, err
	}

	publicKeyResource := authres.NewPublicKey(resources.DefaultNamespace, key.id)
	publicKeyResource.Metadata().Labels().Set(authres.LabelPublicKeyUserID, identity.TypedSpec().Value.UserId)

	publicKeyResource.TypedSpec().Value.PublicKey = key.data
	publicKeyResource.TypedSpec().Value.Expiration = timestamppb.New(key.expiration)
	publicKeyResource.TypedSpec().Value.Role = user.TypedSpec().Value.GetRole()

	publicKeyResource.TypedSpec().Value.Confirmed = true

	publicKeyResource.TypedSpec().Value.Identity = &specs.Identity{
		Email: id,
	}

	err = s.omniState.Create(ctx, publicKeyResource)
	if err != nil {
		return nil, err
	}

	return &management.RenewServiceAccountResponse{PublicKeyId: key.id}, nil
}

func (s *managementServer) ListServiceAccounts(ctx context.Context, _ *emptypb.Empty) (*management.ListServiceAccountsResponse, error) {
	_, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	identityList, err := safe.StateListAll[*authres.Identity](
		ctx,
		s.omniState,
		state.WithLabelQuery(resource.LabelExists(authres.LabelIdentityTypeServiceAccount)),
	)
	if err != nil {
		return nil, err
	}

	serviceAccounts := make([]*management.ListServiceAccountsResponse_ServiceAccount, 0, identityList.Len())

	for iter := identityList.Iterator(); iter.Next(); {
		identity := iter.Value()

		user, err := safe.StateGet[*authres.User](ctx, s.omniState, authres.NewUser(resources.DefaultNamespace, identity.TypedSpec().Value.UserId).Metadata())
		if err != nil {
			return nil, err
		}

		publicKeyList, err := safe.StateListAll[*authres.PublicKey](
			ctx,
			s.omniState,
			state.WithLabelQuery(resource.LabelEqual(authres.LabelPublicKeyUserID, user.Metadata().ID())),
		)
		if err != nil {
			return nil, err
		}

		publicKeys := make([]*management.ListServiceAccountsResponse_ServiceAccount_PgpPublicKey, 0, publicKeyList.Len())

		for keyIter := publicKeyList.Iterator(); keyIter.Next(); {
			key := keyIter.Value()

			publicKeys = append(publicKeys, &management.ListServiceAccountsResponse_ServiceAccount_PgpPublicKey{
				Id:         key.Metadata().ID(),
				Armored:    string(key.TypedSpec().Value.GetPublicKey()),
				Expiration: key.TypedSpec().Value.GetExpiration(),
			})
		}

		sa, isSa := pkgaccess.ParseServiceAccountFromFullID(identity.Metadata().ID())
		if !isSa {
			return nil, fmt.Errorf("unexpected service account ID %q", identity.Metadata().ID())
		}

		name := sa.NameWithPrefix()

		serviceAccounts = append(serviceAccounts, &management.ListServiceAccountsResponse_ServiceAccount{
			Name:          name,
			PgpPublicKeys: publicKeys,
			Role:          user.TypedSpec().Value.GetRole(),
		})
	}

	return &management.ListServiceAccountsResponse{
		ServiceAccounts: serviceAccounts,
	}, nil
}

func (s *managementServer) DestroyServiceAccount(ctx context.Context, req *management.DestroyServiceAccountRequest) (*emptypb.Empty, error) {
	_, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	sa := pkgaccess.ParseServiceAccountFromName(req.Name)
	id := sa.FullID()

	identity, err := safe.StateGet[*authres.Identity](ctx, s.omniState, authres.NewIdentity(resources.DefaultNamespace, id).Metadata())
	if state.IsNotFoundError(err) {
		return nil, status.Errorf(codes.NotFound, "service account %q not found", id)
	}

	if err != nil {
		return nil, err
	}

	_, isServiceAccount := identity.Metadata().Labels().Get(authres.LabelIdentityTypeServiceAccount)
	if !isServiceAccount {
		return nil, status.Errorf(codes.NotFound, "service account %q not found", req.Name)
	}

	pubKeys, err := s.omniState.List(
		ctx,
		authres.NewPublicKey(resources.DefaultNamespace, "").Metadata(),
		state.WithLabelQuery(resource.LabelEqual(authres.LabelIdentityUserID, identity.TypedSpec().Value.UserId)),
	)
	if err != nil {
		return nil, err
	}

	var destroyErr error

	for _, pubKey := range pubKeys.Items {
		err = s.omniState.Destroy(ctx, pubKey.Metadata())
		if err != nil {
			destroyErr = multierror.Append(destroyErr, err)
		}
	}

	err = s.omniState.Destroy(ctx, identity.Metadata())
	if err != nil {
		destroyErr = multierror.Append(destroyErr, err)
	}

	err = s.omniState.Destroy(ctx, authres.NewUser(resources.DefaultNamespace, identity.TypedSpec().Value.UserId).Metadata())
	if err != nil {
		destroyErr = multierror.Append(destroyErr, err)
	}

	if destroyErr != nil {
		return nil, destroyErr
	}

	return &emptypb.Empty{}, nil
}

func (s *managementServer) serviceAccountKubeconfig(ctx context.Context, req *management.KubeconfigRequest) (*management.KubeconfigResponse, error) {
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return nil, err
	}

	cluster := router.ExtractContext(ctx).Name

	if err := s.validateServiceAccountRequest(cluster, req); err != nil {
		return nil, err
	}

	clusterUUID, err := safe.StateGetByID[*omni.ClusterUUID](ctx, s.omniState, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster UUID: %w", err)
	}

	signedToken, err := s.generateServiceAccountJWT(ctx, req, cluster, clusterUUID.TypedSpec().Value.GetUuid())
	if err != nil {
		return nil, err
	}

	kubeconfig, err := s.buildServiceAccountKubeconfig(cluster, req.GetServiceAccountUser(), signedToken)
	if err != nil {
		return nil, err
	}

	return &management.KubeconfigResponse{
		Kubeconfig: kubeconfig,
	}, nil
}

func (s *managementServer) validateServiceAccountRequest(cluster string, req *management.KubeconfigRequest) error {
	if cluster == "" {
		return status.Error(codes.InvalidArgument, "cluster name is not in context")
	}

	if req.GetServiceAccountUser() == "" {
		return status.Error(codes.InvalidArgument, "service account user name is not set")
	}

	if req.GetServiceAccountTtl() == nil {
		return status.Error(codes.InvalidArgument, "service account ttl is not set")
	}

	ttl := req.GetServiceAccountTtl().AsDuration()

	if ttl <= 0 {
		return status.Error(codes.InvalidArgument, "service account ttl is must be positive")
	}

	if ttl > external.ServiceAccountTokenLifetime {
		return status.Errorf(codes.InvalidArgument, "service account ttl is too long (max allowed: %s)", external.ServiceAccountTokenLifetime)
	}

	return nil
}

func (s *managementServer) generateServiceAccountJWT(ctx context.Context, req *management.KubeconfigRequest, clusterName, clusterUUID string) (string, error) {
	signingKey, err := s.jwtSigningKeyProvider.SigningKey(ctx)
	if err != nil {
		return "", err
	}

	signingMethod := jwt.GetSigningMethod(string(signingKey.SignatureAlgorithm()))

	now := time.Now()
	token := jwt.NewWithClaims(signingMethod, jwt.MapClaims{
		"iat":          now.Unix(),
		"iss":          fmt.Sprintf("omni-%s-service-account-issuer", config.Config.Name),
		"exp":          now.Add(req.GetServiceAccountTtl().AsDuration()).Unix(),
		"sub":          req.GetServiceAccountUser(),
		"groups":       req.GetServiceAccountGroups(),
		"cluster":      clusterName,
		"cluster_uuid": clusterUUID,
	})

	token.Header["kid"] = signingKey.ID()

	return token.SignedString(signingKey.Key())
}

func (s *managementServer) buildServiceAccountKubeconfig(cluster, user, token string) ([]byte, error) {
	clusterName := config.Config.Name + "-" + cluster + "-" + user
	contextName := clusterName

	conf := clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: contextName,
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server: config.Config.KubernetesProxyURL,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:   clusterName,
				Namespace: "default",
				AuthInfo:  clusterName,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			clusterName: {
				Token: token,
			},
		},
	}

	return clientcmd.Write(conf)
}
