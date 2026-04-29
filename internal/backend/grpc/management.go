// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/client-go/rest"

	commonOmni "github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	ctlcfg "github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	omniCtrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
	siderolinkinternal "github.com/siderolabs/omni/internal/pkg/siderolink"
)

// KubernetesRuntime provides kubernetes cluster access capabilities.
type KubernetesRuntime interface {
	GetKubeconfig(ctx context.Context, cluster *commonOmni.Context) (*rest.Config, error)
	GetOIDCKubeconfig(context *commonOmni.Context, identity string, extraOptions ...string) ([]byte, error)
	BreakGlassKubeconfig(ctx context.Context, id string) ([]byte, error)
	GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
}

// TalosRuntime provides Talos cluster access capabilities.
type TalosRuntime interface {
	GetTalosconfigRaw(context *commonOmni.Context, identity string) ([]byte, error)
	GetClientForCluster(ctx context.Context, clusterName string) (*talos.Client, error)
	GetClientForMachine(ctx context.Context, machineID string) (*talos.Client, error)
}

// TalosconfigProvider provides raw and operator Talos configurations.
type TalosconfigProvider interface {
	RawTalosconfig(ctx context.Context, clusterName string) ([]byte, error)
	OperatorTalosconfig(ctx context.Context, clusterName string) ([]byte, error)
}

// JWTSigningKeyProvider is an interface for a JWT signing key provider.
type JWTSigningKeyProvider interface {
	SigningKey(ctx context.Context) (op.SigningKey, error)
}

func newManagementServer(cfg *config.Params, omniState state.State, jwtSigningKeyProvider JWTSigningKeyProvider, logHandler *siderolinkinternal.LogHandler, logger *zap.Logger,
	dnsService *dns.Service, imageFactoryClient *imagefactory.Client, auditor AuditLogger, omniconfigDest string,
	kubernetesRuntime KubernetesRuntime, talosRuntime TalosRuntime, talosconfigProvider TalosconfigProvider,
) *managementServer {
	return &managementServer{
		cfg:                   cfg,
		omniState:             omniState,
		jwtSigningKeyProvider: jwtSigningKeyProvider,
		logHandler:            logHandler,
		logger:                logger,
		dnsService:            dnsService,
		imageFactoryClient:    imageFactoryClient,
		auditor:               auditor,
		omniconfigDest:        omniconfigDest,
		kubernetesRuntime:     kubernetesRuntime,
		talosRuntime:          talosRuntime,
		talosconfigProvider:   talosconfigProvider,
	}
}

