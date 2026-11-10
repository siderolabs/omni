// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	oidcres "github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/infraprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/authz"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// PublicResourceTypes is the set of resource types that are accessible without any authentication.
// These are intentionally public and must remain readable before auth.
var PublicResourceTypes = xslices.ToSet([]resource.Type{
	authres.AuthConfigType,
})

// authorizationValidationOptions returns the validation options responsible for all authorization checks.
//
// These include the regular checks on the user's Omni-wide role, as well as the ACLs that can authorize the user to perform additional actions on specific clusters.
func authorizationValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithListValidations(
			func(ctx context.Context, kind resource.Kind, opt ...state.ListOption) error {
				var opts state.ListOptions

				for _, o := range opt {
					o(&opts)
				}

				if len(opts.LabelQueries) == 0 {
					return checkForKindAccess(ctx, st, state.List, kind, nil)
				}

				for _, query := range opts.LabelQueries {
					if err := checkForKindAccess(ctx, st, state.List, kind, query.Terms); err != nil {
						return err
					}
				}

				return nil
			},
		),
		validated.WithWatchKindValidations(
			func(ctx context.Context, kind resource.Kind, opt ...state.WatchKindOption) error {
				var opts state.WatchKindOptions

				for _, o := range opt {
					o(&opts)
				}

				if len(opts.LabelQueries) == 0 {
					return checkForKindAccess(ctx, st, state.Watch, kind, nil)
				}

				for _, query := range opts.LabelQueries {
					if err := checkForKindAccess(ctx, st, state.Watch, kind, query.Terms); err != nil {
						return err
					}
				}

				return nil
			},
		),
		validated.WithGetValidations(
			func(ctx context.Context, ptr resource.Pointer, res resource.Resource, _ ...state.GetOption) error {
				if res == nil {
					// resource does not exist, decide whether to return NotFound or PermissionDenied based on the pointer
					clusterID := clusterIDFromPointer(ptr)

					return checkForRole(ctx, st, state.Access{
						ResourceNamespace: ptr.Namespace(),
						ResourceType:      ptr.Type(),
						ResourceID:        ptr.ID(),
						Verb:              state.Get,
					}, clusterID, false)
				}

				clusterID := clusterIDFromMetadata(res.Metadata())

				return checkForRole(ctx, st, state.Access{
					ResourceNamespace: res.Metadata().Namespace(),
					ResourceType:      res.Metadata().Type(),
					ResourceID:        res.Metadata().ID(),
					Verb:              state.Get,
				}, clusterID, false)
			},
			labelsCompletionValidation(st),
		),
		validated.WithWatchValidations(
			func(ctx context.Context, ptr resource.Pointer, _ ...state.WatchOption) error {
				// todo: watch validation here only works for resources that have same ID as Cluster type,
				// not for the ones that are related over a label.
				// we should improve this by checking/filtering the labels for the cluster.
				clusterID := clusterIDFromPointer(ptr)

				return checkForRole(ctx, st, state.Access{
					ResourceNamespace: ptr.Namespace(),
					ResourceType:      ptr.Type(),
					ResourceID:        ptr.ID(),
					Verb:              state.Watch,
				}, clusterID, false)
			},
		),
		validated.WithCreateValidations(
			func(ctx context.Context, res resource.Resource, _ ...state.CreateOption) error {
				clusterID := clusterIDFromMetadata(res.Metadata())

				return checkForRole(ctx, st, state.Access{
					ResourceNamespace: res.Metadata().Namespace(),
					ResourceType:      res.Metadata().Type(),
					ResourceID:        res.Metadata().ID(),
					Verb:              state.Create,
				}, clusterID, false)
			},
		),
		validated.WithUpdateValidations(
			func(ctx context.Context, existingRes resource.Resource, newRes resource.Resource, _ ...state.UpdateOption) error {
				if existingRes == nil {
					// resource does not exist, decide whether to return NotFound or PermissionDenied based on the pointer
					newClusterID := clusterIDFromMetadata(newRes.Metadata())

					return checkForRole(ctx, st, state.Access{
						ResourceNamespace: newRes.Metadata().Namespace(),
						ResourceType:      newRes.Metadata().Type(),
						ResourceID:        newRes.Metadata().ID(),
						Verb:              state.Update,
					}, newClusterID, false)
				}

				existingClusterID := clusterIDFromMetadata(existingRes.Metadata())

				if err := checkForRole(ctx, st, state.Access{
					ResourceNamespace: existingRes.Metadata().Namespace(),
					ResourceType:      existingRes.Metadata().Type(),
					ResourceID:        existingRes.Metadata().ID(),
					Verb:              state.Update,
				}, existingClusterID, false); err != nil {
					return err
				}

				newClusterID := clusterIDFromMetadata(newRes.Metadata())

				// if this is a cluster-related resource and the ID has not changed, we don't need to do the same check again
				if newClusterID != "" && existingClusterID == newClusterID {
					return nil
				}

				return checkForRole(ctx, st, state.Access{
					ResourceNamespace: newRes.Metadata().Namespace(),
					ResourceType:      newRes.Metadata().Type(),
					ResourceID:        newRes.Metadata().ID(),
					Verb:              state.Update,
				}, newClusterID, false)
			},
		),
		validated.WithDestroyValidations(
			func(ctx context.Context, ptr resource.Pointer, res resource.Resource, _ ...state.DestroyOption) error {
				if res == nil {
					// resource does not exist, decide whether to return NotFound or PermissionDenied based on the pointer
					clusterID := clusterIDFromPointer(ptr)

					return checkForRole(ctx, st, state.Access{
						ResourceNamespace: ptr.Namespace(),
						ResourceType:      ptr.Type(),
						ResourceID:        ptr.ID(),
						Verb:              state.Destroy,
					}, clusterID, false)
				}

				clusterID := clusterIDFromMetadata(res.Metadata())

				return checkForRole(ctx, st, state.Access{
					ResourceNamespace: res.Metadata().Namespace(),
					ResourceType:      res.Metadata().Type(),
					ResourceID:        res.Metadata().ID(),
					Verb:              state.Destroy,
				}, clusterID, false)
			},
		),
	}
}

