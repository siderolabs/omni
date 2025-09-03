// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package mappers provides Omni-specific mappers for QControllers.
package mappers

import (
	"context"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MapByClusterLabel returns a mapper that extracts a LabelCluster value.
func MapByClusterLabel[O generic.ResourceWithRD]() qtransform.MapperFunc {
	return qtransform.MapExtractLabelValue[O](omni.LabelCluster)
}

// MapByClusterLabelOnlyControlplane returns a mapper that extracts a LabelCluster value, but only if the resource has LabelControlPlaneRole.
func MapByClusterLabelOnlyControlplane[O generic.ResourceWithRD]() qtransform.MapperFunc {
	return qtransform.MapExtractLabelValue[O](omni.LabelCluster, resource.LabelTerm{Key: omni.LabelControlPlaneRole, Op: resource.LabelOpExists})
}

// MapByMachineSetLabel returns a mapper that extracts a LabelMachineSet value.
func MapByMachineSetLabel[O generic.ResourceWithRD]() qtransform.MapperFunc {
	return qtransform.MapExtractLabelValue[O](omni.LabelMachineSet)
}

// MapByMachineSetLabelOnlyControlplane returns a mapper that extracts a LabelMachineSet value, but only if the resource has LabelControlPlaneRole.
func MapByMachineSetLabelOnlyControlplane[O generic.ResourceWithRD]() qtransform.MapperFunc {
	return qtransform.MapExtractLabelValue[O](omni.LabelMachineSet, resource.LabelTerm{Key: omni.LabelControlPlaneRole, Op: resource.LabelOpExists})
}

// MapByMachineClassNameLabel returns a mapper that extracts a LabelMachineClassName value.
func MapByMachineClassNameLabel[O generic.ResourceWithRD]() qtransform.MapperFunc {
	return qtransform.MapExtractLabelValue[O](omni.LabelMachineClassName)
}

// MapClusterResourceToLabeledResources returns a mapper that maps a cluster resource to all resources with the same cluster label.
func MapClusterResourceToLabeledResources[O generic.ResourceWithRD]() qtransform.MapperFunc {
	return func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, in controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
		clusterName := in.ID()

		items, err := safe.ReaderListAll[O](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		if err != nil {
			return nil, err
		}

		return slices.Collect(items.Pointers()), nil
	}
}

// MapMachineSetToLabeledResources returns a mapper that maps a machine set resource to all resources with the same machine set label.
func MapMachineSetToLabeledResources[O generic.ResourceWithRD]() qtransform.MapperFunc {
	return func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, in controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
		machineSetName := in.ID()

		items, err := safe.ReaderListAll[O](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSetName)))
		if err != nil {
			return nil, err
		}

		return slices.Collect(items.Pointers()), nil
	}
}