// managementServer implements omni management service.
type managementServer struct {
	management.UnimplementedManagementServiceServer
	cfg                   *config.Params
	kubernetesRuntime     KubernetesRuntime
	omniState             state.State
	jwtSigningKeyProvider JWTSigningKeyProvider
	auditor               AuditLogger
	talosconfigProvider   TalosconfigProvider
	talosRuntime          TalosRuntime
	logHandler            *siderolinkinternal.LogHandler
	logger                *zap.Logger
	dnsService            *dns.Service
	imageFactoryClient    *imagefactory.Client
	omniconfigDest        string
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

	ctx, authResult, err := s.checkClusterAuthorization(ctx, clusterName, role.Reader)
	if err != nil {
		return nil, err
	}

	if req.GetServiceAccount() {
		return s.serviceAccountKubeconfig(ctx, req)
	}

	// not a service account, generate OIDC (user) or admin kubeconfig

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
			"grant-type=" + req.GrantType,
			"oidc-redirect-url=urn:ietf:wg:oauth:2.0:oob",
		}
	}

	// Resolve OIDC cache directory: request fields (from omnictl) take priority over server config.
	cacheDir := req.OidcCacheBaseDir
	if cacheDir == "" {
		cacheDir = s.cfg.Services.KubernetesProxy.GetOidcCacheBaseDir()
	}

	cacheIsolation := req.OidcCacheIsolation
	if !cacheIsolation {
		cacheIsolation = s.cfg.Services.KubernetesProxy.GetOidcCacheIsolation()
	}

	if cacheIsolation {
		if cacheDir == "" {
			cacheDir = filepath.Join("~", ".kube", "cache", "oidc-login")
		}

		cacheDir = filepath.Join(cacheDir, s.cfg.Account.GetName()+"-"+clusterName+"-"+authResult.Identity)
	}

	if cacheDir != "" {
		extraOptions = append(extraOptions, "token-cache-dir="+cacheDir)
	}

	kubeconfig, err := s.kubernetesRuntime.GetOIDCKubeconfig(commonContext, authResult.Identity, extraOptions...)
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

	routerContext := router.ExtractContext(ctx)

	clusterName := ""
	if routerContext != nil {
		clusterName = routerContext.Name
	}

	if !request.BreakGlass {
		var err error

		ctx, _, err = s.checkClusterAuthorization(ctx, clusterName, r)
		if err != nil {
			return nil, err
		}
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

	talosconfig, err := s.talosRuntime.GetTalosconfigRaw(routerContext, authResult.Identity)
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

func (s *managementServer) MachineLogs(request *management.MachineLogsRequest, serv grpc.ServerStreamingServer[common.Data]) error {
	ctx := serv.Context()

	machineID := request.GetMachineId()
	if machineID == "" {
		return status.Error(codes.InvalidArgument, "machine id is required")
	}

	// getting machine logs is equivalent to reading machine resource
	authCtx, _, err := s.checkAuthorization(ctx, machineID, role.Reader)
	if err != nil {
		return err
	}

	tailLines := optional.None[int32]()
	if request.TailLines >= 0 {
		tailLines = optional.Some(request.TailLines)
	}

	logReader, err := s.logHandler.GetReader(authCtx, siderolinkinternal.MachineID(machineID), request.Follow, tailLines)
	if err != nil {
		return handleError(err)
	}

	defer logReader.Close() //nolint:errcheck

	for {
		line, err := logReader.ReadLine(authCtx)
		if err != nil {
			return handleError(err)
		}

		if err := serv.Send(&common.Data{
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

	if err := omnires.ValidateConfigPatch([]byte(request.Config)); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *managementServer) markClusterAsTainted(ctx context.Context, name string) error {
	ctx = actor.MarkContextAsInternalActor(ctx)

	if _, err := safe.StateUpdateWithConflicts(
		ctx,
		s.omniState,
		omnires.NewClusterStatus(name).Metadata(),
		func(res *omnires.ClusterStatus) error {
			res.Metadata().Labels().Set(omnires.LabelClusterTaintedByBreakGlass, "")
			res.Metadata().Annotations().Set(omnires.TaintedByBreakGlassTimestamp, strconv.FormatInt(time.Now().Unix(), 10))

			return nil
		},
		state.WithUpdateOwner(omniCtrl.ClusterStatusControllerName),
	); err != nil {
		return err
	}

	return nil
}

func (s *managementServer) getRaw(ctx context.Context, clusterName string) ([]byte, error) {
	if !constants.IsDebugBuild {
		return nil, status.Error(codes.PermissionDenied, "not allowed")
	}

	return s.talosconfigProvider.RawTalosconfig(ctx, clusterName)
}

func (s *managementServer) getBreakGlass(ctx context.Context, clusterName string) ([]byte, error) {
	return s.talosconfigProvider.OperatorTalosconfig(ctx, clusterName)
}

func (s *managementServer) breakGlassTalosconfig(ctx context.Context, raw bool) (*management.TalosconfigResponse, error) {
	if !constants.IsDebugBuild && !s.cfg.Features.GetEnableBreakGlassConfigs() {
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
		data, err = s.getRaw(ctx, clusterName)
	} else {
		data, err = s.getBreakGlass(ctx, clusterName)
	}

	if err != nil {
		return nil, err
	}

	if err = s.auditTalosAccess(ctx, management.ManagementService_Talosconfig_FullMethodName, clusterName, ""); err != nil {
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

	if !constants.IsDebugBuild && !s.cfg.Features.GetEnableBreakGlassConfigs() {
		return nil, status.Error(codes.PermissionDenied, "not allowed")
	}

	routerContext := router.ExtractContext(ctx)

	if routerContext == nil || routerContext.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster name is required")
	}

	clusterName := routerContext.Name

	data, err := s.kubernetesRuntime.BreakGlassKubeconfig(ctx, clusterName)
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

func (s *managementServer) KubernetesUpgradePreChecks(ctx context.Context, req *management.KubernetesUpgradePreChecksRequest) (*management.KubernetesUpgradePreChecksResponse, error) {
	requestContext := router.ExtractContext(ctx)
	if requestContext == nil {
		return nil, status.Error(codes.InvalidArgument, "unable to extract request context")
	}

	authCtx, _, err := s.checkClusterAuthorization(ctx, requestContext.Name, role.Operator)
	if err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(authCtx)

	upgradeStatus, err := safe.StateGet[*omnires.KubernetesUpgradeStatus](ctx, s.omniState, omnires.NewKubernetesUpgradeStatus(requestContext.Name).Metadata())
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

	restConfig, err := s.kubernetesRuntime.GetKubeconfig(ctx, requestContext)
	if err != nil {
		return nil, fmt.Errorf("error getting kubeconfig: %w", err)
	}

	var controlplaneMachines []string

	cms, err := safe.StateListAll[*omnires.ClusterMachine](
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

	for cm := range cms.All() {
		controlplaneMachines = append(controlplaneMachines, cm.Metadata().ID())
	}

	if err = s.auditTalosAccessForNodes(authCtx, management.ManagementService_KubernetesUpgradePreChecks_FullMethodName, requestContext.Name, controlplaneMachines); err != nil {
		return nil, err
	}

	s.logger.Debug("running k8s upgrade pre-checks", zap.Strings("controlplane_machines", controlplaneMachines), zap.String("cluster", requestContext.Name))

	var logBuffer strings.Builder

	preCheck, err := upgrade.NewChecksWithStateProvider(path, func(ctx context.Context, machineID string) (state.State, error) {
		c, clientErr := s.talosRuntime.GetClientForMachine(ctx, machineID)
		if clientErr != nil {
			return nil, clientErr
		}

		return c.COSI, nil
	}, restConfig, controlplaneMachines, nil, func(format string, args ...any) {
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

// ReadAuditLog reads the audit log from the backend.
func (s *managementServer) ReadAuditLog(req *management.ReadAuditLogRequest, srv grpc.ServerStreamingServer[management.ReadAuditLogResponse]) error {
	ctx := srv.Context()

	_, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return err
	}

	now := time.Now()

	filters := auditlog.ReadFilters{
		OrderByField: auditLogOrderByField(req.GetOrderByField()),
		OrderByDir:   auditLogOrderByDir(req.GetOrderByDir()),
		Search:       req.GetSearch(),
		EventType:    auditLogEventType(req.GetEventType()),
		ResourceType: req.GetResourceType(),
		ResourceID:   req.GetResourceId(),
		ClusterID:    req.GetClusterId(),
		Actor:        req.GetActor(),
	}

	filters.Start, err = parseTime(req.GetStartTime(), now.AddDate(0, 0, -29))
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid start time: %v", err)
	}

	filters.End, err = parseTime(req.GetEndTime(), now)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid end time: %v", err)
	}

	rdr, err := s.auditor.Reader(ctx, filters)
	if err != nil {
		return err
	}

	closeFn := sync.OnceValue(rdr.Close)
	defer closeFn() //nolint:errcheck

	for {
		data, err := rdr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		if err = srv.Send(&management.ReadAuditLogResponse{AuditLog: data}); err != nil {
			return err
		}
	}

	return closeFn()
}

// MaintenanceUpgrade performs a maintenance upgrade on a specified machine.
func (s *managementServer) MaintenanceUpgrade(ctx context.Context, req *management.MaintenanceUpgradeRequest) (*management.MaintenanceUpgradeResponse, error) {
	s.logger.Info("maintenance upgrade request received", zap.String("machine_id", req.MachineId), zap.String("version", req.Version))

	if req.MachineId == "" {
		return nil, status.Error(codes.InvalidArgument, "machine id is required")
	}

	authCtx, clusterName, err := s.checkAuthorization(ctx, req.MachineId, role.Operator)
	if err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(authCtx)

	machineStatus, err := safe.StateGetByID[*omnires.MachineStatus](ctx, s.omniState, req.MachineId)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "machine not found")
		}

		return nil, fmt.Errorf("failed to get machine status: %w", err)
	}

	if !machineStatus.TypedSpec().Value.Maintenance {
		return nil, status.Error(codes.FailedPrecondition, "machine is not in maintenance mode")
	}

	if !machineStatus.TypedSpec().Value.SchematicReady() {
		return nil, status.Error(codes.FailedPrecondition, "machine schematic is not ready yet")
	}

	platform := machineStatus.TypedSpec().Value.GetPlatformMetadata().GetPlatform()
	if platform == "" {
		return nil, status.Error(codes.FailedPrecondition, "machine platform is not known yet")
	}

	securityState := machineStatus.TypedSpec().Value.SecurityState
	if securityState == nil {
		return nil, status.Error(codes.FailedPrecondition, "machine security state is not known yet")
	}

	installImage := &specs.MachineConfigGenOptionsSpec_InstallImage{
		TalosVersion:         req.Version,
		SchematicId:          machineStatus.TypedSpec().Value.Schematic.FullId,
		SchematicInitialized: true,
		Platform:             platform,
		SecurityState:        securityState,
	}

	installImageStr, err := installimage.Build(s.imageFactoryClient.Host(), req.MachineId, installImage, s.cfg.Registries.GetTalos())
	if err != nil {
		return nil, fmt.Errorf("failed to build install image: %w", err)
	}

	s.logger.Info("built maintenance upgrade image", zap.String("image", installImageStr))

	address := machineStatus.TypedSpec().Value.ManagementAddress
	opts := talos.GetSocketOptions(address)
	opts = append(opts, client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), client.WithEndpoints(address))

	talosClient, err := client.New(authCtx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create talos client: %w", err)
	}

	if err = s.auditTalosAccess(authCtx, machineapi.MachineService_Upgrade_FullMethodName, clusterName, req.MachineId); err != nil {
		return nil, err
	}

	//nolint:staticcheck
	if _, err = talosClient.UpgradeWithOptions(authCtx, client.WithUpgradeImage(installImageStr)); err != nil {
		return nil, fmt.Errorf("failed to upgrade machine: %w", err)
	}

	return &management.MaintenanceUpgradeResponse{}, nil
}

func (s *managementServer) GetMachineJoinConfig(ctx context.Context, request *management.GetMachineJoinConfigRequest) (*management.GetMachineJoinConfigResponse, error) {
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	apiConfig, err := safe.StateGetByID[*siderolinkres.APIConfig](ctx, s.omniState, siderolinkres.ConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get APIConfig for the extra kernel arguments: %w", err)
	}

	if request.JoinToken == "" {
		var defaultToken *siderolinkres.DefaultJoinToken

		defaultToken, err = safe.StateGetByID[*siderolinkres.DefaultJoinToken](ctx, s.omniState, siderolinkres.DefaultJoinTokenID)
		if err != nil {
			return nil, fmt.Errorf("failed to get the default join token: %w", err)
		}

		request.JoinToken = defaultToken.TypedSpec().Value.TokenId
	}

	// If the tunnel is enabled instance-wide or in the request, the final state is enabled
	opts, err := siderolink.NewJoinOptions(
		siderolink.WithMachineAPIURL(apiConfig.TypedSpec().Value.MachineApiAdvertisedUrl),
		siderolink.WithJoinToken(request.JoinToken),
		siderolink.WithGRPCTunnel(request.UseGrpcTunnel),
		siderolink.WithEventSinkPort(int(apiConfig.TypedSpec().Value.EventsPort)),
		siderolink.WithLogServerPort(int(apiConfig.TypedSpec().Value.LogsPort)),
	)
	if err != nil {
		return nil, err
	}

	config, err := opts.RenderJoinConfig()
	if err != nil {
		return nil, err
	}

	return &management.GetMachineJoinConfigResponse{
		Config:     string(config),
		KernelArgs: opts.GetKernelArgs(),
	}, nil
}

func (s *managementServer) CreateJoinToken(ctx context.Context, request *management.CreateJoinTokenRequest) (*management.CreateJoinTokenResponse, error) {
	_, err := auth.CheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	token, err := jointoken.Generate()
	if err != nil {
		return nil, err
	}

	joinToken := siderolinkres.NewJoinToken(token)

	if request.Name == "" && len(request.Name) > omni.MaxJoinTokenNameLength {
		return nil, status.Error(codes.InvalidArgument, "token name is invalid")
	}

	joinToken.TypedSpec().Value.Name = request.Name
	joinToken.TypedSpec().Value.ExpirationTime = request.ExpirationTime

	if err = s.omniState.Create(actor.MarkContextAsInternalActor(ctx), joinToken); err != nil {
		return nil, err
	}

	return &management.CreateJoinTokenResponse{
		Id: token,
	}, nil
}

func (s *managementServer) ResetNodeUniqueToken(ctx context.Context, request *management.ResetNodeUniqueTokenRequest) (*management.ResetNodeUniqueTokenResponse, error) {
	_, err := auth.CheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	_, err = safe.StateGetByID[*omnires.MachineStatus](ctx, s.omniState, request.Id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "machine not found")
		}

		return nil, fmt.Errorf("failed to get machine status: %w", err)
	}

	if err = s.omniState.TeardownAndDestroy(actor.MarkContextAsInternalActor(ctx), siderolinkres.NewNodeUniqueToken(request.Id).Metadata()); err != nil {
		return nil, fmt.Errorf("failed to teardown and destroy node unique token: %w", err)
	}

	return &management.ResetNodeUniqueTokenResponse{}, nil
}

