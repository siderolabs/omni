// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useFetch } from '@vueuse/core'
import { computed, type MaybeRefOrGetter, ref, toValue } from 'vue'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { downloadAPIFile } from '@/methods'
import { showError } from '@/notification'
import type { VulnerabilityReport } from '@/views/ClusterSecurity/util/ReportTypes'

export enum VulnerabilityReportFile {
  JSON = 'report.json',
  SARIF = 'report.sarif',
  CycloneDX = 'report.cdx',
  TABLE = 'report.table',
}

export function getVulnerabilityReportPath({
  schematicId,
  talosVersion,
  arch,
  filename,
}: {
  schematicId: string
  talosVersion: string
  arch: PlatformConfigSpecArch
  filename: VulnerabilityReportFile
}) {
  return `/api/vulns/${schematicId}/${talosVersion}/${archToString(arch)}/${filename}`
}

/** The architecture as it appears in a report URL, e.g. `amd64` or `arm64`. */
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

/**
 * The architecture an `amd64`/`arm64` string names, or `undefined` for anything else - a machine
 * reporting an architecture no report is published for simply has no reports, which is not an error.
 */
export function archFromString(arch?: string): PlatformConfigSpecArch | undefined {
  switch (arch) {
    case 'arm64':
      return PlatformConfigSpecArch.ARM64
    case 'amd64':
      return PlatformConfigSpecArch.AMD64
    default:
      return undefined
  }
}

export function useVulnerabilityReport(
  opts: MaybeRefOrGetter<{
    schematicId: string
    talosVersion: string
    arch: PlatformConfigSpecArch
  }>,
) {
  const path = computed(() =>
    getVulnerabilityReportPath({
      ...toValue(opts),
      filename: VulnerabilityReportFile.JSON,
    }),
  )

  const { data, isFetching, error } = useFetch(path, {
    // Omni reports a failure as `{"status": "..."}`, which says more than the HTTP status text
    // useFetch reports by default - "scan report not found" rather than "Not Found".
    onFetchError(ctx) {
      const status = (ctx.data as { status?: string } | null)?.status

      return { data: null, error: status ? new Error(status) : ctx.error }
    },
  }).json<VulnerabilityReport>()

  return {
    data,
    loading: isFetching,
    err: error,
  }
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
    const { arch: archEnum, talosVersion, schematicId } = toValue(opts)
    const arch = archToString(archEnum)

    try {
      isDownloading.value = true

      await downloadAPIFile(
        `/api/sbom/${schematicId}/${talosVersion}/${arch}`,
        `${schematicId}-v${talosVersion}-${arch}-sbom.spdx.json`,
      )
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

      await downloadAPIFile(`/api/vex/${talosVersion}`, `v${talosVersion}.vex.json`)
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
