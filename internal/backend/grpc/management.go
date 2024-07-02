// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"

	commonOmni "github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	pkgaccess "github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	ctlcfg "github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	omniCtrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// JWTSigningKeyProvider is an interface for a JWT signing key provider.
type JWTSigningKeyProvider interface {
	SigningKey(ctx context.Context) (op.SigningKey, error)
}

// managementServer implements omni management service.
type managementServer struct {
	management.UnimplementedManagementServiceServer

	omniState             state.State
	jwtSigningKeyProvider JWTSigningKeyProvider

	logHandler         *siderolink.LogHandler
	logger             *zap.Logger
	dnsService         *dns.Service
	imageFactoryClient *imagefactory.Client
	omniconfigDest     string
}

func (s *managementServer) register(server grpc.ServiceRegistrar) {
	management.RegisterManagementServiceServer(server, s)
}

func (s *managementServer) gateway(ctx context.Context, mux *gateway.ServeMux, address string, opts []grpc.DialOption) error {
	return management.RegisterManagementServiceHandlerFromEndpoint(ctx, mux, address, opts)
}

func (s *managementServer) Kubeconfig(ctx context.Context, req *management.KubeconfigRequest) (*management.KubeconfigResponse, error) {
	if req.BreakGlass {
		return s.breakGlassKubeconfig(ctx)
	}

	commonContext := router.ExtractContext(ctx)

	clusterName := ""
	if commonContext != nil {
		clusterName = commonContext.Name
	}

	ctx, err := s.applyClusterAccessPolicy(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	if req.GetServiceAccount() {
		return s.serviceAccountKubeconfig(ctx, req)
	}

	// not a service account, generate OIDC (user) or admin kubeconfig

	authResult, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader))
	if err != nil {
		return nil, err
	}

	type oidcRuntime interface {
		GetOIDCKubeconfig(context *commonOmni.Context, identity string, extraOptions ...string) ([]byte, error)
	}

	rt, err := runtime.LookupInterface[oidcRuntime](kubernetes.Name)
	if err != nil {
		return nil, err
	}

	var extraOptions []string

	if req.GrantType != "" {
		switch req.GrantType {
		case "auto":
		case "authcode":
		case "authcode-keyboard":
		default:
			return nil, status.Errorf(codes.InvalidArgument, "invalid grant type %q", req.GrantType)
		}

		extraOptions = []string{
			fmt.Sprintf("grant-type=%s", req.GrantType),
		}
	}

	kubeconfig, err := rt.GetOIDCKubeconfig(commonContext, authResult.Identity, extraOptions...)
	if err != nil {
		return nil, err
	}

	return &management.KubeconfigResponse{
		Kubeconfig: kubeconfig,
	}, nil
}

func (s *managementServer) Talosconfig(ctx context.Context, request *management.TalosconfigRequest) (*management.TalosconfigResponse, error) {
	r := role.Reader
	if request.BreakGlass {
		r = role.Admin
	}

	// getting talosconfig is low risk, as it doesn't contain any sensitive data
	// real check for authentication happens in the Talos API gRPC proxy
	authResult, err := auth.CheckGRPC(ctx, auth.WithRole(r))
	if err != nil {
		return nil, err
	}

	// this one is not low-risk, but it works only in debug mode
	switch {
	case request.Raw:
		return s.breakGlassTalosconfig(ctx, true)
	case request.BreakGlass:
		return s.breakGlassTalosconfig(ctx, false)
	}

	type talosRuntime interface {
		GetTalosconfigRaw(context *commonOmni.Context, identity string) ([]byte, error)
	}

	t, err := runtime.LookupInterface[talosRuntime](talos.Name)
	if err != nil {
		return nil, err
	}

	talosconfig, err := t.GetTalosconfigRaw(router.ExtractContext(ctx), authResult.Identity)
	if err != nil {
		return nil, err
	}

	return &management.TalosconfigResponse{
		Talosconfig: talosconfig,
	}, nil
}

