// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package helpers defines common runtime helper functions.
package helpers

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// GetMachineEndpoints reads all possible machine endpoints from the ClusterMachineIdentity resources.
func GetMachineEndpoints(ctx context.Context, st state.State, clusterName string) ([]string, error) {
	endpoints, err := safe.ReaderListAll[*omni.ClusterMachineIdentity](ctx, st,
		state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
			resource.LabelExists(omni.LabelControlPlaneRole),
		),
	)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, endpoints.Len())

	endpoints.ForEach(func(r *omni.ClusterMachineIdentity) {
		res = append(res, r.TypedSpec().Value.NodeIps...)
	})

	return res, nil
}
