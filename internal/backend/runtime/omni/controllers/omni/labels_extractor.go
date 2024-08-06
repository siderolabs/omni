// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

// NewLabelsExtractorController creates new LabelsExtractorController for a specific resource type.
func NewLabelsExtractorController[T generic.ResourceWithRD]() *qtransform.QController[T, *system.ResourceLabels[T]] {
	var zero T

	rd := zero.ResourceDefinition()

	return qtransform.NewQController(
		qtransform.Settings[T, *system.ResourceLabels[T]]{
			Name: fmt.Sprintf("LabelsExtractor[%s]", rd.Type),
			MapMetadataFunc: func(res T) *system.ResourceLabels[T] {
				return system.NewResourceLabels[T](res.Metadata().ID())
			},
			UnmapMetadataFunc: func(labels *system.ResourceLabels[T]) T {
				res, err := protobuf.CreateResource(rd.Type)
				if err != nil {
					return zero
				}

				*res.Metadata() = resource.NewMetadata(rd.DefaultNamespace, rd.Type, labels.Metadata().ID(), resource.VersionUndefined)

				return res.(T) //nolint:forcetypeassert
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, res T, labels *system.ResourceLabels[T]) error {
				*labels.Metadata().Labels() = *res.Metadata().Labels()

				return nil
			},
		},
	)
}