func (s *managementServer) Omniconfig(ctx context.Context, _ *emptypb.Empty) (*management.OmniconfigResponse, error) {
	// getting omniconfig is low risk, since it only contains parameters already known by the user
	authResult, err := auth.CheckGRPC(ctx, auth.WithValidSignature(true))
	if err != nil {
		return nil, err
	}

	cfg, err := generateConfig(authResult, s.omniconfigDest)
	if err != nil {
		return nil, err
	}

	return &management.OmniconfigResponse{
		Omniconfig: cfg,
	}, nil
}

func (s *managementServer) MachineLogs(request *management.MachineLogsRequest, response management.ManagementService_MachineLogsServer) error {
	// getting machine logs is equivalent to reading machine resource
	if _, err := auth.CheckGRPC(response.Context(), auth.WithRole(role.Reader)); err != nil {
		return err
	}

	machineID := request.GetMachineId()
	if machineID == "" {
		return status.Error(codes.InvalidArgument, "machine id is required")
	}

	tailLines := optional.None[int32]()
	if request.TailLines >= 0 {
		tailLines = optional.Some(request.TailLines)
	}

	logReader, err := s.logHandler.GetReader(siderolink.MachineID(machineID), request.Follow, tailLines)
	if err != nil {
		return handleError(err)
	}

	once := sync.Once{}
	cancel := func() {
		once.Do(func() {
			logReader.Close() //nolint:errcheck
		})
	}

	defer cancel()

	panichandler.Go(func() {
		// connection closed, stop reading
		<-response.Context().Done()
		cancel()
	}, s.logger)

	for {
		line, err := logReader.ReadLine()
		if err != nil {
			return handleError(err)
		}

		if err := response.Send(&common.Data{
			Bytes: line,
		}); err != nil {
			return err
		}
	}
}

func (s *managementServer) ValidateConfig(ctx context.Context, request *management.ValidateConfigRequest) (*emptypb.Empty, error) {
	// validating machine config is low risk, require any valid signature
	if _, err := auth.CheckGRPC(ctx, auth.WithValidSignature(true)); err != nil {
		return nil, err
	}

	if err := omnires.ValidateConfigPatch(request.Config); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *managementServer) markClusterAsTainted(ctx context.Context, name string) error {
	ctx = actor.MarkContextAsInternalActor(ctx)

	if err := s.omniState.Create(ctx, omnires.NewClusterTaint(resources.DefaultNamespace, name)); err != nil && !state.IsConflictError(err) {
		return err
	}

	return nil
}

func getRaw(ctx context.Context, clusterName string) ([]byte, error) {
	if !constants.IsDebugBuild {
		return nil, status.Error(codes.PermissionDenied, "not allowed")
	}

	type raw interface {
		RawTalosconfig(ctx context.Context, clusterName string) ([]byte, error)
	}

	omniRuntime, err := runtime.LookupInterface[raw](omni.Name)
	if err != nil {
		return nil, err
	}

	return omniRuntime.RawTalosconfig(ctx, clusterName)
}

func getBreakGlass(ctx context.Context, clusterName string) ([]byte, error) {
	type operator interface {
		OperatorTalosconfig(ctx context.Context, clusterName string) ([]byte, error)
	}

	omniRuntime, err := runtime.LookupInterface[operator](omni.Name)
	if err != nil {
		return nil, err
	}

	return omniRuntime.OperatorTalosconfig(ctx, clusterName)
}

func (s *managementServer) breakGlassTalosconfig(ctx context.Context, raw bool) (*management.TalosconfigResponse, error) {
	if !constants.IsDebugBuild && !config.Config.EnableBreakGlassConfigs {
		return nil, status.Error(codes.PermissionDenied, "not allowed")
	}

	routerContext := router.ExtractContext(ctx)

	if routerContext == nil || routerContext.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster name is required")
	}

	clusterName := routerContext.Name

	var (
		data []byte
		err  error
	)

	if raw {
		data, err = getRaw(ctx, clusterName)
	} else {
		data, err = getBreakGlass(ctx, clusterName)
	}

	if err != nil {
		return nil, err
	}

	if err = s.markClusterAsTainted(ctx, clusterName); err != nil {
		return nil, err
	}

	return &management.TalosconfigResponse{
		Talosconfig: data,
	}, nil
}

