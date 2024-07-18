// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package mappers provides Omni-specific mappers for QControllers.
package mappers

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MapExtractLabelValue returns a mapper that extracts a label value from a resource, potentially evaluating a condition on labels.
func MapExtractLabelValue[I generic.ResourceWithRD, O generic.ResourceWithRD](extractableLabel string, labelTerms ...resource.LabelTerm) qtransform.MapperFuncGeneric[I] {
	var zeroOutput O

	outputNamespace := zeroOutput.ResourceDefinition().DefaultNamespace
	outputType := zeroOutput.ResourceDefinition().Type

	return func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, i I) ([]resource.Pointer, error) {
		for _, labelTerm := range labelTerms {
			if !i.Metadata().Labels().Matches(labelTerm) {
				return nil, nil
			}
		}

		value, ok := i.Metadata().Labels().Get(extractableLabel)
		if !ok {
			return nil, nil
		}

		return []resource.Pointer{resource.NewMetadata(outputNamespace, outputType, value, resource.VersionUndefined)}, nil
	}
}

// MapByClusterLabel returns a mapper that extracts a LabelCluster value.
func MapByClusterLabel[I generic.ResourceWithRD, O generic.ResourceWithRD]() qtransform.MapperFuncGeneric[I] {
	return MapExtractLabelValue[I, O](omni.LabelCluster)
}

// MapByClusterLabelOnlyControlplane returns a mapper that extracts a LabelCluster value, but only if the resource has LabelControlPlaneRole.
func MapByClusterLabelOnlyControlplane[I generic.ResourceWithRD, O generic.ResourceWithRD]() qtransform.MapperFuncGeneric[I] {
	return MapExtractLabelValue[I, O](omni.LabelCluster, resource.LabelTerm{Key: omni.LabelControlPlaneRole, Op: resource.LabelOpExists})
}

// MapByMachineSetLabel returns a mapper that extracts a LabelMachineSet value.
func MapByMachineSetLabel[I generic.ResourceWithRD, O generic.ResourceWithRD]() qtransform.MapperFuncGeneric[I] {
	return MapExtractLabelValue[I, O](omni.LabelMachineSet)
}

// MapByMachineSetLabelOnlyControlplane returns a mapper that extracts a LabelMachineSet value, but only if the resource has LabelControlPlaneRole.
func MapByMachineSetLabelOnlyControlplane[I generic.ResourceWithRD, O generic.ResourceWithRD]() qtransform.MapperFuncGeneric[I] {
	return MapExtractLabelValue[I, O](omni.LabelMachineSet, resource.LabelTerm{Key: omni.LabelControlPlaneRole, Op: resource.LabelOpExists})
}

// MapByMachineClassNameLabel returns a mapper that extracts a LabelMachineClassName value.
func MapByMachineClassNameLabel[I generic.ResourceWithRD, O generic.ResourceWithRD]() qtransform.MapperFuncGeneric[I] {
	return MapExtractLabelValue[I, O](omni.LabelMachineClassName)
}

// MapClusterResourceToLabeledResources returns a mapper that maps a cluster resource to all resources with the same cluster label.
func MapClusterResourceToLabeledResources[I generic.ResourceWithRD, O generic.ResourceWithRD]() qtransform.MapperFuncGeneric[I] {
	return func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, i I) ([]resource.Pointer, error) {
		clusterName := i.Metadata().ID()

		items, err := safe.ReaderListAll[O](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		if err != nil {
			return nil, err
		}

		return safe.Map(items, func(item O) (resource.Pointer, error) {
			return item.Metadata(), nil
		})
	}
}

// MapMachineSetToLabeledResources returns a mapper that maps a machine set resource to all resources with the same machine set label.
func MapMachineSetToLabeledResources[I generic.ResourceWithRD, O generic.ResourceWithRD]() qtransform.MapperFuncGeneric[I] {
	return func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, i I) ([]resource.Pointer, error) {
		machineSetName := i.Metadata().ID()

		items, err := safe.ReaderListAll[O](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSetName)))
		if err != nil {
			return nil, err
		}

		return safe.Map(items, func(item O) (resource.Pointer, error) {
			return item.Metadata(), nil
		})
	}
}
