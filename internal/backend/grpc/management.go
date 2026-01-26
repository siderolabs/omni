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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests/event"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	talosconsts "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cli-utils/pkg/inventory"

	commonOmni "github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client/helpers"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	ctlcfg "github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/installimage"
	"github.com/siderolabs/omni/internal/backend/runtime"
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
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	siderolinkinternal "github.com/siderolabs/omni/internal/pkg/siderolink"
)

type talosRuntime interface {
	GetClient(ctx context.Context, clusterName string) (*talos.Client, error)
}

type kubernetesRuntime interface {
	GetKubeconfig(ctx context.Context, cluster *commonOmni.Context) (*rest.Config, error)
}

// JWTSigningKeyProvider is an interface for a JWT signing key provider.
type JWTSigningKeyProvider interface {
	SigningKey(ctx context.Context) (op.SigningKey, error)
}

func newManagementServer(omniState state.State, jwtSigningKeyProvider JWTSigningKeyProvider, logHandler *siderolinkinternal.LogHandler, logger *zap.Logger,
	dnsService *dns.Service, imageFactoryClient *imagefactory.Client, auditor AuditLogger, omniconfigDest string,
) *managementServer {
	return &managementServer{
		omniState:             omniState,
		jwtSigningKeyProvider: jwtSigningKeyProvider,
		logHandler:            logHandler,
		logger:                logger,
		dnsService:            dnsService,
		imageFactoryClient:    imageFactoryClient,
		auditor:               auditor,
		omniconfigDest:        omniconfigDest,
	}
}