func (s *managementServer) breakGlassKubeconfig(ctx context.Context) (*management.KubeconfigResponse, error) {
	_, err := auth.CheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	if !constants.IsDebugBuild && !config.Config.EnableBreakGlassConfigs {
		return nil, status.Error(codes.PermissionDenied, "not allowed")
	}

	routerContext := router.ExtractContext(ctx)

	if routerContext == nil || routerContext.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster name is required")
	}

	clusterName := routerContext.Name

	type kubernetesAdmin interface {
		BreakGlassKubeconfig(ctx context.Context, id string) ([]byte, error)
	}

	kubernetesRuntime, err := runtime.LookupInterface[kubernetesAdmin](kubernetes.Name)
	if err != nil {
		return nil, err
	}

	data, err := kubernetesRuntime.BreakGlassKubeconfig(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	if err = s.markClusterAsTainted(ctx, clusterName); err != nil {
		return nil, err
	}

	return &management.KubeconfigResponse{
		Kubeconfig: data,
	}, nil
}

func (s *managementServer) CreateServiceAccount(ctx context.Context, req *management.CreateServiceAccountRequest) (*management.CreateServiceAccountResponse, error) {
	authCheckResult, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	key, err := validatePGPPublicKey(
		[]byte(req.GetArmoredPgpPublicKey()),
		pgp.WithMaxAllowedLifetime(auth.ServiceAccountMaxAllowedLifetime),
	)
	if err != nil {
		return nil, err
	}

	email := key.username + pkgaccess.ServiceAccountNameSuffix

	_, err = s.omniState.Get(ctx, authres.NewIdentity(resources.DefaultNamespace, email).Metadata())
	if err == nil {
		return nil, fmt.Errorf("service account %q already exists", email)
	}

	if !state.IsNotFoundError(err) { // the identity must not exist
		return nil, err
	}

	newUserID := uuid.New().String()

	publicKeyResource := authres.NewPublicKey(resources.DefaultNamespace, key.id)
	publicKeyResource.Metadata().Labels().Set(authres.LabelPublicKeyUserID, newUserID)

	publicKeyResource.TypedSpec().Value.PublicKey = key.data
	publicKeyResource.TypedSpec().Value.Expiration = timestamppb.New(key.expiration)
	publicKeyResource.TypedSpec().Value.Role = req.Role

	// register the public key of the service account as "confirmed" because we are already authenticated
	publicKeyResource.TypedSpec().Value.Confirmed = true

	publicKeyResource.TypedSpec().Value.Identity = &specs.Identity{
		Email: email,
	}

	if req.GetUseUserRole() {
		publicKeyResource.TypedSpec().Value.Role = string(authCheckResult.Role)
	} else {
		var reqRole role.Role

		reqRole, err = role.Parse(req.GetRole())
		if err != nil {
			return nil, err
		}

		err = authCheckResult.Role.Check(reqRole)
		if err != nil {
			return nil, status.Errorf(
				codes.PermissionDenied,
				"not enough permissions to create service account with role %q: %s",
				req.GetRole(),
				err.Error(),
			)
		}

		publicKeyResource.TypedSpec().Value.Role = req.GetRole()
	}

	err = s.omniState.Create(ctx, publicKeyResource)
	if err != nil {
		return nil, err
	}

	// create the user resource representing the service account with the same scopes as the public key
	user := authres.NewUser(resources.DefaultNamespace, newUserID)
	user.TypedSpec().Value.Role = publicKeyResource.TypedSpec().Value.GetRole()

	err = s.omniState.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// create the identity resource representing the service account
	identity := authres.NewIdentity(resources.DefaultNamespace, email)
	identity.TypedSpec().Value.UserId = user.Metadata().ID()
	identity.Metadata().Labels().Set(authres.LabelIdentityUserID, newUserID)
	identity.Metadata().Labels().Set(authres.LabelIdentityTypeServiceAccount, "")

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

	name := req.Name + pkgaccess.ServiceAccountNameSuffix

	identity, err := safe.StateGet[*authres.Identity](ctx, s.omniState, authres.NewIdentity(resources.DefaultNamespace, name).Metadata())
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
		Email: name,
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

		name := strings.TrimSuffix(identity.Metadata().ID(), pkgaccess.ServiceAccountNameSuffix)

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

	name := req.Name + pkgaccess.ServiceAccountNameSuffix

	identity, err := safe.StateGet[*authres.Identity](ctx, s.omniState, authres.NewIdentity(resources.DefaultNamespace, name).Metadata())
	if state.IsNotFoundError(err) {
		return nil, status.Errorf(codes.NotFound, "service account %q not found", name)
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

func (s *managementServer) KubernetesUpgradePreChecks(ctx context.Context, req *management.KubernetesUpgradePreChecksRequest) (*management.KubernetesUpgradePreChecksResponse, error) {
	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	requestContext := router.ExtractContext(ctx)
	if requestContext == nil {
		return nil, status.Error(codes.InvalidArgument, "unable to extract request context")
	}

	upgradeStatus, err := safe.StateGet[*omnires.KubernetesUpgradeStatus](ctx, s.omniState, omnires.NewKubernetesUpgradeStatus(resources.DefaultNamespace, requestContext.Name).Metadata())
	if err != nil {
		return nil, err
	}

	currentVersion := upgradeStatus.TypedSpec().Value.LastUpgradeVersion
	if currentVersion == "" {
		return nil, status.Error(codes.FailedPrecondition, "current version is not known yet")
	}

	path, err := upgrade.NewPath(currentVersion, req.NewVersion)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid upgrade path: %v", err)
	}

	if !path.IsSupported() {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported upgrade path: %s", path)
	}

	type kubeConfigGetter interface {
		GetKubeconfig(ctx context.Context, cluster *commonOmni.Context) (*rest.Config, error)
	}

	k8sRuntime, err := runtime.LookupInterface[kubeConfigGetter](kubernetes.Name)
	if err != nil {
		return nil, err
	}

	restConfig, err := k8sRuntime.GetKubeconfig(ctx, requestContext)
	if err != nil {
		return nil, fmt.Errorf("error getting kubeconfig: %w", err)
	}

	type talosClientGetter interface {
		GetClient(ctx context.Context, clusterName string) (*talos.Client, error)
	}

	talosRuntime, err := runtime.LookupInterface[talosClientGetter](talos.Name)
	if err != nil {
		return nil, err
	}

	talosClient, err := talosRuntime.GetClient(ctx, requestContext.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting talos client: %w", err)
	}

	var controlplaneNodes []string

	cmis, err := safe.StateListAll[*omnires.ClusterMachineIdentity](
		ctx,
		s.omniState,
		state.WithLabelQuery(
			resource.LabelEqual(omnires.LabelCluster, requestContext.Name),
			resource.LabelExists(omnires.LabelControlPlaneRole),
		),
	)
	if err != nil {
		return nil, err
	}

	for iter := cmis.Iterator(); iter.Next(); {
		if len(iter.Value().TypedSpec().Value.NodeIps) > 0 {
			controlplaneNodes = append(controlplaneNodes, iter.Value().TypedSpec().Value.NodeIps[0])
		}
	}

	s.logger.Debug("running k8s upgrade pre-checks", zap.Strings("controlplane_nodes", controlplaneNodes), zap.String("cluster", requestContext.Name))

	var logBuffer strings.Builder

	preCheck, err := upgrade.NewChecks(path, talosClient.COSI, restConfig, controlplaneNodes, nil, func(format string, args ...any) {
		fmt.Fprintf(&logBuffer, format, args...)
		fmt.Fprintln(&logBuffer)
	})
	if err != nil {
		return nil, err
	}

	if err = preCheck.Run(ctx); err != nil {
		s.logger.Error("failed running pre-checks", zap.String("log", logBuffer.String()), zap.String("cluster", requestContext.Name), zap.Error(err))

		fmt.Fprintf(&logBuffer, "pre-checks failed: %v\n", err)

		return &management.KubernetesUpgradePreChecksResponse{
			Ok:     false,
			Reason: logBuffer.String(),
		}, nil
	}

	s.logger.Debug("k8s upgrade pre-checks successful", zap.String("log", logBuffer.String()), zap.String("cluster", requestContext.Name))

	return &management.KubernetesUpgradePreChecksResponse{
		Ok: true,
	}, nil
}

//nolint:gocognit,gocyclo,cyclop
func (s *managementServer) KubernetesSyncManifests(req *management.KubernetesSyncManifestRequest, srv management.ManagementService_KubernetesSyncManifestsServer) error {
	ctx := srv.Context()

	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	requestContext := router.ExtractContext(ctx)
	if requestContext == nil {
		return status.Error(codes.InvalidArgument, "unable to extract request context")
	}

	type kubernetesConfigurator interface {
		GetKubeconfig(ctx context.Context, context *commonOmni.Context) (*rest.Config, error)
	}

	kubernetesRuntime, err := runtime.LookupInterface[kubernetesConfigurator](kubernetes.Name)
	if err != nil {
		return err
	}

	cfg, err := kubernetesRuntime.GetKubeconfig(ctx, requestContext)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	type talosClientProvider interface {
		GetClient(ctx context.Context, clusterName string) (*talos.Client, error)
	}

	talosRuntime, err := runtime.LookupInterface[talosClientProvider](talos.Name)
	if err != nil {
		return err
	}

	talosClient, err := talosRuntime.GetClient(ctx, requestContext.Name)
	if err != nil {
		return fmt.Errorf("failed to get talos client: %w", err)
	}

	bootstrapManifests, err := manifests.GetBootstrapManifests(ctx, talosClient.COSI, nil)
	if err != nil {
		return fmt.Errorf("failed to get manifests: %w", err)
	}

	errCh := make(chan error, 1)
	synCh := make(chan manifests.SyncResult)

	panichandler.Go(func() {
		errCh <- manifests.Sync(ctx, bootstrapManifests, cfg, req.DryRun, synCh)
	}, s.logger)

	var updatedManifests []manifests.Manifest

syncLoop:
	for {
		select {
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("failed to sync manifests: %w", err)
			}

			break syncLoop
		case result := <-synCh:
			obj, err := yaml.Marshal(result.Object.Object)
			if err != nil {
				return fmt.Errorf("failed to marshal object: %w", err)
			}

			if err := srv.Send(&management.KubernetesSyncManifestResponse{
				ResponseType: management.KubernetesSyncManifestResponse_MANIFEST,
				Path:         result.Path,
				Object:       obj,
				Diff:         result.Diff,
				Skipped:      result.Skipped,
			}); err != nil {
				return err
			}

			if !result.Skipped {
				updatedManifests = append(updatedManifests, result.Object)
			}
		}
	}

	if req.DryRun {
		// no rollout if dry run
		return s.triggerManifestResync(ctx, requestContext)
	}

	rolloutCh := make(chan manifests.RolloutProgress)

	panichandler.Go(func() {
		errCh <- manifests.WaitForRollout(ctx, cfg, updatedManifests, rolloutCh)
	}, s.logger)

rolloutLoop:
	for {
		select {
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("failed to wait fo rollout: %w", err)
			}

			break rolloutLoop
		case result := <-rolloutCh:
			obj, err := yaml.Marshal(result.Object.Object)
			if err != nil {
				return fmt.Errorf("failed to marshal object: %w", err)
			}

			if err := srv.Send(&management.KubernetesSyncManifestResponse{
				ResponseType: management.KubernetesSyncManifestResponse_ROLLOUT,
				Path:         result.Path,
				Object:       obj,
			}); err != nil {
				return err
			}
		}
	}

	return s.triggerManifestResync(ctx, requestContext)
}