// labelsCompletionValidation requires list access to the type a LabelsCompletion is for, as it is built from all resources of that type.
func labelsCompletionValidation(st state.State) validated.GetValidation {
	return func(ctx context.Context, ptr resource.Pointer, _ resource.Resource, _ ...state.GetOption) error {
		if ptr.Type() != virtual.LabelsCompletionType {
			return nil
		}

		return checkForKindAccess(ctx, st, state.List, resource.NewMetadata(resources.DefaultNamespace, ptr.ID(), "", resource.VersionUndefined), nil)
	}
}

func checkForRole(ctx context.Context, st state.State, access state.Access, clusterID resource.ID, requireAll bool) error {
	if actor.ContextIsInternalActor(ctx) {
		return nil
	}

	if requireAll {
		clusterID = "any"
	}

	if clusterID != "" {
		clusterRole, matchesAll, err := accesspolicy.RoleForCluster(ctx, clusterID, st)
		if err != nil {
			return err
		}

		if clusterRole != role.None && (!requireAll || matchesAll) {
			// override the role in the context with the computed role for this cluster
			ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: clusterRole})
		}
	}

	return filterAccess(ctx, access)
}

func checkForKindAccess(ctx context.Context, st state.State, verb state.Verb, kind resource.Kind, labelTerms []resource.LabelTerm) error {
	clusterID := ""
	requireAll := false

	if isClusterRelatedType(kind.Type()) {
		// terms that do not name a single cluster leave the ID empty, which requires access to all clusters
		if id, err := labels.ExactValue(labelTerms, omni.LabelCluster); err == nil {
			clusterID = id
		}

		requireAll = clusterID == ""
	}

	return checkForRole(ctx, st, state.Access{
		ResourceNamespace: kind.Namespace(),
		ResourceType:      kind.Type(),
		Verb:              verb,
	}, clusterID, requireAll)
}

func isClusterRelatedType(typ resource.Type) bool {
	return authz.LookupResourceType(typ).Cluster != authz.NoCluster
}

func clusterIDFromMetadata(resMD *resource.Metadata) resource.ID {
	switch authz.LookupResourceType(resMD.Type()).Cluster {
	case authz.ClusterByID:
		return resMD.ID()
	case authz.ClusterByLabel:
		cluster, _ := resMD.Labels().Get(omni.LabelCluster)

		return cluster
	case authz.NoCluster:
	}

	return ""
}