func (s *managementServer) MachinePowerOff(ctx context.Context, request *management.MachinePowerOffRequest) (*management.MachinePowerOffResponse, error) {
	if request.MachineId == "" {
		return nil, status.Error(codes.InvalidArgument, "machine id is required")
	}

	authCtx, clusterName, err := s.checkAuthorization(ctx, request.MachineId, role.Operator)
	if err != nil {
		return nil, err
	}

	internalCtx := actor.MarkContextAsInternalActor(authCtx)

	isManagedByStaticInfraProvider, err := s.isManagedByStaticInfraProvider(internalCtx, request.MachineId)
	if err != nil {
		return nil, err
	}

	// For machines managed by a static infra provider, update InfraMachineConfig with a new power off request ID.
	if isManagedByStaticInfraProvider {
		if err = s.setMachinePowerOffRequestID(internalCtx, request.MachineId); err != nil {
			return nil, err
		}
	}

	// Call Talos Shutdown API.
	talosClient, err := s.talosRuntime.GetClientForMachine(internalCtx, request.MachineId)
	if err != nil {
		return nil, fmt.Errorf("failed to get talos client: %w", err)
	}

	if err = s.auditTalosAccess(authCtx, machineapi.MachineService_Shutdown_FullMethodName, clusterName, request.MachineId); err != nil {
		return nil, err
	}

	if err = talosClient.Shutdown(authCtx); err != nil {
		return nil, fmt.Errorf("failed to shutdown machine: %w", err)
	}

	return &management.MachinePowerOffResponse{}, nil
}

