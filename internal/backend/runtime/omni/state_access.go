// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

var (
	// clusterIDTypeSet is the set of resource types which have the related cluster's ID as their ID.
	clusterIDTypeSet = xslices.ToSet([]resource.Type{
		omni.ClusterBootstrapStatusType,
		omni.ClusterConfigVersionType,
		omni.ClusterDestroyStatusType,
		omni.ClusterEndpointType,
		omni.ClusterWorkloadProxyStatusType,
		omni.ClusterKubernetesNodesType,
		omni.ClusterTaintType,
		omni.ClusterType,
		omni.ClusterUUIDType,
		omni.ClusterSecretsType,
		omni.ClusterStatusType,
		omni.EtcdAuditResultType,
		omni.EtcdBackupStatusType,
		omni.EtcdManualBackupType,
	})

	// clusterLabelTypeSet is the set of resource types which have the related cluster's ID as a label.
	clusterLabelTypeSet = xslices.ToSet([]resource.Type{
		omni.ClusterMachineConfigType,
		omni.ClusterMachineConfigStatusType,
		omni.ClusterMachineIdentityType,
		omni.ClusterMachineType,
		omni.ClusterMachineConfigPatchesType,
		omni.ClusterMachineConfigStatusType,
		omni.ClusterMachineTalosVersionType,
		omni.ClusterMachineTemplateType,
		omni.ConfigPatchType,
		omni.ExposedServiceType,
		omni.ImagePullRequestType,
		omni.ImagePullStatusType,
		omni.MachineSetType,
		omni.MachineSetRequiredMachinesType,
		omni.MachineSetStatusType,
		omni.MachineSetNodeType,
		omni.EtcdBackupType,
		omni.SchematicConfigurationType,
		omni.ExtensionsConfigurationType,
		omni.MachineExtensionsStatusType,
		omni.MachineExtensionsType,
		omni.ExtensionsConfigurationStatusType,
	})

	// userManagedResourceTypeSet is the set of resource types that are managed by the user.
	userManagedResourceTypeSet = xslices.ToSet(common.UserManagedResourceTypes)
)

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
		clusterID = clusterIDFromLabelTerms(labelTerms)

		requireAll = clusterID == ""
	}

	return checkForRole(ctx, st, state.Access{
		ResourceNamespace: kind.Namespace(),
		ResourceType:      kind.Type(),
		Verb:              verb,
	}, clusterID, requireAll)
}

func isClusterRelatedType(typ resource.Type) bool {
	_, ok := clusterIDTypeSet[typ]
	if ok {
		return true
	}

	_, ok = clusterLabelTypeSet[typ]

	return ok
}

func clusterIDFromMetadata(resMD *resource.Metadata) resource.ID {
	if _, ok := clusterIDTypeSet[resMD.Type()]; ok {
		return resMD.ID()
	}

	if _, ok := clusterLabelTypeSet[resMD.Type()]; ok {
		cluster, _ := resMD.Labels().Get(omni.LabelCluster)

		return cluster
	}

	return ""
}

func clusterIDFromLabelTerms(labelTerms []resource.LabelTerm) resource.ID {
	for _, term := range labelTerms {
		if term.Key == omni.LabelCluster && term.Op == resource.LabelOpEqual {
			return term.Value[0]
		}
	}

	return ""
}

func clusterIDFromPointer(ptr resource.Pointer) resource.ID {
	if _, ok := clusterIDTypeSet[ptr.Type()]; ok {
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
		panic(fmt.Sprintf("unknown verb %q", verb))
	}
}

