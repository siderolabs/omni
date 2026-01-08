// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import {
  type PlatformConfigSpec,
  PlatformConfigSpecArch,
  PlatformConfigSpecBootMethod,
} from '@/api/omni/specs/virtual.pb'
import { CloudPlatformConfigType, MetalPlatformConfigType, VirtualNamespace } from '@/api/resources'
import { getDocsLink } from '@/methods'
import { useFeatures } from '@/methods/features'
import { useResourceGet } from '@/methods/useResourceGet'

export function usePresetDownloadLinks(
  schematicId: MaybeRefOrGetter<string>,
  presetRef: MaybeRefOrGetter<InstallationMediaConfigSpec>,
) {
  const isMetal = computed(() => !toValue(presetRef).cloud && !toValue(presetRef).sbc)

  const { data: features } = useFeatures()

  const { data: selectedCloudProvider } = useResourceGet<PlatformConfigSpec>(() => ({
    skip: !toValue(presetRef).cloud,
    runtime: Runtime.Omni,
    resource: {
      namespace: VirtualNamespace,
      type: CloudPlatformConfigType,
      id: toValue(presetRef).cloud?.platform,
    },
  }))

  const { data: metalProvider } = useResourceGet<PlatformConfigSpec>(() => ({
    skip: !isMetal.value,
    runtime: Runtime.Omni,
    resource: {
      namespace: VirtualNamespace,
      type: MetalPlatformConfigType,
      id: 'metal',
    },
  }))

  const selectedPlatform = computed(() =>
    isMetal.value ? metalProvider.value : selectedCloudProvider.value,
  )

  const secureBootSuffix = computed(() => (toValue(presetRef).secure_boot ? '-secureboot' : ''))
  const arch = computed(() => {
    const preset = toValue(presetRef)

    switch (preset.architecture) {
      case PlatformConfigSpecArch.AMD64:
        return 'amd64'
      case PlatformConfigSpecArch.ARM64:
        return 'arm64'
      default:
        return preset.sbc ? 'arm64' : undefined
    }
  })

  const imageBaseURL = computed(
    () =>
      `${features.value?.spec.image_factory_base_url}/image/${toValue(schematicId)}/${toValue(presetRef).talos_version}`,
  )

  const sbcDiskImagePath = computed(() => `${imageBaseURL.value}/metal-${arch.value}.raw.xz`)

  const pxeBootURL = computed(() =>
    selectedPlatform.value
      ? `${features.value?.spec.image_factory_pxe_base_url}/${toValue(schematicId)}/${toValue(presetRef).talos_version}/${selectedPlatform.value.metadata.id}-${arch.value}${secureBootSuffix.value}`
      : undefined,
  )

  const platformDiskImagePath = computed(() =>
    selectedPlatform.value
      ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}${secureBootSuffix.value}.${selectedPlatform.value.spec.disk_image_suffix}`
      : undefined,
  )

  const qcow2DiskImagePath = computed(() =>
    selectedPlatform.value
      ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}.qcow2`
      : undefined,
  )

  const isoPath = computed(() =>
    selectedPlatform.value
      ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}${secureBootSuffix.value}.iso`
      : undefined,
  )

  const links = computed(() => {
    const preset = toValue(presetRef)

    if (preset.sbc) {
      return [{ label: 'Disk Image', link: sbcDiskImagePath.value }]
    }

    if (!selectedPlatform.value?.spec.boot_methods) {
      return []
    }

    interface DownloadLink {
      label: string
      link: string
      copyOnly?: boolean
      documentation?: {
        label: string
        link: string
      }
    }

    return selectedPlatform.value.spec.boot_methods.flatMap<DownloadLink>((bootMethod) => {
      switch (bootMethod) {
        case PlatformConfigSpecBootMethod.DISK_IMAGE: {
          if (!platformDiskImagePath.value) return []

          if (preset.secure_boot) {
            return {
              label: 'SecureBoot Disk Image',
              link: platformDiskImagePath.value,
              documentation: isMetal.value
                ? {
                    label: 'SecureBoot documentation',
                    link: getDocsLink(
                      'talos',
                      '/platform-specific-installations/bare-metal-platforms/secureboot',
                      { talosVersion: preset.talos_version },
                    ),
                  }
                : undefined,
            }
          }

          if (isMetal.value && qcow2DiskImagePath.value) {
            return [
              { label: 'Disk Image (raw)', link: platformDiskImagePath.value },
              { label: 'Disk Image (qcow2)', link: qcow2DiskImagePath.value },
            ]
          }

          return { label: 'Disk Image', link: platformDiskImagePath.value }
        }

        case PlatformConfigSpecBootMethod.ISO: {
          if (!isoPath.value) return []

          if (preset.secure_boot) {
            return {
              label: 'SecureBoot ISO',
              link: isoPath.value,
              documentation: {
                label: 'SecureBoot documentation',
                link: getDocsLink(
                  'talos',
                  '/platform-specific-installations/bare-metal-platforms/secureboot',
                  { talosVersion: preset.talos_version },
                ),
              },
            }
          }

          return {
            label: 'ISO',
            link: isoPath.value,
            documentation: isMetal.value
              ? {
                  label: 'ISO documentation',
                  link: getDocsLink(
                    'talos',
                    '/platform-specific-installations/bare-metal-platforms/iso',
                    { talosVersion: preset.talos_version },
                  ),
                }
              : undefined,
          }
        }

        case PlatformConfigSpecBootMethod.PXE: {
          if (!pxeBootURL.value) return []

          return {
            label: preset.secure_boot ? 'SecureBoot PXE (iPXE script)' : 'PXE boot (iPXE script)',
            link: pxeBootURL.value,
            copyOnly: true,
            documentation: isMetal.value
              ? {
                  label: 'PXE documentation',
                  link: getDocsLink(
                    'talos',
                    '/platform-specific-installations/bare-metal-platforms/pxe',
                    { talosVersion: preset.talos_version },
                  ),
                }
              : undefined,
          }
        }
      }

      return []
    })
  })

  return { links }
}
