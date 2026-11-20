// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/pkg/authz"
	"github.com/siderolabs/omni/internal/pkg/authz/grants"
)

// errNoAccess is the answer to a caller who is refused for the target of a request to Talos.
//
// It is the same error for every target, also for a cluster or a machine which does not exist.
// This way, a caller without access cannot find out from the answer which clusters and machines exist.
var errNoAccess = status.Error(codes.PermissionDenied, "no role grants access to Talos on the target")

// CheckRead checks that the caller can read from Talos on the machines of the cluster.
// An empty cluster ID is for a machine which is in no cluster.
func CheckRead(ctx context.Context, st state.State, clusterID string) error {
	return check(ctx, st, clusterID, authz.Read)
}

// CheckModify checks that the caller can change the machines of the cluster through Talos.
// An empty cluster ID is for a machine which is in no cluster.
func CheckModify(ctx context.Context, st state.State, clusterID string) error {
	return check(ctx, st, clusterID, authz.Update)
}

func check(ctx context.Context, st state.State, clusterID string, verb authz.Verbs) error {
	var err error

	if clusterID == "" {
		// A machine in no cluster is in maintenance mode, where the Talos API has no roles.
		// Reading from it and changing it are the same access, and both require managing machines.
		err = grants.CheckOperation(ctx, st, authz.MachineMaintenance, authz.Target{})
	} else {
		err = grants.CheckResourceGroup(ctx, st, authz.Clusters, verb, authz.Target{Cluster: clusterID})
	}

	return hideTarget(err)
}

// ResolveError returns the error for a request to Talos whose target could not be resolved, e.g., a node which
// is not known.
//
// The caller gets resolveErr only when it can read every machine, as it can see which machines exist anyway.
// Every other caller gets the error of a request it has no access for.
//
// The access policy is not read for it: it gives access to clusters, and never to every machine.
func ResolveError(ctx context.Context, resolveErr error) error {
	if err := authz.CheckResourceGroup(grants.Subject(ctx), authz.Machines, authz.Read, authz.Target{}); err != nil {
		return hideTarget(err)
	}

	return resolveErr
}

// hideTarget replaces every refusal with errNoAccess. An error which is not a refusal is returned as it is.
func hideTarget(err error) error {
	if status.Code(err) == codes.PermissionDenied {
		return errNoAccess
	}

	return err
}