// filterAccess provides a filter to exclude some resources and operations from external sources.
//
//nolint:cyclop,gocyclo
func filterAccess(ctx context.Context, access state.Access) error {
	if actor.ContextIsInternalActor(ctx) {
		return nil
	}

	// check if the resource is a cloud provider resource - if it is, the access is managed in cloudprovider.State
	if strings.HasPrefix(access.ResourceNamespace, resources.CloudProviderSpecificNamespacePrefix) || access.ResourceNamespace == resources.CloudProviderNamespace {
		return nil
	}

	var err error

	// authentication and authorization checks
	switch access.ResourceType {
	case omni.MachineType, // cloud provider needs to be able to read machines to find out force-deleted ones and deprovision them
		siderolink.ConnectionParamsType: // cloud provider needs to be able to read connection params to join nodes to Omni
		_, err = auth.CheckGRPC(ctx, auth.WithRole(role.CloudProvider))
	case
		omni.ClusterType,
		omni.ClusterBootstrapStatusType,
		omni.ClusterDestroyStatusType,
		omni.ClusterEndpointType,
		omni.ClusterKubernetesNodesType,
		omni.ClusterMachineIdentityType,
		omni.ClusterMachineStatusType,
		omni.ClusterMachineTalosVersionType,
		omni.ClusterMachineType,
		omni.ClusterMachineConfigPatchesType,
		omni.ClusterMachineTemplateType,
		omni.ClusterStatusType,
		omni.ClusterUUIDType,
		omni.ClusterWorkloadProxyStatusType,
		omni.ClusterTaintType,
		omni.ConfigPatchType,
		omni.ControlPlaneStatusType,
		omni.KubernetesNodeAuditResultType,
		omni.ExposedServiceType,
		omni.EtcdBackupType,
		omni.EtcdBackupStatusType,
		omni.EtcdBackupStoreStatusType,
		omni.EtcdBackupOverallStatusType,
		omni.EtcdManualBackupType,
		omni.ImagePullRequestType,
		omni.ImagePullStatusType,
		omni.KubernetesStatusType,
		omni.KubernetesUpgradeManifestStatusType,
		omni.KubernetesUpgradeStatusType,
		omni.LoadBalancerConfigType,
		omni.LoadBalancerStatusType,
		omni.MachineLabelsType,
		omni.MachineSetType,
		omni.MachineSetDestroyStatusType,
		omni.MachineSetRequiredMachinesType,
		omni.MachineSetNodeType,
		omni.MachineSetStatusType,
		omni.TalosUpgradeStatusType,
		omni.RedactedClusterMachineConfigType,
		siderolink.LinkType,
		omni.MachineClassType,
		omni.MachineClassStatusType,
		omni.MachineExtensionsStatusType,
		omni.MachineExtensionsType,
		omni.MachineStatusType,
		omni.MachineStatusSnapshotType,
		omni.MachineStatusLinkType,
		omni.MachineConfigGenOptionsType,
		omni.SchematicType,
		omni.SchematicConfigurationType,
		omni.ExtensionsConfigurationType,
		omni.ExtensionsConfigurationStatusType,
		virtual.LabelsCompletionType,
		virtual.KubernetesUsageType:
		_, err = auth.CheckGRPC(ctx, auth.WithRole(verbToRole(access.Verb)))
	case
		meta.NamespaceType,
		meta.ResourceDefinitionType,
		omni.FeaturesConfigType,
		omni.TalosExtensionsType,
		omni.TalosVersionType,
		omni.KubernetesVersionType,
		omni.InstallationMediaType,
		omni.OngoingTaskType,
		omni.MachineStatusMetricsType,
		omni.ClusterStatusMetricsType,
		system.SysVersionType,
		virtual.CurrentUserType,
		virtual.ClusterPermissionsType,
		virtual.PermissionsType:
		// allow access with just valid signature
		_, err = auth.CheckGRPC(ctx, auth.WithValidSignature(true))
	case authres.IdentityType, authres.UserType, authres.SAMLLabelRuleType, authres.AccessPolicyType, omni.EtcdBackupS3ConfType:
		var checkResult auth.CheckResult
		// user management access
		checkResult, err = auth.CheckGRPC(ctx, auth.WithRole(role.Admin))

		if err == nil && (access.Verb == state.Destroy || access.Verb == state.Update) {
			if access.ResourceType == authres.IdentityType && checkResult.Identity == access.ResourceID {
				err = status.Errorf(codes.PermissionDenied, "destroying/updating resource %s is not allowed by the current user", access.ResourceID)
			}

			if access.ResourceType == authres.UserType && checkResult.UserID == access.ResourceID {
				err = status.Errorf(codes.PermissionDenied, "destroying/updating resource %s is not allowed by the current user", access.ResourceID)
			}
		}
	case authres.AuthConfigType:
		// allow access even without auth
	default:
		err = status.Error(codes.PermissionDenied, "no access is permitted")
	}

	if err != nil {
		return err
	}

	if config.Config.Auth.SAML.Enabled {
		switch access.ResourceType {
		case authres.UserType:
			// If SAML is enabled only enable read, update and destroy on User resources.
			if access.Verb.Readonly() || access.Verb == state.Update || access.Verb == state.Destroy {
				return nil
			}

			return status.Error(codes.PermissionDenied, "only read and destroy access is permitted")
		case authres.IdentityType:
			// If SAML is enabled only enable read, update and destroy on Identity resources.
			if access.Verb.Readonly() || access.Verb == state.Update || access.Verb == state.Destroy {
				return nil
			}

			return status.Error(codes.PermissionDenied, "only read and destroy access is permitted")
		}
	}

	// authorization checks by access type
	return filterAccessByType(access)
}