func (s *managementServer) triggerManifestResync(ctx context.Context, requestContext *commonOmni.Context) error {
	// trigger fake update in KubernetesUpgradeStatusType to force re-calculating the status
	// this is needed because the status is not updated when the rollout is finished
	_, err := safe.StateUpdateWithConflicts(
		ctx,
		s.omniState,
		omnires.NewKubernetesUpgradeStatus(resources.DefaultNamespace, requestContext.Name).Metadata(),
		func(res *omnires.KubernetesUpgradeStatus) error {
			res.Metadata().Annotations().Set("manifest-rollout", strconv.Itoa(int(time.Now().Unix())))

			return nil
		},
		state.WithUpdateOwner(omniCtrl.KubernetesUpgradeStatusControllerName),
	)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to update KubernetesUpgradeStatus: %w", err)
	}

	return nil
}

func (s *managementServer) authCheckGRPC(ctx context.Context, opts ...auth.CheckOption) (auth.CheckResult, error) {
	authCheckResult, err := auth.Check(ctx, opts...)
	if errors.Is(err, auth.ErrUnauthenticated) {
		return auth.CheckResult{}, status.Error(codes.Unauthenticated, err.Error())
	}

	if errors.Is(err, auth.ErrUnauthorized) {
		return auth.CheckResult{}, status.Error(codes.PermissionDenied, err.Error())
	}

	if err != nil {
		return auth.CheckResult{}, err
	}

	return authCheckResult, nil
}