func (s *managementServer) MachinePowerOn(ctx context.Context, request *management.MachinePowerOnRequest) (*management.MachinePowerOnResponse, error) {
	if request.MachineId == "" {
		return nil, status.Error(codes.InvalidArgument, "machine id is required")
	}

	authCtx, _, err := s.checkAuthorization(ctx, request.MachineId, role.Operator)
	if err != nil {
		return nil, err
	}

	internalCtx := actor.MarkContextAsInternalActor(authCtx)

	isManagedByStaticInfraProvider, err := s.isManagedByStaticInfraProvider(internalCtx, request.MachineId)
	if err != nil {
		return nil, err
	}

	if !isManagedByStaticInfraProvider {
		return nil, status.Error(codes.FailedPrecondition, "machine is not managed by a static infra provider, power on the machine manually")
	}

	if err = s.clearMachinePowerOffRequestID(internalCtx, request.MachineId); err != nil {
		return nil, err
	}

	return &management.MachinePowerOnResponse{}, nil
}

// checkAuthorization checks if the user has the required role to perform an action on a machine.
//
// It also returns the authorized context and the cluster name the machine belongs to, if available.
func (s *managementServer) checkAuthorization(ctx context.Context, machineID string, checkRole role.Role) (context.Context, string, error) {
	cm, err := s.omniState.Get(actor.MarkContextAsInternalActor(ctx), omnires.NewClusterMachine(machineID).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, "", err
	}

	cluster := ""

	if cm != nil {
		var ok bool

		if cluster, ok = cm.Metadata().Labels().Get(omnires.LabelCluster); !ok {
			return nil, "", status.Errorf(codes.FailedPrecondition, "cluster label not found on cluster machine %q", machineID)
		}
	}

	authCtx, _, err := s.checkClusterAuthorization(ctx, cluster, checkRole)
	if err != nil {
		return nil, "", err
	}

	return authCtx, cluster, nil
}

