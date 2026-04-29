// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/ssa"
	"github.com/siderolabs/go-kubernetes/kubernetes/ssa/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

func (s *managementServer) KubernetesSyncManifests(
	req *management.KubernetesSyncManifestRequest,
	srv grpc.ServerStreamingServer[management.KubernetesSyncManifestResponse],
) error {
	ctx := srv.Context()

	requestContext := router.ExtractContext(ctx)
	if requestContext == nil {
		return status.Error(codes.InvalidArgument, "unable to extract request context")
	}

	authCtx, _, err := s.checkClusterAuthorization(ctx, requestContext.Name, role.Operator)
	if err != nil {
		return err
	}

	ctx = actor.MarkContextAsInternalActor(authCtx)

	sync, err := s.prepareKubernetesSyncHelpers(ctx, authCtx, requestContext)
	if err != nil {
		return err
	}

	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, s.omniState, sync.requestContext.Name)
	if err != nil {
		return fmt.Errorf("failed to fetch cluster data: %w", err)
	}

	talosVersion := cluster.TypedSpec().Value.TalosVersion

	talosVersionCompatibility, err := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: talosVersion})
	if err != nil {
		return fmt.Errorf("failed to parse Talos version: %w", err)
	}

	syncManifests := s.syncSSA

	if !talosVersionCompatibility.SupportsSSAManifestSync() {
		syncManifests = s.syncLegacy
	}

	if err = syncManifests(ctx, req, sync, s.logger, srv); err != nil {
		return fmt.Errorf("error syncing manifests: %w", err)
	}

	if err := s.triggerManifestResync(ctx, sync.requestContext); err != nil {
		return fmt.Errorf("failed to trigger manifest resync: %w", err)
	}

	return nil
}

func (s *managementServer) syncSSA(
	ctx context.Context,
	req *management.KubernetesSyncManifestRequest,
	sync *kubernetesSyncHelpers,
	logger *zap.Logger,
	srv management.ManagementService_KubernetesSyncManifestsServer,
) error {
	if req.Ssa == nil {
		// fallback to defaults if SSA options are not provided
		req.Ssa = &management.KubernetesSSAOptions{
			InventoryPolicy: management.KubernetesSSAOptions_ADOPT_IF_NO_INVENTORY,
			NoPrune:         true,
		}
	}

	if req.Ssa.ReconcileTimeout == nil {
		req.Ssa.ReconcileTimeout = durationpb.New(time.Second * 30)
	}

	if req.DryRun {
		return s.diffSSA(ctx, req, sync, srv)
	}

	manager, err := ssa.NewManager(ctx, sync.cfg,
		constants.KubernetesFieldManagerName,
		constants.KubernetesInventoryNamespace,
		constants.KubernetesBootstrapManifestsInventoryName,
	)
	if err != nil {
		return fmt.Errorf("failed to create SSA manager: %w", err)
	}

	defer manager.Close()

	var rollouts []*management.KubernetesSyncManifestResponse

	applyOpts := ssa.ApplyOptions{
		InventoryPolicy:         getInventoryPolicy(req.Ssa.InventoryPolicy),
		NoPrune:                 req.Ssa.NoPrune,
		Force:                   req.Ssa.ForceConflicts,
		DeletePropagationPolicy: v1.DeletePropagationForeground,
		WaitTimeout:             req.Ssa.ReconcileTimeout.AsDuration(),
	}

	result, err := manager.Apply(ctx, sync.manifests, applyOpts)
	if err != nil {
		return fmt.Errorf("failed to apply manifests: %w", err)
	}

	client, err := s.kubernetesRuntime.GetClient(ctx, sync.requestContext.Name)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	for _, r := range result {
		resp := &management.KubernetesSyncManifestResponse{
			ResponseType: management.KubernetesSyncManifestResponse_MANIFEST,
			Path:         r.Subject,
		}

		if r.Action == ssa.SkippedAction {
			resp.Skipped = true
		}

		resp.Diff = r.Diff

		resp.Object, err = getObject(ctx, client, r.GroupVersion, r.ObjMetadata)
		if err != nil {
			return fmt.Errorf("failed to get object for response: %w", err)
		}

		if err = srv.Send(resp); err != nil {
			return fmt.Errorf("failed to send manifest sync response: %w", err)
		}

		switch r.Action {
		case ssa.CreatedAction, ssa.ConfiguredAction:
			if !req.DryRun && !resp.Skipped {
				rollouts = append(rollouts, &management.KubernetesSyncManifestResponse{
					ResponseType: management.KubernetesSyncManifestResponse_ROLLOUT,
					Diff:         r.Diff,
					Object:       resp.Object,
					Path:         resp.Path,
				})
			}
		}
	}

	if len(rollouts) == 0 {
		return nil
	}

	if err = cli.Wait(ctx, result, func(line string, args ...any) {
		logger.Sugar().Infof(line, args...)
	}, manager, ssa.WaitOptions{
		Timeout: req.Ssa.ReconcileTimeout.AsDuration(),
	}); err != nil {
		return fmt.Errorf("error waiting for manifest sync to complete: %w", err)
	}

	for _, rollout := range rollouts {
		if err = srv.Send(rollout); err != nil {
			return fmt.Errorf("failed to send rollout progress response: %w", err)
		}
	}

	return nil
}

