// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/infraprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/authz"
	"github.com/siderolabs/omni/internal/pkg/authz/grants"
)

// authorizationValidationOptions returns the validation options which check the access of the caller to the resources.
//
// Each one finds out what the request is about (the type, the verb and the cluster the resource belongs to) and asks the authz package.
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
			getValidation(st),
			labelsCompletionValidation(st),
		),
		validated.WithWatchValidations(watchValidation(st)),
		validated.WithCreateValidations(
			func(ctx context.Context, res resource.Resource, _ ...state.CreateOption) error {
				clusterID := clusterIDFromMetadata(res.Metadata())

				return checkAccess(ctx, st, state.Access{
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

					return checkAccess(ctx, st, state.Access{
						ResourceNamespace: newRes.Metadata().Namespace(),
						ResourceType:      newRes.Metadata().Type(),
						ResourceID:        newRes.Metadata().ID(),
						Verb:              state.Update,
					}, newClusterID, false)
				}

				existingClusterID := clusterIDFromMetadata(existingRes.Metadata())

				if err := checkAccess(ctx, st, state.Access{
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

				return checkAccess(ctx, st, state.Access{
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

					return checkAccess(ctx, st, state.Access{
						ResourceNamespace: ptr.Namespace(),
						ResourceType:      ptr.Type(),
						ResourceID:        ptr.ID(),
						Verb:              state.Destroy,
					}, clusterID, false)
				}

				clusterID := clusterIDFromMetadata(res.Metadata())

				return checkAccess(ctx, st, state.Access{
					ResourceNamespace: res.Metadata().Namespace(),
					ResourceType:      res.Metadata().Type(),
					ResourceID:        res.Metadata().ID(),
					Verb:              state.Destroy,
				}, clusterID, false)
			},
		),
	}
}

// getValidation checks a get of a resource, which is nil when it does not exist.
func getValidation(st state.State) validated.GetValidation {
	return func(ctx context.Context, ptr resource.Pointer, res resource.Resource, _ ...state.GetOption) error {
		// when the resource does not exist, the pointer decides between NotFound and PermissionDenied
		clusterID := clusterIDFromPointer(ptr)
		if res != nil {
			clusterID = clusterIDFromMetadata(res.Metadata())
		}

		return checkRead(ctx, st, ptr, state.Get, clusterID)
	}
}

// watchValidation checks a watch of a single resource.
//
// It is the same check as for a get. The difference is what the two are given: a get validation receives the resource,
// a watch validation only the pointer to it. A resource which belongs to a cluster by its label has that cluster in
// its labels, so the resource is read here to find it.
func watchValidation(st state.State) validated.WatchValidation {
	return func(ctx context.Context, ptr resource.Pointer, _ ...state.WatchOption) error {
		clusterID := clusterIDFromPointer(ptr)

		if authz.LookupResourceType(ptr.Type()).Cluster == authz.ClusterByLabel {
			res, err := st.Get(actor.MarkContextAsInternalActor(ctx), ptr)
			if err != nil && !state.IsNotFoundError(err) {
				return err
			}

			if res != nil {
				clusterID = clusterIDFromMetadata(res.Metadata())
			}
		}

		return checkRead(ctx, st, ptr, state.Watch, clusterID)
	}
}

// checkRead checks a get or a watch of a single resource.
func checkRead(ctx context.Context, st state.State, ptr resource.Pointer, verb state.Verb, clusterID resource.ID) error {
	access := state.Access{
		ResourceNamespace: ptr.Namespace(),
		ResourceType:      ptr.Type(),
		ResourceID:        ptr.ID(),
		Verb:              verb,
	}

	err := checkAccess(ctx, st, access, clusterID, false)
	if status.Code(err) != codes.PermissionDenied || authz.LookupResourceType(ptr.Type()).Cluster != authz.ClusterByMachine {
		return err
	}

	// the cluster of the machine is looked up only when the caller has no access to the machine without it
	machineID := machineIDOf(ptr)
	if machineID == "" {
		return err
	}

	clusterID, lookupErr := grants.ClusterOfMachine(ctx, st, machineID)
	if lookupErr != nil {
		return lookupErr
	}

	if clusterID == "" {
		return err
	}

	return checkAccess(ctx, st, access, clusterID, false)
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

// checkAccess checks the request of the caller in the context to the resource API.
func checkAccess(ctx context.Context, st state.State, access state.Access, clusterID resource.ID, requireAll bool) error {
	if !actor.ContextIsInternalActor(ctx) {
		// the access to the infra provider resources is checked by the infra provider state
		if infraprovider.IsInfraProviderResource(access.ResourceNamespace, access.ResourceType) {
			return nil
		}

		if infraprovider.IsInfraProviderResource(resources.InfraProviderNamespace, access.ResourceType) {
			return status.Error(codes.PermissionDenied, "infra provider resources are only served in the infra provider namespaces")
		}
	}

	return grants.AllowResource(ctx, st, authz.Target{
		Type:        access.ResourceType,
		Verb:        access.Verb,
		Cluster:     clusterID,
		AllClusters: requireAll,
	})
}

// checkForKindAccess checks a request for many resources of a type: a list, or a watch of the type.
//
// Such a request has no single resource to check. What it returns depends on its label query, so the query decides
// what the caller needs:
//   - a query for a single cluster needs access to that cluster
//   - a query for a single machine, on a type which holds settings of machines, needs access to the settings of machines
//   - any other query can return the resources of every cluster, so it needs access to all clusters
func checkForKindAccess(ctx context.Context, st state.State, verb state.Verb, kind resource.Kind, labelTerms []resource.LabelTerm) error {
	clusterID := ""
	requireAll := false

	if isClusterRelatedType(kind.Type()) {
		// a query which is not for a single cluster leaves the ID empty
		if id, err := labels.ExactValue(labelTerms, omni.LabelCluster); err == nil {
			clusterID = id
		}

		requireAll = clusterID == ""

		// Some types hold both resources of clusters and settings of machines, e.g., a config patch can be for a cluster or for a machine.
		// The ones for a machine have the machine label and no cluster label.
		// So a query for a single machine on such a type asks for the settings of that machine, and not for the resources of all clusters.
		machineID, err := labels.ExactValue(labelTerms, omni.LabelMachine)
		isForSingleMachine := err == nil && machineID != ""
		holdsMachineSettings := authz.LookupResourceType(kind.Type()).GroupWithoutCluster != ""

		if requireAll && isForSingleMachine && holdsMachineSettings {
			requireAll = false
		}
	}

	return checkAccess(ctx, st, state.Access{
		ResourceNamespace: kind.Namespace(),
		ResourceType:      kind.Type(),
		Verb:              verb,
	}, clusterID, requireAll)
}

// isClusterRelatedType reports whether a list of the type can be limited to a cluster.
func isClusterRelatedType(typ resource.Type) bool {
	relation := authz.LookupResourceType(typ).Cluster

	return relation == authz.ClusterByID || relation == authz.ClusterByLabel
}

// machineIDOf returns the ID of the machine a resource of a ClusterByMachine type is for, empty when it is for no machine.
func machineIDOf(ptr resource.Pointer) resource.ID {
	if ptr.Type() != siderolink.LinkStatusType {
		return ptr.ID()
	}

	// LinkStatus needs special handling, because its ID is not the ID of the machine: it also contains the type of
	// the resource it is the status of. Only the LinkStatus of a Link is for a machine which can be in a cluster,
	// the one of a PendingMachine is for a machine which is not accepted yet.
	id, typ, _ := siderolink.ParseLinkStatusID(ptr.ID())
	if typ != siderolink.LinkType {
		return ""
	}

	return id
}

func clusterIDFromMetadata(resMD *resource.Metadata) resource.ID {
	switch authz.LookupResourceType(resMD.Type()).Cluster {
	case authz.ClusterByID:
		return resMD.ID()
	case authz.ClusterByLabel:
		cluster, _ := resMD.Labels().Get(omni.LabelCluster)

		return cluster
	case authz.NoCluster, authz.ClusterByMachine:
	}

	return ""
}

func clusterIDFromPointer(ptr resource.Pointer) resource.ID {
	if authz.LookupResourceType(ptr.Type()).Cluster == authz.ClusterByID {
		return ptr.ID()
	}

	return ""
}
