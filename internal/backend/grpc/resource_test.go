// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/internal/backend/dns"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/runtime"
)

type countingNodeResolver struct {
	machineID string
	calls     int
}

func (r *countingNodeResolver) Resolve(string, string) (dns.Info, error) {
	r.calls++

	return dns.Info{ID: r.machineID}, nil
}

// TestMachineOptionsResolveOnlyForTalosRuntime checks that the node headers reach the resolver only for the runtime
// that consumes them, and that what it resolves them to is what the query targets.
func TestMachineOptionsResolveOnlyForTalosRuntime(t *testing.T) {
	const machineID = "6d0b70b0-de37-4b1d-9b5a-3f4c0a9b6f41"

	for _, test := range []struct {
		name     string
		runtime  string
		node     string
		machines []string
	}{
		{name: "omni runtime", runtime: common.Runtime_Omni.String(), node: "some-machine"},
		{name: "kubernetes runtime", runtime: common.Runtime_Kubernetes.String(), node: "some-machine"},
		{name: "no runtime header", node: "some-machine"},
		{name: "talos runtime without a node header", runtime: common.Runtime_Talos.String()},
		{
			name:     "talos runtime with a node header",
			runtime:  common.Runtime_Talos.String(),
			node:     "some-machine",
			machines: []string{machineID},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := &countingNodeResolver{machineID: machineID}
			server := grpcomni.NewResourceServer(nil, nil, nil, resolver)

			md := metadata.New(nil)
			if test.runtime != "" {
				md.Set("runtime", test.runtime)
			}

			if test.node != "" {
				md.Set("node", test.node)
			}

			opts, err := server.MachineOptions(context.Background(), md)
			require.NoError(t, err)

			require.Equal(t, test.machines, runtime.NewQueryOptions(opts...).Machines)

			if test.machines == nil {
				require.Zero(t, resolver.calls, "the node headers must not be resolved for this request")
			}
		})
	}
}
