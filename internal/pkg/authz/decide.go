// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authz

import (
	"errors"
	"slices"

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

// ErrNoGrant marks the error of a request which is refused only because no grant of the subject covers it.
//
// More grants can change that answer, so a caller which has another source of grants, e.g., the access policy,
// tries it when it sees this error. Every other refusal is final: no grant makes an instance which is suspended writable.
var ErrNoGrant = errors.New("no grant covers the request")

// noGrantError is a PermissionDenied error for the client, which is also ErrNoGrant.
type noGrantError struct {
	error
}

func (e noGrantError) GRPCStatus() *status.Status {
	return status.Convert(e.error)
}

func (e noGrantError) Is(target error) bool {
	return target == ErrNoGrant
}

// CheckResource checks whether the subject can do the request to the resource API, and returns an error when it cannot.
func CheckResource(subject Subject, target Target) error {
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

	// The resource group the role needs a rule for. A type has one group, with one exception: a type which holds both
	// resources of clusters and settings of machines has a second group for the ones which belong to no cluster.
	// For example, a config patch of a cluster is in "clusters", a config patch of a machine is in "machine-management".
	group := resourceType.Group
	if target.Cluster == "" && !target.AllClusters && resourceType.GroupWithoutCluster != "" {
		group = resourceType.GroupWithoutCluster
	}

	if !subject.grants(group, verbOf(target.Verb), target) {
		return noGrantError{status.Errorf(codes.PermissionDenied, "no role grants %s access to %s", verbOf(target.Verb), target.Type)}
	}

	return nil
}

// CheckResourceGroup checks whether the subject has the verb on the resource group for the target, and returns an error when it has not.
//
// It is for the requests which are about a resource group as a whole and not about a resource of some type,
// like the RPCs of the management API. The type and the verb of the target are ignored.
func CheckResourceGroup(subject Subject, group ResourceGroup, verb Verbs, target Target) error {
	switch {
	case subject.Internal:
		return nil
	case !subject.Authenticated:
		return status.Error(codes.Unauthenticated, "missing valid signature")
	case subject.Provider:
		return status.Error(codes.PermissionDenied, "infra providers have no access")
	case subject.Suspended && verb != Read:
		return status.Error(codes.PermissionDenied, "the instance is suspended, only read access is permitted")
	case !subject.grants(group, verb, target):
		return noGrantError{status.Errorf(codes.PermissionDenied, "no role grants %s access to %s", verb, group)}
	}

	return nil
}

// CheckOperation checks whether the subject can do the operation on the target, and returns an error when it cannot.
func CheckOperation(subject Subject, operation Operation, target Target) error {
	switch {
	case subject.Internal:
		return nil
	case !subject.Authenticated:
		return status.Error(codes.Unauthenticated, "missing valid signature")
	case subject.Provider:
		return status.Error(codes.PermissionDenied, "infra providers have no access")
	case subject.Suspended && !slices.Contains(readOnlyOperations, operation):
		return status.Errorf(codes.PermissionDenied, "the instance is suspended, %s is not permitted", operation)
	}

	for _, grant := range subject.Grants {
		if grant.Scope.covers(target) && grant.Role.allowsOperation(operation) {
			return nil
		}
	}

	return noGrantError{status.Errorf(codes.PermissionDenied, "no role grants %s", operation)}
}

// CheckInstallDiskConfig checks whether the subject's access to clusters covers the request to the install disk config
// of a machine, and returns an error when it does not. The cluster is the one the machine is in, empty when it is in no cluster.
//
// It runs only after CheckResource refused the request with ErrNoGrant. The comment at its call in the resource API
// explains why the install disk config is an exception.
//
//   - The machine is in no cluster: allowed when one grant on at least one cluster allows create
//     and the verb of the request on clusters.
//   - The machine is in a cluster: allowed when the subject has the verb on that cluster.
func CheckInstallDiskConfig(subject Subject, verb state.Verb, cluster string) error {
	for _, grant := range subject.Grants {
		if cluster == "" {
			// a grant on any cluster is enough, because the subject can add the machine to that cluster
			if (grant.Scope.Global || len(grant.Scope.Clusters) > 0) && grant.Role.allows(Clusters, Create) && grant.Role.allows(Clusters, verbOf(verb)) {
				return nil
			}

			continue
		}

		if grant.Scope.covers(Target{Cluster: cluster}) && grant.Role.allows(Clusters, verbOf(verb)) {
			return nil
		}
	}

	return noGrantError{status.Errorf(codes.PermissionDenied, "no role grants %s access to the install disk config of the machine", verbOf(verb))}
}

// grants reports whether a grant of the subject covers the target and allows the verb on the resource group.
func (subject Subject) grants(group ResourceGroup, verb Verbs, target Target) bool {
	for _, grant := range subject.Grants {
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