func (s *managementServer) checkClusterAuthorization(ctx context.Context, cluster string, checkRole role.Role) (context.Context, auth.CheckResult, error) {
	authCtx := ctx

	var err error

	if cluster != "" {
		authCtx, err = accesspolicy.ApplyClusterAccessPolicy(ctx, cluster, s.omniState)
		if err != nil {
			return nil, auth.CheckResult{}, err
		}
	}

	authResult, err := auth.CheckGRPC(authCtx, auth.WithRole(checkRole))
	if err != nil {
		return nil, auth.CheckResult{}, err
	}

	return authCtx, authResult, nil
}

func (s *managementServer) auditTalosAccess(ctx context.Context, fullMethodName, clusterID, nodeID string) error {
	if s.auditor == nil {
		return nil
	}

	fullMethodName = strings.TrimLeft(fullMethodName, "/")

	if err := s.auditor.AuditTalosAccess(ctx, fullMethodName, clusterID, nodeID); err != nil {
		return fmt.Errorf("failed to audit talos access: %w", err)
	}

	return nil
}

func (s *managementServer) auditTalosAccessForNodes(ctx context.Context, fullMethodName, clusterID string, nodeIDs []string) error {
	for _, nodeID := range nodeIDs {
		if err := s.auditTalosAccess(ctx, fullMethodName, clusterID, nodeID); err != nil {
			return err
		}
	}

	return nil
}