// managementServer implements omni management service.
type managementServer struct {
	management.UnimplementedManagementServiceServer

	omniState             state.State
	jwtSigningKeyProvider JWTSigningKeyProvider

	logHandler         *siderolinkinternal.LogHandler
	logger             *zap.Logger
	dnsService         *dns.Service
	imageFactoryClient *imagefactory.Client
	auditor            AuditLogger
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
			"grant-type=" + req.GrantType,
			"oidc-redirect-url=urn:ietf:wg:oauth:2.0:oob",
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

func (s *managementServer) MachineLogs(request *management.MachineLogsRequest, serv grpc.ServerStreamingServer[common.Data]) error {
	ctx := serv.Context()

	// getting machine logs is equivalent to reading machine resource
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
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

	logReader, err := s.logHandler.GetReader(ctx, siderolinkinternal.MachineID(machineID), request.Follow, tailLines)
	if err != nil {
		return handleError(err)
	}

	defer logReader.Close() //nolint:errcheck

	for {
		line, err := logReader.ReadLine(ctx)
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

			return nil
		},
		state.WithUpdateOwner(omniCtrl.ClusterStatusControllerName),
	); err != nil {
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
	if !constants.IsDebugBuild && !config.Config.Features.GetEnableBreakGlassConfigs() {
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

	if !constants.IsDebugBuild && !config.Config.Features.GetEnableBreakGlassConfigs() {
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

func (s *managementServer) KubernetesUpgradePreChecks(ctx context.Context, req *management.KubernetesUpgradePreChecksRequest) (*management.KubernetesUpgradePreChecksResponse, error) {
	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	requestContext := router.ExtractContext(ctx)
	if requestContext == nil {
		return nil, status.Error(codes.InvalidArgument, "unable to extract request context")
	}

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

	k8sRuntime, err := runtime.LookupInterface[kubernetesRuntime](kubernetes.Name)
	if err != nil {
		return nil, err
	}

	restConfig, err := k8sRuntime.GetKubeconfig(ctx, requestContext)
	if err != nil {
		return nil, fmt.Errorf("error getting kubeconfig: %w", err)
	}

	talosRt, err := runtime.LookupInterface[talosRuntime](talos.Name)
	if err != nil {
		return nil, err
	}

	talosClient, err := talosRt.GetClient(ctx, requestContext.Name)
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

	for cmi := range cmis.All() {
		if len(cmi.TypedSpec().Value.NodeIps) > 0 {
			controlplaneNodes = append(controlplaneNodes, cmi.TypedSpec().Value.NodeIps[0])
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

func (s *managementServer) KubernetesDiffManifests(ctx context.Context, req *management.KubernetesSSAOptions) (*management.KubernetesBootstrapManifestDiffResponseList, error) {
	ctx, _, kubeconfig, bootstrapManifests, err := s.prepareKubernetesSyncHelpers(ctx)
	if err != nil {
		return nil, err
	}

	ssaOps := getBootstrapManifestsProtoSSAOptions(req)

	result, err := manifests.DiffSSA(ctx, bootstrapManifests, kubeconfig, ssaOps)
	if err != nil {
		return nil, err
	}

	diffResultList := management.KubernetesBootstrapManifestDiffResponseList{}

	for _, r := range result {
		diffItem, err := helpers.ConvertDiffResultToProto(r)
		if err != nil {
			return nil, err
		}

		diffResultList.Diffs = append(diffResultList.Diffs, diffItem)
	}

	return &diffResultList, nil
}

func getBootstrapManifestsProtoSSAOptions(req *management.KubernetesSSAOptions) manifests.SSAOptions {
	ssaOps := getProtoSSAOptions(req, constants.KubernetesFieldManagerName, talosconsts.KubernetesInventoryNamespace, talosconsts.KubernetesBootstrapManifestsInventoryName)

	return ssaOps
}

func (s *managementServer) KubernetesSyncManifestsSSA(
	req *management.KubernetesSSAOptions,
	srv grpc.ServerStreamingServer[management.KubernetesBootstrapManifestSyncResponse],
) error {
	ctx, _, kubeconfig, bootstrapManifests, err := s.prepareKubernetesSyncHelpers(srv.Context())
	if err != nil {
		return err
	}

	ssaOps := getBootstrapManifestsProtoSSAOptions(req)

	syncCh := make(chan event.Event)
	errCh := make(chan error, 1)

	go func() {
		errCh <- manifests.SyncSSA(ctx, bootstrapManifests, kubeconfig, syncCh, ssaOps)
	}()

syncLoop:
	for {
		select {
		case e := <-syncCh:
			resp, err1 := helpers.ConvertSyncEventToProto(e)
			if err1 != nil {
				return err1
			}

			if err := srv.Send(resp); err != nil {
				return err
			}

		case err := <-errCh:
			if err == nil {
				break syncLoop
			}

			return err
		}
	}

	return nil
}

func getProtoSSAOptions(req *management.KubernetesSSAOptions, fieldManagerName, inventoryNS, inventoryName string) manifests.SSAOptions {
	ssaOps := manifests.SSAOptions{
		FieldManagerName:   fieldManagerName,
		InventoryNamespace: inventoryNS,
		InventoryName:      inventoryName,
		SSApplyBehaviorOptions: manifests.SSApplyBehaviorOptions{
			ReconcileTimeout: req.ReconcileTimeout.AsDuration(),
			PruneTimeout:     req.PruneTimeout.AsDuration(),
			ForceConflicts:   req.ForceConflicts,
			DryRun:           req.DryRun,
			NoPrune:          req.NoPrune,
		},
	}

	switch req.InventoryPolicy {
	case management.KubernetesSSAOptions_ADOPT_ALL:
		ssaOps.InventoryPolicy = inventory.PolicyAdoptAll
	case management.KubernetesSSAOptions_ADOPT_IF_NO_INVENTORY:
		ssaOps.InventoryPolicy = inventory.PolicyAdoptIfNoInventory
	case management.KubernetesSSAOptions_MUST_MATCH:
		ssaOps.InventoryPolicy = inventory.PolicyMustMatch
	}

	return ssaOps
}

//nolint:gocognit,gocyclo,cyclop
func (s *managementServer) KubernetesSyncManifests(req *management.KubernetesSyncManifestRequest, srv grpc.ServerStreamingServer[management.KubernetesSyncManifestResponse]) error {
	ctx, requestContext, cfg, bootstrapManifests, err := s.prepareKubernetesSyncHelpers(srv.Context())
	if err != nil {
		return err
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

func (s *managementServer) prepareKubernetesSyncHelpers(serverStreamingServerCtx context.Context) (
	context.Context,
	*commonOmni.Context,
	*rest.Config,
	[]manifests.Manifest,
	error,
) {
	ctx := serverStreamingServerCtx

	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return nil, nil, nil, nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	requestContext := router.ExtractContext(ctx)
	if requestContext == nil {
		return nil, nil, nil, nil, status.Error(codes.InvalidArgument, "unable to extract request context")
	}

	k8sRuntime, err := runtime.LookupInterface[kubernetesRuntime](kubernetes.Name)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cfg, err := k8sRuntime.GetKubeconfig(ctx, requestContext)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	talosRt, err := runtime.LookupInterface[talosRuntime](talos.Name)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	talosClient, err := talosRt.GetClient(ctx, requestContext.Name)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get talos client: %w", err)
	}

	bootstrapManifests, err := manifests.GetBootstrapManifests(ctx, talosClient.COSI, nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get manifests: %w", err)
	}

	return ctx, requestContext, cfg, bootstrapManifests, nil
}

// ReadAuditLog reads the audit log from the backend.
func (s *managementServer) ReadAuditLog(req *management.ReadAuditLogRequest, srv grpc.ServerStreamingServer[management.ReadAuditLogResponse]) error {
	ctx := srv.Context()

	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Admin)); err != nil {
		return err
	}

	now := time.Now()

	start, err := parseTime(req.GetStartTime(), now.AddDate(0, 0, -29))
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid start time: %v", err)
	}

	end, err := parseTime(req.GetEndTime(), now)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid end time: %v", err)
	}

	rdr, err := s.auditor.Reader(ctx, start, end)
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

	if _, err := s.authCheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return nil, err
	}

	if req.MachineId == "" {
		return nil, status.Error(codes.InvalidArgument, "machine id is required")
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

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

	schematic := machineStatus.TypedSpec().Value.Schematic
	if schematic == nil || schematic.FullId == "" {
		return nil, status.Error(codes.FailedPrecondition, "machine schematic is not known yet")
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
		SchematicId:          schematic.FullId,
		SchematicInitialized: true,
		Platform:             platform,
		SecurityState:        securityState,
	}

	installImageStr, err := installimage.Build(s.imageFactoryClient.Host(), req.MachineId, installImage)
	if err != nil {
		return nil, fmt.Errorf("failed to build install image: %w", err)
	}

	s.logger.Info("built maintenance upgrade image", zap.String("image", installImageStr))

	address := machineStatus.TypedSpec().Value.ManagementAddress
	opts := talos.GetSocketOptions(address)
	opts = append(opts, client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), client.WithEndpoints(address))

	talosClient, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create talos client: %w", err)
	}

	if _, err = talosClient.UpgradeWithOptions(ctx, client.WithUpgradeImage(installImageStr)); err != nil {
		return nil, fmt.Errorf("failed to upgrade machine: %w", err)
	}

	return &management.MaintenanceUpgradeResponse{}, nil
}

func (s *managementServer) GetMachineJoinConfig(ctx context.Context, request *management.GetMachineJoinConfigRequest) (*management.GetMachineJoinConfigResponse, error) {
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

//nolint:unparam
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

	userRole := role.None

	if val, ok := ctxstore.Value[auth.RoleContextKey](ctx); ok {
		userRole = val.Role
	}

	newRole, err := role.Max(userRole, clusterRole)
	if err != nil {
		return nil, err
	}

	if newRole == userRole {
		return ctx, nil
	}

	return ctxstore.WithValue(ctx, auth.RoleContextKey{Role: newRole}), nil
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

// AuditLogger is an interface for reading the audit log.
type AuditLogger interface {
	Reader(ctx context.Context, start, end time.Time) (auditlog.Reader, error)
}