func (s *managementServer) diffSSA(
	ctx context.Context,
	req *management.KubernetesSyncManifestRequest,
	sync *kubernetesSyncHelpers,
	srv management.ManagementService_KubernetesSyncManifestsServer,
) error {
	client, err := s.kubernetesRuntime.GetClient(ctx, sync.requestContext.Name)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	manager, err := ssa.NewManager(ctx, sync.cfg,
		constants.KubernetesFieldManagerName,
		constants.KubernetesInventoryNamespace,
		constants.KubernetesBootstrapManifestsInventoryName,
	)
	if err != nil {
		return fmt.Errorf("failed to create SSA manager: %w", err)
	}

	defer manager.Close()

	diffOpts := ssa.DiffOptions{
		InventoryPolicy: getInventoryPolicy(req.Ssa.InventoryPolicy),
		NoPrune:         req.Ssa.NoPrune,
		Force:           req.Ssa.ForceConflicts,
	}

	result, err := manager.Diff(ctx, sync.manifests, diffOpts)
	if err != nil {
		return fmt.Errorf("failed to diff manifests: %w", err)
	}

	for _, r := range result {
		resp := &management.KubernetesSyncManifestResponse{
			ResponseType: management.KubernetesSyncManifestResponse_MANIFEST,
			Path:         r.Subject,
			Diff:         r.Diff,
		}

		resp.Object, err = getObject(ctx, client, r.GroupVersion, r.ObjMetadata)
		if err != nil {
			return fmt.Errorf("failed to get object for response: %w", err)
		}

		if err = srv.Send(resp); err != nil {
			return fmt.Errorf("failed to send manifest diff response: %w", err)
		}
	}

	return nil
}

func (s *managementServer) syncLegacy(
	ctx context.Context,
	req *management.KubernetesSyncManifestRequest,
	sync *kubernetesSyncHelpers,
	logger *zap.Logger,
	srv management.ManagementService_KubernetesSyncManifestsServer,
) error {
	errCh := make(chan error, 1)
	synCh := make(chan manifests.SyncResult)

	panichandler.Go(func() {
		errCh <- manifests.Sync(ctx, sync.manifests, sync.cfg, req.DryRun, synCh)
	}, logger)

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

			if err = srv.Send(&management.KubernetesSyncManifestResponse{
				ResponseType: management.KubernetesSyncManifestResponse_MANIFEST,
				Path:         result.Path,
				Diff:         result.Diff,
				Skipped:      result.Skipped,
				Object:       obj,
			}); err != nil {
				return fmt.Errorf("failed to send manifest sync response: %w", err)
			}

			if !result.Skipped {
				updatedManifests = append(updatedManifests, result.Object)
			}
		}
	}

	if req.DryRun {
		// no rollout if dry run
		return nil
	}

	rolloutCh := make(chan manifests.RolloutProgress)

	panichandler.Go(func() {
		errCh <- manifests.WaitForRollout(ctx, sync.cfg, updatedManifests, rolloutCh)
	}, logger)

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

			if err = srv.Send(&management.KubernetesSyncManifestResponse{
				ResponseType: management.KubernetesSyncManifestResponse_ROLLOUT,
				Path:         result.Path,
				Object:       obj,
			}); err != nil {
				return fmt.Errorf("failed to send rollout progress response: %w", err)
			}
		}
	}

	return nil
}

type kubernetesSyncHelpers struct {
	requestContext *common.Context
	cfg            *rest.Config
	manifests      []manifests.Manifest
}

func (s *managementServer) prepareKubernetesSyncHelpers(ctx, authCtx context.Context, requestContext *common.Context) (*kubernetesSyncHelpers, error) {
	cfg, err := s.kubernetesRuntime.GetKubeconfig(ctx, requestContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	if err = s.auditTalosAccess(authCtx, management.ManagementService_KubernetesSyncManifests_FullMethodName, requestContext.Name, ""); err != nil {
		return nil, err
	}

	talosClient, err := s.talosRuntime.GetClientForCluster(ctx, requestContext.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get talos client: %w", err)
	}

	bootstrapManifests, err := manifests.GetBootstrapManifests(ctx, talosClient.COSI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifests: %w", err)
	}

	return &kubernetesSyncHelpers{
		requestContext: requestContext,
		cfg:            cfg,
		manifests:      bootstrapManifests,
	}, nil
}

func getObject(ctx context.Context, client *kubernetes.Client, groupVersion string, metadata object.ObjMetadata) ([]byte, error) {
	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse group version: %w", err)
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   metadata.GroupKind.Group,
		Version: gv.Version,
		Kind:    metadata.GroupKind.Kind,
	})
	u.SetNamespace(metadata.Namespace)

	dr, err := client.Resource(u)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic resource interface: %w", err)
	}

	obj, err := dr.Get(ctx, metadata.Name, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get resource %s/%s: %w", metadata.Namespace, metadata.Name, err)
	}

	return yaml.Marshal(obj)
}

func getInventoryPolicy(inventoryPolicy management.KubernetesSSAOptions_InventoryPolicy) ssa.InventoryPolicy {
	switch inventoryPolicy {
	case management.KubernetesSSAOptions_ADOPT_IF_NO_INVENTORY:
		return ssa.InventoryPolicyAdoptIfNoInventory
	case management.KubernetesSSAOptions_ADOPT_ALL:
		return ssa.InventoryPolicyAdoptAll
	case management.KubernetesSSAOptions_MUST_MATCH:
		return ssa.InventoryPolicyMustMatch
	default:
		return ssa.InventoryPolicyAdoptIfNoInventory
	}
}
