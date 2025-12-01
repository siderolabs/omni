// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package configs computes virtual resources for all available SBCs, Cloud platforms in the image factory
// using the common module from the Talos machinery.
// This data is used in the UI to display the available options in the installation media creation wizard.
package configs

import (
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/platforms"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual/pkg/errors"
)

// GetMetalPlatformConfig supports only metal ID.
func GetMetalPlatformConfig(ptr resource.Pointer) (*virtual.MetalPlatformConfig, error) {
	metal := platforms.MetalPlatform()
	if ptr.ID() != metal.Name {
		return nil, errors.ErrNotFound(ptr)
	}

	return newMetalPlatformConfig(metal), nil
}

// ListMetalPlatformConfigs always returns a single item but is there to make the API similar to the cloud platforms.
func ListMetalPlatformConfigs() resource.List {
	return resource.List{
		Items: []resource.Resource{newMetalPlatformConfig(platforms.MetalPlatform())},
	}
}

// GetSBCConfig by name.
func GetSBCConfig(ptr resource.Pointer) (*virtual.SBCConfig, error) {
	sbcs := platforms.SBCs()

	idx := slices.IndexFunc(sbcs, func(sbc platforms.SBC) bool {
		return sbc.Name == ptr.ID()
	})

	if idx == -1 {
		return nil, errors.ErrNotFound(ptr)
	}

	return newSBCConfig(sbcs[idx]), nil
}

// ListSBCConfigs returns all available SBC configs.
func ListSBCConfigs() resource.List {
	return resource.List{
		Items: xslices.Map(platforms.SBCs(), func(sbc platforms.SBC) resource.Resource {
			return newSBCConfig(sbc)
		}),
	}
}

// GetCloudPlatformConfig by name.
func GetCloudPlatformConfig(ptr resource.Pointer) (*virtual.CloudPlatformConfig, error) {
	list := platforms.CloudPlatforms()

	idx := slices.IndexFunc(list, func(platform platforms.Platform) bool {
		return platform.Name == ptr.ID()
	})

	if idx == -1 {
		return nil, errors.ErrNotFound(ptr)
	}

	return newCloudPlatformConfig(list[idx]), nil
}

// ListCloudPlatformConfigs returns all available SBC configs.
func ListCloudPlatformConfigs() resource.List {
	return resource.List{
		Items: xslices.Map(platforms.CloudPlatforms(), func(platform platforms.Platform) resource.Resource {
			return newCloudPlatformConfig(platform)
		}),
	}
}

func newSBCConfig(sbc platforms.SBC) *virtual.SBCConfig {
	res := virtual.NewSBCConfig(sbc.Name)

	res.TypedSpec().Value.Label = sbc.Label
	res.TypedSpec().Value.Documentation = sbc.Documentation
	res.TypedSpec().Value.OverlayImage = sbc.OverlayImage
	res.TypedSpec().Value.OverlayName = sbc.OverlayName

	if sbc.MinVersion.String() != "0.0.0" {
		res.TypedSpec().Value.MinVersion = sbc.MinVersion.String()
	}

	return res
}

func newCloudPlatformConfig(platform platforms.Platform) *virtual.CloudPlatformConfig {
	res := virtual.NewCloudPlatformConfig(platform.Name)

	res.TypedSpec().Value = newPlatformConfig(platform)

	return res
}

func newMetalPlatformConfig(platform platforms.Platform) *virtual.MetalPlatformConfig {
	res := virtual.NewMetalPlatformConfig(platform.Name)

	res.TypedSpec().Value = newPlatformConfig(platform)

	return res
}

func newPlatformConfig(platform platforms.Platform) *specs.PlatformConfigSpec {
	spec := &specs.PlatformConfigSpec{}

	spec.Label = platform.Label
	spec.Description = platform.Description
	spec.Documentation = platform.Documentation
	spec.Architectures = platform.Architectures
	spec.DiskImageSuffix = platform.DiskImageSuffix

	for _, bootMethod := range platform.BootMethods {
		switch bootMethod {
		case platforms.BootMethodDiskImage:
			spec.BootMethods = append(spec.BootMethods, specs.PlatformConfigSpec_DISK_IMAGE)
		case platforms.BootMethodISO:
			spec.BootMethods = append(spec.BootMethods, specs.PlatformConfigSpec_ISO)
		case platforms.BootMethodPXE:
			spec.BootMethods = append(spec.BootMethods, specs.PlatformConfigSpec_PXE)
		}
	}

	if platform.MinVersion.String() != "0.0.0" {
		spec.MinVersion = platform.MinVersion.String()
	}

	return spec
}