func (s *managementServer) setMachinePowerOffRequestID(ctx context.Context, machineID string) error {
	ctx = actor.MarkContextAsInternalActor(ctx)

	_, err := safe.StateUpdateWithConflicts(ctx, s.omniState, omnires.NewInfraMachineConfig(machineID).Metadata(),
		func(config *omnires.InfraMachineConfig) error {
			if config.TypedSpec().Value.AcceptanceStatus != specs.InfraMachineConfigSpec_ACCEPTED {
				return fmt.Errorf("infra machine config is not accepted yet, cannot set machine power-off request")
			}

			config.TypedSpec().Value.PowerOffRequestId = uuid.NewString()

			return nil
		},
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return status.Error(codes.FailedPrecondition, "infra machine config not found, cannot set machine power-off request")
		}

		return fmt.Errorf("failed to update infra machine config: %w", err)
	}

	return nil
}

func (s *managementServer) isManagedByStaticInfraProvider(ctx context.Context, machineID string) (bool, error) {
	link, err := safe.StateGetByID[*siderolinkres.Link](ctx, s.omniState, machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, status.Error(codes.NotFound, "machine not found")
		}

		return false, fmt.Errorf("failed to get machine link: %w", err)
	}

	infraProviderID, ok := link.Metadata().Labels().Get(omnires.LabelInfraProviderID)
	if !ok {
		return false, nil
	}

	providerStatus, err := safe.StateGetByID[*infra.ProviderStatus](ctx, s.omniState, infraProviderID)
	if err != nil {
		return false, status.Error(codes.FailedPrecondition, "failed to get infra provider status")
	}

	_, isStaticInfraProvider := providerStatus.Metadata().Labels().Get(omnires.LabelIsStaticInfraProvider)

	return isStaticInfraProvider, nil
}

