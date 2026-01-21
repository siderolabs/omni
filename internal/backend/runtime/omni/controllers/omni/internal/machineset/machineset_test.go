// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset_test

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/siderolabs/gen/pair"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/machineset"
)

func tearingDown[T resource.Resource](res T) T {
	res = tearingDownNoFinalizers(res)

	res.Metadata().Finalizers().Add(machineset.ControllerName)

	return res
}

func tearingDownNoFinalizers[T resource.Resource](res T) T {
	res.Metadata().SetPhase(resource.PhaseTearingDown)

	return res
}

func withVersion[T resource.Resource](res T, version resource.Version) T {
	res.Metadata().SetVersion(version)

	return res
}

func withSpecSetter[T resource.Resource](res T, f func(T)) T {
	f(res)

	return res
}

func withUpdateInputVersions[T, R resource.Resource](res T, inputs ...R) T {
	helpers.UpdateInputsVersions(res, inputs...)

	return res
}

func withClusterMachineConfigVersionSetter(cmcs *omni.ClusterMachineConfigStatus, version resource.Version) *omni.ClusterMachineConfigStatus {
	return withSpecSetter(cmcs, func(cmcs *omni.ClusterMachineConfigStatus) {
		cmcs.TypedSpec().Value.ClusterMachineConfigVersion = version.String()
	})
}

func withLabels[T resource.Resource](r T, labels ...pair.Pair[string, string]) T {
	r.Metadata().Labels().Do(func(temp kvutils.TempKV) {
		for _, label := range labels {
			temp.Set(label.F1, label.F2)
		}
	})

	return r
}

func newHealthyLB(id string) *omni.LoadBalancerStatus {
	return withSpecSetter(omni.NewLoadBalancerStatus(id), func(r *omni.LoadBalancerStatus) {
		r.TypedSpec().Value.Healthy = true
	})
}
