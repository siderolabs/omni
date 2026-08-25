// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computedAsync } from '@vueuse/core'
import { type MaybeRefOrGetter, ref, toValue } from 'vue'

import {
  Arch,
  ImageFactoryService,
  VulnerabilityReportFormat,
} from '@/api/omni/imagefactory/imagefactory.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { downloadFile } from '@/methods'
import { showError } from '@/notification'
import type { VulnerabilityReport } from '@/views/ClusterSecurity/util/ReportTypes'

export function archFromConfigArch(arch: PlatformConfigSpecArch) {
  switch (arch) {
    case PlatformConfigSpecArch.ARM64:
      return Arch.ARM64
    case PlatformConfigSpecArch.AMD64:
      return Arch.AMD64
    default:
      throw new Error(`Unexpected arch "${arch}" received`)
  }
}

export function archToString(arch: PlatformConfigSpecArch) {
  switch (arch) {
    case PlatformConfigSpecArch.ARM64:
      return 'arm64'
    case PlatformConfigSpecArch.AMD64:
      return 'amd64'
    default:
      throw new Error(`Unexpected arch "${arch}" received`)
  }
}

export function archFromString(arch?: string) {
  switch (arch) {
    case 'arm64':
      return PlatformConfigSpecArch.ARM64
    case 'amd64':
      return PlatformConfigSpecArch.AMD64
    default:
      return undefined
  }
}

/**
 * The `bytes` payload of an artifact response, as text.
 *
 * The generated client types a `bytes` field as `Uint8Array`, but the gateway marshals it to base64
 * and what actually arrives is that string.
 */
export function artifactText(data?: Uint8Array) {
  const binary = window.atob(String(data ?? ''))

  // The bytes are UTF-8, and atob yields one character per byte, so they have to be decoded as such
  // - a report describing a CVE in anything but ASCII would otherwise come back mojibake.
  return new TextDecoder().decode(Uint8Array.from(binary, (char) => char.charCodeAt(0)))
}

/** Fetches the vulnerability scan report of a schematic, as the JSON the report views render. */
export function useVulnerabilityReport(
  opts: MaybeRefOrGetter<{
    schematicId: string
    talosVersion: string
    arch: PlatformConfigSpecArch
  }>,
) {
  const loading = ref(false)
  const err = ref<Error>()

  const data = computedAsync<VulnerabilityReport | undefined>(
    async () => {
      const { schematicId, talosVersion, arch } = toValue(opts)

      err.value = undefined

      try {
        const { data } = await ImageFactoryService.VulnerabilityReport({
          schematic_id: schematicId,
          talos_version: talosVersion,
          arch: archFromConfigArch(arch),
          format: VulnerabilityReportFormat.JSON,
        })

        return JSON.parse(artifactText(data))
      } catch (e) {
        err.value = e instanceof Error ? e : new Error(String(e))

        return undefined
      }
    },
    undefined,
    loading,
  )

  return { data, loading, err }
}

const reportFilenames: Record<
  Exclude<VulnerabilityReportFormat, VulnerabilityReportFormat.UNKNOWN_FORMAT>,
  string
> = {
  [VulnerabilityReportFormat.JSON]: 'report.json',
  [VulnerabilityReportFormat.SARIF]: 'report.sarif',
  [VulnerabilityReportFormat.CYCLONEDX]: 'report.cdx',
  [VulnerabilityReportFormat.TABLE]: 'report.table',
}

/** Downloads the vulnerability scan report of a schematic, in the given format. */
export async function downloadVulnerabilityReport({
  schematicId,
  talosVersion,
  arch,
  format,
}: {
  schematicId: string
  talosVersion: string
  arch: PlatformConfigSpecArch
  format: VulnerabilityReportFormat
}) {
  if (format === VulnerabilityReportFormat.UNKNOWN_FORMAT) {
    throw new Error('VulnerabilityReportFormat is required')
  }

  const { data } = await ImageFactoryService.VulnerabilityReport({
    schematic_id: schematicId,
    talos_version: talosVersion,
    arch: archFromConfigArch(arch),
    format,
  })

  downloadArtifact(data, reportFilenames[format])
}

export function useSbomReport(
  opts: MaybeRefOrGetter<{
    schematicId: string
    talosVersion: string
    arch: PlatformConfigSpecArch
  }>,
) {
  const isDownloading = ref(false)

  async function download() {
    const { arch, talosVersion, schematicId } = toValue(opts)

    try {
      isDownloading.value = true

      const { data } = await ImageFactoryService.SBOM({
        schematic_id: schematicId,
        talos_version: talosVersion,
        arch: archFromConfigArch(arch),
      })

      downloadArtifact(data, `${schematicId}-v${talosVersion}-${archToString(arch)}-sbom.spdx.json`)
    } catch (e) {
      showError('Failed to download SBOM', e instanceof Error ? e.message : String(e))
    } finally {
      isDownloading.value = false
    }
  }

  return {
    isDownloading,
    download,
  }
}

export function useVexReport(
  opts: MaybeRefOrGetter<{
    talosVersion: string
  }>,
) {
  const isDownloading = ref(false)

  async function download() {
    const { talosVersion } = toValue(opts)

    try {
      isDownloading.value = true

      const { data } = await ImageFactoryService.VEXDocument({ talos_version: talosVersion })

      downloadArtifact(data, `v${talosVersion}.vex.json`)
    } catch (e) {
      showError('Failed to download VEX document', e instanceof Error ? e.message : String(e))
    } finally {
      isDownloading.value = false
    }
  }

  return {
    isDownloading,
    download,
  }
}

/**
 * Saves an artifact response to a file.
 *
 * The payload arrives base64-encoded, and SPDX bundles routinely run to several megabytes - too
 * large to hand to the browser as a `data:` URL - so it is decoded into a blob instead.
 */
function downloadArtifact(data: Uint8Array | undefined, filename: string) {
  const base64 = String(data ?? '')

  // The gateway drops an empty `bytes` field entirely, so nothing on the wire means the factory
  // served an empty artifact. Saving that would silently produce a zero-byte file.
  if (!base64) {
    throw new Error('the image factory returned an empty response')
  }

  const binary = window.atob(base64)
  const url = window.URL.createObjectURL(
    new Blob([Uint8Array.from(binary, (char) => char.charCodeAt(0))], {
      type: 'application/octet-stream',
    }),
  )

  downloadFile(url, filename)
  window.URL.revokeObjectURL(url)
}