func (s *managementServer) clearMachinePowerOffRequestID(ctx context.Context, machineID string) error {
	ctx = actor.MarkContextAsInternalActor(ctx)

	_, err := safe.StateUpdateWithConflicts(ctx, s.omniState, omnires.NewInfraMachineConfig(machineID).Metadata(),
		func(config *omnires.InfraMachineConfig) error {
			if config.TypedSpec().Value.AcceptanceStatus != specs.InfraMachineConfigSpec_ACCEPTED {
				return fmt.Errorf("infra machine config is not accepted yet, cannot clear machine power-off request")
			}

			config.TypedSpec().Value.PowerOffRequestId = ""

			return nil
		},
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return status.Error(codes.FailedPrecondition, "infra machine config not found, cannot clear machine power-off request")
		}

		return fmt.Errorf("failed to update infra machine config: %w", err)
	}

	return nil
}

func parseTime(date string, fallback time.Time) (time.Time, error) {
	if date == "" {
		return fallback, nil
	}

	result, err := time.ParseInLocation(time.DateOnly, date, time.Local) //nolint:gosmopolitan
	if err != nil {
		return time.Time{}, err
	}

	return result, nil
}

func (s *managementServer) triggerManifestResync(ctx context.Context, requestContext *commonOmni.Context) error {
	// trigger fake update in KubernetesUpgradeStatusType to force re-calculating the status
	// this is needed because the status is not updated when the rollout is finished
	_, err := safe.StateUpdateWithConflicts(
		ctx,
		s.omniState,
		omnires.NewKubernetesUpgradeStatus(requestContext.Name).Metadata(),
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

func handleError(err error) error {
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case errors.Is(err, siderolinkinternal.ErrLogStoreNotFound):
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

// AuditLogger is an interface for reading the audit log and logging Talos access events.
type AuditLogger interface {
	Reader(ctx context.Context, filters auditlog.ReadFilters) (auditlog.Reader, error)
	AuditTalosAccess(ctx context.Context, fullMethodName, clusterID, nodeID string) error
}

func auditLogOrderByField(f management.AuditLogOrderByField) auditlog.OrderByField {
	switch f {
	case management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_UNSPECIFIED:
		return auditlog.OrderByFieldDate
	case management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_DATE:
		return auditlog.OrderByFieldDate
	case management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_EVENT_TYPE:
		return auditlog.OrderByFieldEventType
	case management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_RESOURCE_TYPE:
		return auditlog.OrderByFieldResourceType
	case management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_RESOURCE_ID:
		return auditlog.OrderByFieldResourceID
	case management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_CLUSTER_ID:
		return auditlog.OrderByFieldClusterID
	case management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_ACTOR:
		return auditlog.OrderByFieldActor
	}

	return auditlog.OrderByFieldDate
}

func auditLogOrderByDir(d management.AuditLogOrderByDir) auditlog.OrderByDir {
	switch d {
	case management.AuditLogOrderByDir_AUDIT_LOG_ORDER_BY_DIR_UNSPECIFIED:
		return auditlog.OrderByDirASC
	case management.AuditLogOrderByDir_AUDIT_LOG_ORDER_BY_DIR_ASC:
		return auditlog.OrderByDirASC
	case management.AuditLogOrderByDir_AUDIT_LOG_ORDER_BY_DIR_DESC:
		return auditlog.OrderByDirDESC
	}

	return auditlog.OrderByDirASC
}

func auditLogEventType(e management.AuditLogEventType) auditlog.EventType {
	switch e {
	case management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_UNSPECIFIED:
		return auditlog.EventTypeUnspecified
	case management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_CREATE:
		return auditlog.EventTypeCreate
	case management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_UPDATE:
		return auditlog.EventTypeUpdate
	case management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_UPDATE_WITH_CONFLICTS:
		return auditlog.EventTypeUpdateWithConflicts
	case management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_DESTROY:
		return auditlog.EventTypeDestroy
	case management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_TEARDOWN:
		return auditlog.EventTypeTeardown
	case management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_TALOS_ACCESS:
		return auditlog.EventTypeTalosAccess
	case management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_K8S_ACCESS:
		return auditlog.EventTypeK8SAccess
	}

	return auditlog.EventTypeUnspecified
}
