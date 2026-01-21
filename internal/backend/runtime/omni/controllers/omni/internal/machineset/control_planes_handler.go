// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset

import (
	"context"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
)

// ReconcileControlPlanes gets the reconciliation context and produces the list of changes to apply on the machine set.
func ReconcileControlPlanes(ctx context.Context, rc *ReconciliationContext, etcdStatusGetter func(ctx context.Context) (*check.EtcdStatusResult, error)) ([]Operation, error) {
	toCreate := xslices.Map(rc.GetMachinesToCreate(), func(id string) Operation {
		return &Create{ID: id}
	})

	// create all machines and exit
	if len(toCreate) > 0 {
		return toCreate, nil
	}

	// destroy all ready for destroy machines
	if len(rc.GetMachinesToDestroy()) > 0 {
		return xslices.Map(rc.GetMachinesToDestroy(), func(id string) Operation {
			return &Destroy{ID: id}
		}), nil
	}

	// pending tearing down machines with finalizers should cancel any other operations
	if len(rc.GetTearingDownMachines()) > 0 {
		return nil, nil
	}

	if rc.LBHealthy() {
		// do a single destroy
		for _, id := range rc.GetMachinesToTeardown() {
			clusterMachine, ok := rc.GetClusterMachine(id)
			if !ok {
				continue
			}

			etcdStatus, err := etcdStatusGetter(ctx)
			if err != nil {
				return nil, err
			}

			if err := check.CanScaleDown(etcdStatus, clusterMachine); err != nil {
				return nil, err
			}

			return []Operation{&Teardown{ID: id}}, nil
		}
	}

	return nil, nil
}
