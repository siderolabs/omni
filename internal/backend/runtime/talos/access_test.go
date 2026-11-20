// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos_test

import (
	"errors"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/pkg/authz/authztest"
)

// TestNoAccessAnswerDoesNotDependOnTarget checks that a caller without access gets the same error for every target,
// also for a cluster or a node which does not exist.
func TestNoAccessAnswerDoesNotDependOnTarget(t *testing.T) {
	keys := authztest.GenerateKeys(t)
	st := state.WrapCore(inmem.NewState())

	authztest.Seed(t, st, keys)

	notResolved := errors.New("node not found")

	for _, subject := range authztest.Subjects {
		var clusters []string

		switch subject.Name {
		case "none":
			clusters = []string{authztest.ClusterA, authztest.ClusterB}
		case "acl-reader-a": // has access to cluster-a only
			clusters = []string{authztest.ClusterB}
		case "acl-operator-all": // has access to every cluster, but not to the machines in no cluster
		default:
			continue
		}

		t.Run(subject.Name, func(t *testing.T) {
			ctx := authztest.SignIn(t, st, keys, subject)

			answers := make([]error, 0, 3+2*len(clusters))

			// a machine in no cluster, and a node which is not known
			answers = append(answers, talos.CheckRead(ctx, st, ""), talos.CheckModify(ctx, st, ""), talos.ResolveError(ctx, notResolved))

			for _, cluster := range clusters {
				answers = append(answers, talos.CheckRead(ctx, st, cluster), talos.CheckModify(ctx, st, cluster))
			}

			for _, answer := range answers {
				require.Equal(t, codes.PermissionDenied, status.Code(answer))
				require.Equal(t, answers[0].Error(), answer.Error())
			}
		})
	}
}
