// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Subject is the caller of a request.
type Subject struct {
	Grants []Grant
	// Authenticated is set when the request has a valid signature.
	Authenticated bool
	// Internal is set for Omni itself, which can do everything.
	Internal bool
	// Provider is set for the infra providers. They get no grants.
	Provider bool
	// Suspended is set when the instance is suspended, which leaves only the read access.
	Suspended bool
}

// Target is what a request to the resource API is about.
type Target struct {
	Type resource.Type
	// Cluster is the cluster the resource belongs to, empty when it belongs to none.
	Cluster string
	Verb    state.Verb
	// AllClusters is set for a list or a watch of a type which belongs to clusters, when it is not limited to a single cluster.
	AllClusters bool
}

// AllowResource returns nil when the subject can do the request to the resource API.
func AllowResource(subject Subject, target Target) error {
	if subject.Internal {
		return nil
	}

	resourceType := LookupResourceType(target.Type)

	// the cases are in the order the checks apply, the first one which matches decides
	switch {
	case resourceType.Access != Public && !subject.Authenticated: // no valid signature, and the type is not open to everyone
		return status.Error(codes.Unauthenticated, "missing valid signature")
	case resourceType.Verbs == 0: // the type is internal to Omni, no caller of the API can access it
		return status.Error(codes.PermissionDenied, "access is not permitted")
	case !resourceType.Verbs.Has(target.Verb): // the type does not allow this verb to any caller, e.g., a status is read-only
		return status.Errorf(codes.PermissionDenied, "only %s access is permitted", resourceType.Verbs)
	case resourceType.Access != RequiresGrant: // the type needs no grant, a signature (or nothing) was enough
		return nil
	case subject.Provider:
		// An infra provider has no grants. It works with its own resources, which the infra provider state checks.
		// Outside of those, it only reads the few types it needs to do its job.
		if resourceType.ProviderReadable && target.Verb.Readonly() {
			return nil
		}

		return status.Error(codes.PermissionDenied, "infra providers have no access")
	case subject.Suspended && !target.Verb.Readonly(): // a suspended instance is read-only, whatever the grants are
		return status.Error(codes.PermissionDenied, "the instance is suspended, only read access is permitted")
	}

	if !subject.grants(resourceType, target) {
		return status.Errorf(codes.PermissionDenied, "no role grants %s access to %s", verbOf(target.Verb), target.Type)
	}

	return nil
}

// grants reports whether a grant of the subject covers the target and allows the verb on it.
func (subject Subject) grants(resourceType ResourceType, target Target) bool {
	verb := verbOf(target.Verb)
	inCluster := target.Cluster != "" || target.AllClusters

	// The resource group the role needs a rule for. A type has one group, with one exception: a type which holds both
	// resources of clusters and settings of machines has a second group for the ones which belong to no cluster.
	// For example, a config patch of a cluster is in "clusters", a config patch of a machine is in "machine-management".
	group := resourceType.Group
	if !inCluster && resourceType.GroupWithoutCluster != "" {
		group = resourceType.GroupWithoutCluster
	}

	for _, grant := range subject.Grants {
		// a role about clusters reaches the resources of a machine only while the machine is in a cluster
		if resourceType.Cluster == ClusterByMachine && grant.Role.ClusterScoped && !inCluster {
			continue
		}

		if grant.Scope.covers(target) && grant.Role.allows(group, verb) {
			return true
		}
	}

	return false
}

// verbOf returns the verb of the roles for a verb of the resource API. Get, list and watch are all a read.
//
// It returns no verb for a verb it does not know, which no rule allows.
func verbOf(verb state.Verb) Verbs {
	switch verb {
	case state.Create:
		return Create
	case state.Update:
		return Update
	case state.Destroy:
		return Destroy
	case state.Get, state.List, state.Watch:
		return Read
	default:
		return 0
	}
}