// filterAccessByType provides a filter to exclude some resources and operations from external sources.
func filterAccessByType(access state.Access) error {
	// allow full access, these resources are managed on the client side
	if _, ok := userManagedResourceTypeSet[access.ResourceType]; ok {
		return nil
	}

	switch access.ResourceType {
	case siderolink.LinkType:
		// Allow read, update and delete access
		// Update access is required for siderolink by rtestutils.Destroy[*siderolink.Link] call on integration tests
		if access.Verb.Readonly() || access.Verb == state.Update || access.Verb == state.Destroy {
			return nil
		}

		return status.Error(codes.PermissionDenied, "only read, update and delete access is permitted")
	case
		cloud.MachineRequestType,       // read-only for all except for CloudProvider role (checked in filterAccess)
		cloud.MachineRequestStatusType, // read-only for all except for CloudProvider role (checked in filterAccess)
		omni.ClusterBootstrapStatusType,
		omni.ClusterDestroyStatusType,
		omni.ClusterEndpointType,
		omni.ClusterKubernetesNodesType,
		omni.ClusterMachineIdentityType,
		omni.ClusterMachineStatusType,
		omni.ClusterMachineTalosVersionType,
		omni.ClusterMachineType,
		omni.ClusterMachineConfigPatchesType,
		omni.ClusterMachineTemplateType,
		omni.ClusterStatusMetricsType,
		omni.ClusterStatusType,
		omni.ClusterTaintType,
		omni.ClusterUUIDType,
		omni.ClusterWorkloadProxyStatusType,
		omni.ControlPlaneStatusType,
		omni.KubernetesNodeAuditResultType,
		omni.ExposedServiceType,
		omni.EtcdBackupType,
		omni.EtcdBackupStatusType,
		omni.EtcdBackupOverallStatusType,
		omni.EtcdBackupStoreStatusType,
		omni.FeaturesConfigType,
		omni.ImagePullRequestType,
		omni.ImagePullStatusType,
		omni.KubernetesStatusType,
		omni.KubernetesUpgradeManifestStatusType,
		omni.KubernetesUpgradeStatusType,
		omni.LoadBalancerConfigType,
		omni.LoadBalancerStatusType,
		omni.MachineType,
		omni.MachineClassType,
		omni.MachineClassStatusType,
		omni.MachineConfigGenOptionsType,
		omni.MachineSetDestroyStatusType,
		omni.MachineSetRequiredMachinesType,
		omni.MachineSetStatusType,
		omni.MachineStatusType,
		omni.MachineStatusLinkType,
		omni.MachineStatusSnapshotType,
		omni.KubernetesVersionType,
		omni.TalosExtensionsType,
		omni.TalosVersionType,
		omni.TalosUpgradeStatusType,
		omni.InstallationMediaType,
		omni.OngoingTaskType,
		omni.RedactedClusterMachineConfigType,
		omni.SchematicType,
		omni.SchematicConfigurationType,
		omni.ExtensionsConfigurationStatusType,
		omni.MachineExtensionsStatusType,
		omni.MachineExtensionsType,
		omni.MachineStatusMetricsType,
		authres.AuthConfigType,
		siderolink.ConnectionParamsType,
		system.SysVersionType,
		meta.NamespaceType,
		meta.ResourceDefinitionType,
		virtual.CurrentUserType,
		virtual.PermissionsType,
		virtual.KubernetesUsageType,
		virtual.LabelsCompletionType,
		virtual.ClusterPermissionsType:
		// allow read access only, these resources are either managed by controllers or plugins (e.g., cloud provider plugins)
		if access.Verb.Readonly() {
			return nil
		}

		return status.Error(codes.PermissionDenied, "only read access is permitted")
	default:
		return status.Error(codes.PermissionDenied, "access is not permitted")
	}
}
