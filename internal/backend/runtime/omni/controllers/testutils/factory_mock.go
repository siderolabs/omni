// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testutils

import (
	"context"

	"github.com/siderolabs/image-factory/pkg/schematic"

	"github.com/siderolabs/omni/internal/backend/imagefactory"
)

// NewImageFactoryMockClient creates a fake image factory client.
func NewImageFactoryMockClient() *ImageFactoryMockClient {
	return &ImageFactoryMockClient{}
}

// ImageFactoryMockClient is a mock image factory client.
type ImageFactoryMockClient struct{}

// EnsureSchematic ...
func (i *ImageFactoryMockClient) EnsureSchematic(_ context.Context, sch schematic.Schematic) (imagefactory.EnsuredSchematic, error) {
	fullID, err := sch.ID()
	if err != nil {
		return imagefactory.EnsuredSchematic{}, err
	}

	plainSchematic := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: sch.Customization.SystemExtensions.OfficialExtensions,
			},
		},
	}

	plainID, err := plainSchematic.ID()
	if err != nil {
		return imagefactory.EnsuredSchematic{}, err
	}

	return imagefactory.EnsuredSchematic{
		FullID:  fullID,
		PlainID: plainID,
	}, nil
}

// Host ...
func (i *ImageFactoryMockClient) Host() string {
	return "image.factory.test"
}