func clusterIDFromPointer(ptr resource.Pointer) resource.ID {
	if authz.LookupResourceType(ptr.Type()).Cluster == authz.ClusterByID {
		return ptr.ID()
	}

	return ""
}

func verbToRole(verb state.Verb) role.Role {
	switch verb {
	case state.Create, state.Update, state.Destroy:
		return role.Operator
	case state.Get, state.List, state.Watch:
		return role.Reader
	default:
		panic(fmt.Sprintf("unknown verb: %v", verb))
	}
}

// filterAccess provides a filter to exclude some resources and operations from external sources.
func filterAccess(ctx context.Context, access state.Access) error {
	if actor.ContextIsInternalActor(ctx) {
		return nil
	}

	// check if the resource is an infra provider resource - if it is, the access is managed in infraprovider.State
	if infraprovider.IsInfraProviderResource(access.ResourceNamespace, access.ResourceType) {
		return nil
	}

	// Public resource types are accessible without any authentication.
	if _, ok := PublicResourceTypes[access.ResourceType]; ok {
		return filterAccessByType(access)
	}

	var err error

	// authentication and authorization checks
	switch access.ResourceType {
	case omni.MachineType, // infra provider needs to be able to read machines to find out force-deleted ones and deprovision them
		siderolink.APIConfigType,        // infra provider needs to be able to read connection params to join nodes to Omni
		siderolink.ConnectionParamsType: // infra provider needs to be able to read connection params to join nodes to Omni
		_, err = auth.CheckGRPC(ctx, auth.WithRole(role.InfraProvider))
	case
		omni.ClusterType,
		omni.ClusterBootstrapStatusType,
		omni.ClusterConfigVersionType,
		omni.ClusterDestroyStatusType,
		omni.ClusterEndpointType,
		omni.ClusterKubernetesNodesType,
		omni.ClusterMachineIdentityType,
		omni.ClusterMachineStatusType,
		omni.ClusterMachineRequestStatusType,
		omni.ClusterMachineTalosVersionType,
		omni.ClusterMachineType,
		omni.ClusterMachineConfigPatchesType,
		omni.ClusterMachineConfigStatusType,
		omni.ClusterStatusType,
		omni.ClusterDiagnosticsType,
		omni.ClusterUUIDType,
		omni.ClusterWorkloadProxyStatusType,
		omni.ClusterTaintType,
		omni.ClusterKubernetesManifestsStatusType,
		omni.ConfigPatchType,
		omni.ControlPlaneStatusType,
		omni.DiscoveryAffiliateDeleteTaskType,
		omni.KubernetesNodeAuditResultType,
		omni.ExposedServiceType,
		omni.EtcdBackupType,
		omni.EtcdBackupStatusType,
		omni.EtcdBackupStoreStatusType,
		omni.EtcdBackupOverallStatusType,
		omni.EtcdManualBackupType,
		omni.ImagePullRequestType,
		omni.ImagePullStatusType,
		omni.InfraMachineConfigType,
		omni.KubernetesStatusType,
		omni.KubernetesUpgradeManifestStatusType,
		omni.KubernetesUpgradeStatusType,
		omni.KubernetesManifestGroupType,
		omni.LoadBalancerConfigType,
		omni.LoadBalancerStatusType,
		omni.MachineLabelsType,
		omni.MachineSetType,
		omni.MachineSetDestroyStatusType,
		omni.MachineSetNodeType,
		omni.MachineRequestSetStatusType,
		omni.MachineRequestSetType,
		omni.MachineSetStatusType,
		omni.MaintenanceConfigStatusType,
		omni.MachineConfigExtractionStatusType,
		omni.NodeForceDestroyRequestType,
		omni.TalosUpgradeStatusType,
		omni.RedactedClusterMachineConfigType,
		omni.MachineConfigDiffType,
		siderolink.LinkType,
		siderolink.PendingMachineType,
		siderolink.LinkStatusType,
		siderolink.JoinTokenStatusType,
		siderolink.JoinTokenType,
		siderolink.DefaultJoinTokenType,
		siderolink.NodeUniqueTokenStatusType,
		siderolink.GRPCTunnelConfigType,
		omni.MachineClassType,
		omni.MachineExtensionsStatusType,
		omni.MachineExtensionsType,
		omni.MachineInstallDiskConfigType,
		omni.MachineInstallDiskStatusType,
		omni.MachineStatusType,
		omni.MachineStatusSnapshotType,
		omni.MachineStatusLinkType,
		omni.MachineConfigGenOptionsType,
		omni.MachinePendingUpdatesType,
		omni.SchematicConfigurationType,
		omni.InfraProviderCombinedStatusType,
		omni.InstallationMediaConfigType,
		omni.ExtensionsConfigurationType,
		omni.MachineUpgradeStatusType,
		omni.KernelArgsType,
		omni.KernelArgsStatusType,
		omni.ClusterSecretsRotationStatusType,
		omni.RotateTalosCAType,
		omni.RotateKubernetesCAType,
		omni.UpgradeRolloutType,
		omni.KubernetesHealthCheckType,
		omni.KubernetesHealthCheckStatusType,
		system.ResourceLabelsType[*omni.MachineStatus](),
		virtual.SupportType,
		virtual.LabelsCompletionType,
		virtual.SBCConfigType,
		virtual.CloudPlatformConfigType,
		virtual.MetalPlatformConfigType,
		virtual.KubernetesUsageType:
		_, err = auth.CheckGRPC(ctx, auth.WithRole(verbToRole(access.Verb)))

		if err == nil && access.Verb == state.Create && access.ResourceType == siderolink.JoinTokenType {
			err = status.Error(codes.PermissionDenied, "only read, update and destroy access is permitted, create should be done via the management.CreateJoinToken API call")
		}
	case
		meta.NamespaceType,
		meta.ResourceDefinitionType,
		omni.FeaturesConfigType,
		omni.TalosExtensionsType,
		omni.TalosVersionType,
		omni.KubernetesVersionType,
		authres.EulaAcceptanceType,
		omni.InstallationMediaType,
		omni.OngoingTaskType,
		omni.MachineStatusMetricsType,
		omni.NotificationType,
		omni.ClusterMetricsType,
		omni.ClusterStatusMetricsType,
		system.SysVersionType,
		virtual.AdvertisedEndpointsType,
		virtual.CurrentUserType,
		virtual.ClusterPermissionsType,
		virtual.QuirksType,
		virtual.VersionContractType,
		virtual.PermissionsType:
		// allow access with just valid signature
		_, err = auth.CheckGRPC(ctx, auth.WithValidSignature(true))
	case // admin-only resources
		authres.IdentityType,
		authres.IdentityLastActiveType,
		authres.IdentityStatusType,
		authres.PublicKeyLastActiveType,
		authres.UserType,
		authres.ServiceAccountStatusType,
		authres.IdentityLabelRuleType,
		authres.AccessPolicyType,
		omni.EtcdBackupS3ConfType,
		omni.InfraMachineBMCConfigType,
		oidcres.JWTPublicKeyType:
		_, err = auth.CheckGRPC(ctx, auth.WithRole(role.Admin))

		// Restrict direct mutations on Identity and User resources — these must go through ManagementService gRPC endpoints.
		if err == nil && !access.Verb.Readonly() && (access.ResourceType == authres.IdentityType || access.ResourceType == authres.UserType) {
			err = status.Errorf(codes.PermissionDenied, "only read access is permitted on resource %v, mutations should be done via ManagementService API calls", access.ResourceType)
		}
	case omni.ImportedClusterSecretsType:
		_, err = auth.CheckGRPC(ctx, auth.WithRole(role.Operator))
	default:
		err = status.Error(codes.PermissionDenied, "no access is permitted")
	}

	if err != nil {
		return err
	}

	// authorization checks by access type
	return filterAccessByType(access)
}

// filterAccessByType refuses the verbs no caller of the API is allowed on the resource type.
func filterAccessByType(access state.Access) error {
	verbs := authz.LookupResourceType(access.ResourceType).Verbs

	switch {
	case verbs.Has(access.Verb):
		return nil
	case verbs == 0:
		return status.Error(codes.PermissionDenied, "access is not permitted")
	default:
		return status.Errorf(codes.PermissionDenied, "only %s access is permitted", verbs)
	}
}