// applyClusterAccessPolicy checks the ACLs for the user in the context against the given cluster ID.
// If there is a match and the matched role is higher than the user's role,
// a child context containing the given role will be returned.
func (s *managementServer) applyClusterAccessPolicy(ctx context.Context, clusterID resource.ID) (context.Context, error) {
	clusterRole, _, err := accesspolicy.RoleForCluster(ctx, clusterID, s.omniState)
	if err != nil {
		return nil, err
	}

	userRole, userRoleExists := ctx.Value(auth.RoleContextKey{}).(role.Role)
	if !userRoleExists {
		userRole = role.None
	}

	newRole, err := role.Max(userRole, clusterRole)
	if err != nil {
		return nil, err
	}

	if newRole == userRole {
		return ctx, nil
	}

	return context.WithValue(ctx, auth.RoleContextKey{}, newRole), nil
}

func handleError(err error) error {
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case siderolink.IsBufferNotFoundError(err):
		return status.Error(codes.NotFound, err.Error())
	}

	return err
}

func generateConfig(authResult auth.CheckResult, contextURL string) ([]byte, error) {
	// This is safe to do, since omnictl config pkg doesn't import anything from the backend
	cfg := &ctlcfg.Config{
		Contexts: map[string]*ctlcfg.Context{
			"default": {
				URL: contextURL,
				Auth: ctlcfg.Auth{
					SideroV1: ctlcfg.SideroV1{
						Identity: authResult.Identity,
					},
				},
			},
		},
		Context: "default",
	}

	result, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal omnicfg: %w", err)
	}

	return result, error(nil)
}

func generateDest(apiurl string) (string, error) {
	parsedDest, err := url.Parse(apiurl)
	if err != nil {
		return "", fmt.Errorf("incorrect destination: %w", err)
	}

	result := parsedDest.String()
	if result == "" {
		// This can happen if Parse actually failed but didn't return an error
		return "", fmt.Errorf("incorrect destination '%s'", parsedDest)
	}

	return result, nil
}
