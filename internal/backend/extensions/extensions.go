// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package extensions provides utilities to work with extensions.
package extensions

import (
	"github.com/blang/semver"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/gen/xslices"
)

// OfficialPrefix is the prefix for the official extensions.
const OfficialPrefix = "siderolabs/"

var (
	talosV170 = semver.MustParse("1.7.0")
	talosV180 = semver.MustParse("1.8.0")
	talosV190 = semver.MustParse("1.9.0")

	talosV17RenamedExtensions = newBiMap[string, string](map[string]string{
		OfficialPrefix + "xe-guest-utilities": OfficialPrefix + "xen-guest-agent",
	})

	talosV18RenamedExtensions = newBiMap[string, string](map[string]string{
		OfficialPrefix + "nvidia-container-toolkit":       OfficialPrefix + "nvidia-container-toolkit-lts",
		OfficialPrefix + "nvidia-open-gpu-kernel-modules": OfficialPrefix + "nvidia-open-gpu-kernel-modules-lts",
		OfficialPrefix + "nonfree-kmod-nvidia":            OfficialPrefix + "nonfree-kmod-nvidia-lts",
		OfficialPrefix + "nvidia-fabricmanager":           OfficialPrefix + "nvidia-fabric-manager-lts",
	})

	talosV19RenamedExtensions = newBiMap[string, string](map[string]string{
		OfficialPrefix + "i915-ucode":      OfficialPrefix + "i915",
		OfficialPrefix + "amdgpu-firmware": OfficialPrefix + "amdgpu",
	})
)

// MapNames maps the extension names to their correct final names, taking extensions with the wrong name in their manifests into account.
//
// It does not take renamed extensions into account, MapNamesByVersion should be used for that.
func MapNames(extensions []string) []string {
	return xslices.Map(extensions, func(extension string) string {
		// extensions with the wrong name in their manifests
		switch extension {
		case OfficialPrefix + "v4l-uvc":
			return OfficialPrefix + "v4l-uvc-drivers"
		case OfficialPrefix + "usb-modem":
			return OfficialPrefix + "usb-modem-drivers"
		case OfficialPrefix + "gasket":
			return OfficialPrefix + "gasket-driver"
		case OfficialPrefix + "talos-vmtoolsd":
			return OfficialPrefix + "vmtoolsd-guest-agent"
		default:
			return extension
		}
	})
}

// MapNamesByVersion maps the extension names to their correct final names, taking both the extensions with the wrong name in their manifests and the renamed extensions into account.
//
// It is taken and adapted from here: https://github.com/siderolabs/image-factory/blob/9687413a9a85744c8d8254d6f8604c6a7854c244/internal/profile/profile.go#L267-L289
func MapNamesByVersion(extensions []string, talosVersion semver.Version) []string {
	extensions = MapNames(extensions)

	gte170 := talosVersion.GTE(talosV170)
	gte180 := talosVersion.GTE(talosV180)
	gte190 := talosVersion.GTE(talosV190)

	return xslices.Map(extensions, func(extension string) string {
		return mapSingleNameByVersion(extension, gte170, gte180, gte190)
	})
}

// mapSingleNameByVersion returns the renamed extension based on the talos version.
func mapSingleNameByVersion(extension string, gte170, gte180, get190 bool) string {
	if gte170 {
		if name, ok := talosV17RenamedExtensions.Get(extension); ok {
			return name
		}
	} else {
		if name, ok := talosV17RenamedExtensions.GetInverse(extension); ok {
			return name
		}
	}

	if gte180 {
		if name, ok := talosV18RenamedExtensions.Get(extension); ok {
			return name
		}
	} else {
		if name, ok := talosV18RenamedExtensions.GetInverse(extension); ok {
			return name
		}
	}

	if get190 {
		if name, ok := talosV19RenamedExtensions.Get(extension); ok {
			return name
		}
	} else {
		if name, ok := talosV19RenamedExtensions.GetInverse(extension); ok {
			return name
		}
	}

	return extension
}

func newBiMap[K, V comparable](m map[K]V) containers.BiMap[K, V] {
	bm := containers.BiMap[K, V]{}

	for k, v := range m {
		bm.Set(k, v)
	}

	return bm
}
