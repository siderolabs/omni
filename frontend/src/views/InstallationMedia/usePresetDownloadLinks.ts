// Copyright (c) 2026 Sidero Labs, Inc.
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
import {
  CloudPlatformConfigType,
  MetalPlatformConfigType,
  PlatformMetalID,
  VirtualNamespace,
} from '@/api/resources'
import { getDocsLink } from '@/methods'
import { useFeatures, useIsEnterprise } from '@/methods/features'
import { withImageFactoryAuth } from '@/methods/useImageFactoryAuth'
import { useResolvedFactory } from '@/methods/useResolvedFactory'
import { useResourceGet } from '@/methods/useResourceGet'

export function usePresetDownloadLinks(
  schematicId: MaybeRefOrGetter<string>,
  presetRef: MaybeRefOrGetter<InstallationMediaConfigSpec>,
) {
  const isMetal = computed(() => !toValue(presetRef).cloud && !toValue(presetRef).sbc)

  const isEnterpriseFactory = useIsEnterprise()
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
      id: PlatformMetalID,
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

  const {
    url: factoryBaseURLRaw,
    pxeUrl: factoryPxeBaseURLRaw,
    credentials,
  } = useResolvedFactory(() => toValue(presetRef).image_factory_url)

  const factoryBaseURL = computed(() =>
    factoryBaseURLRaw.value
      ? withImageFactoryAuth(factoryBaseURLRaw.value, credentials.value)
      : undefined,
  )

  const factoryPxeBaseURL = computed(() =>
    factoryPxeBaseURLRaw.value
      ? withImageFactoryAuth(factoryPxeBaseURLRaw.value, credentials.value)
      : undefined,
  )

  // If the resolved factory is no longer configured in Omni, this preset is orphaned
  const orphaned = computed(() => !factoryBaseURL.value)

  const imageBaseURL = computed(() =>
    factoryBaseURL.value
      ? `${factoryBaseURL.value}/image/${toValue(schematicId)}/${toValue(presetRef).talos_version}`
      : undefined,
  )

  const pxeBaseURL = computed(() =>
    factoryPxeBaseURL.value
      ? `${factoryPxeBaseURL.value}/pxe/${toValue(schematicId)}/${toValue(presetRef).talos_version}`
      : undefined,
  )

  const sbcDiskImagePath = computed(() =>
    imageBaseURL.value ? `${imageBaseURL.value}/metal-${arch.value}.raw.xz` : undefined,
  )

  const pxeBootURL = computed(() =>
    selectedPlatform.value && pxeBaseURL.value
      ? `${pxeBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}${secureBootSuffix.value}`
      : undefined,
  )

  const platformDiskImagePath = computed(() =>
    selectedPlatform.value && imageBaseURL.value
      ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}${secureBootSuffix.value}.${selectedPlatform.value.spec.disk_image_suffix}`
      : undefined,
  )

  const qcow2DiskImagePath = computed(() =>
    selectedPlatform.value && imageBaseURL.value
      ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}.qcow2`
      : undefined,
  )

  const isoPath = computed(() =>
    selectedPlatform.value && imageBaseURL.value
      ? `${imageBaseURL.value}/${selectedPlatform.value.metadata.id}-${arch.value}${secureBootSuffix.value}.iso`
      : undefined,
  )

  interface DownloadLink {
    label: string
    link: string
    linkBare: string
    linkSha256?: string
    linkSha512?: string
    copyOnly?: boolean
    documentation?: {
      label: string
      link: string
    }
  }

  function constructSearchParams(imageUrl: string) {
    const accountName = features.value?.spec.account?.name ?? 'default'
    const talosVersion = toValue(presetRef).talos_version
    const imageName = imageUrl.split('/').at(-1)

    return new URLSearchParams([['filename', `omni-${accountName}-${talosVersion}-${imageName}`]])
  }

  function toDownloadLink({
    label,
    link,
    search,
    withChecksums,
  }: {
    label: string
    link: string
    search?: URLSearchParams
    withChecksums?: boolean
  }): DownloadLink {
    const url = new URL(link)

    if (search) {
      for (const [key, value] of search) {
        url.searchParams.set(key, value)
      }
    }

    const downloadLink: DownloadLink = {
      label,
      link: url.toString(),
      linkBare: link,
    }

    if (withChecksums) {
      downloadLink.linkSha256 = `${link}.sha256`
      downloadLink.linkSha512 = `${link}.sha512`
    }

    return downloadLink
  }

  const links = computed<DownloadLink[]>(() => {
    const preset = toValue(presetRef)

    if (preset.sbc && sbcDiskImagePath.value) {
      return [
        toDownloadLink({
          label: 'Disk Image',
          link: sbcDiskImagePath.value,
          search: constructSearchParams(sbcDiskImagePath.value),
          withChecksums: true,
        }),
      ]
    }

    if (!selectedPlatform.value?.spec.boot_methods) {
      return []
    }

    return selectedPlatform.value.spec.boot_methods
      .filter((m) => !isEnterpriseFactory.value || m !== PlatformConfigSpecBootMethod.PXE)
      .flatMap<DownloadLink>((bootMethod) => {
        switch (bootMethod) {
          case PlatformConfigSpecBootMethod.DISK_IMAGE: {
            if (!platformDiskImagePath.value) return []

            if (preset.secure_boot) {
              return {
                ...toDownloadLink({
                  label: 'SecureBoot Disk Image',
                  link: platformDiskImagePath.value,
                  search: constructSearchParams(platformDiskImagePath.value),
                  withChecksums: true,
                }),
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
                toDownloadLink({
                  label: 'Disk Image (raw)',
                  link: platformDiskImagePath.value,
                  search: constructSearchParams(platformDiskImagePath.value),
                  withChecksums: true,
                }),
                toDownloadLink({
                  label: 'Disk Image (qcow2)',
                  link: qcow2DiskImagePath.value,
                  search: constructSearchParams(qcow2DiskImagePath.value),
                  withChecksums: true,
                }),
              ]
            }

            return toDownloadLink({
              label: 'Disk Image',
              link: platformDiskImagePath.value,
              search: constructSearchParams(platformDiskImagePath.value),
              withChecksums: true,
            })
          }

          case PlatformConfigSpecBootMethod.ISO: {
            if (!isoPath.value) return []

            if (preset.secure_boot) {
              return {
                ...toDownloadLink({
                  label: 'SecureBoot ISO',
                  link: isoPath.value,
                  search: constructSearchParams(isoPath.value),
                  withChecksums: true,
                }),
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
              ...toDownloadLink({
                label: 'ISO',
                link: isoPath.value,
                search: constructSearchParams(isoPath.value),
                withChecksums: true,
              }),
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
              ...toDownloadLink({
                label: preset.secure_boot
                  ? 'SecureBoot PXE (iPXE script)'
                  : 'PXE boot (iPXE script)',
                link: pxeBootURL.value,
              }),
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

  return { links, orphaned }
}
