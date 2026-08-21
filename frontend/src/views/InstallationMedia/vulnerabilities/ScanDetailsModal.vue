<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import TButton from '@/components/Button/TButton.vue'
import Modal from '@/components/Modals/Modal.vue'
import { downloadAPIFile } from '@/methods'
import { showError } from '@/notification'
import SeverityBadges from '@/views/ClusterSecurity/components/SeverityBadges.vue'
import VulnerabilityList from '@/views/ClusterSecurity/components/VulnerabilityList.vue'
import type { Match } from '@/views/ClusterSecurity/util/ReportTypes'
import {
  getVulnerabilityReportPath,
  useSbomReport,
  useVexReport,
  VulnerabilityReportFile,
} from '@/views/ClusterSecurity/util/securityReports'

const { matches, schematicId, talosVersion, arch } = defineProps<{
  matches: Match[]
  schematicId: string
  talosVersion: string
  arch: PlatformConfigSpecArch
}>()

const open = defineModel<boolean>('open', { default: false })

const severityFilter = ref<string>()
const isDownloadingReport = ref<Partial<Record<VulnerabilityReportFile, boolean>>>({})

const { isDownloading: isDownloadingSbom, download: downloadSbom } = useSbomReport(() => ({
  arch,
  schematicId,
  talosVersion,
}))

const { isDownloading: isDownloadingVEX, download: downloadVEX } = useVexReport(() => ({
  talosVersion,
}))

watchEffect(() => {
  if (open.value) return

  severityFilter.value = undefined
})

const filteredMatches = computed(() =>
  severityFilter.value
    ? matches.filter((m) => m.vulnerability.severity === severityFilter.value)
    : matches,
)

function toggleSeverityFilter(severity: string) {
  severityFilter.value = severityFilter.value === severity ? undefined : severity
}

async function downloadReport(filename: VulnerabilityReportFile) {
  const path = getVulnerabilityReportPath({
    schematicId,
    talosVersion,
    arch,
    filename,
  })

  try {
    isDownloadingReport.value[filename] = true

    await downloadAPIFile(path, filename)
  } catch (e) {
    showError('Failed to download report', e instanceof Error ? e.message : String(e))
  } finally {
    isDownloadingReport.value[filename] = false
  }
}
</script>

<template>
  <Modal v-model:open="open" title="Scan details" cancel-label="Close">
    <template #description>
      <SeverityBadges
        :matches
        :active-filter="severityFilter"
        class="mt-2"
        clickable
        @click-severity="toggleSeverityFilter"
      />
    </template>

    <div class="flex flex-col gap-4">
      <div class="flex items-center gap-4 text-sm">
        Supply chain
        <ul class="flex gap-2">
          <li>
            <TButton
              size="sm"
              class="max-w-max"
              :icon="isDownloadingSbom ? 'loading' : 'arrow-down-tray'"
              icon-position="left"
              :disabled="isDownloadingSbom"
              @click="downloadSbom"
            >
              SBOM
            </TButton>
          </li>

          <li>
            <TButton
              size="sm"
              class="max-w-max"
              :icon="isDownloadingVEX ? 'loading' : 'arrow-down-tray'"
              icon-position="left"
              :disabled="isDownloadingVEX"
              @click="downloadVEX"
            >
              VEX
            </TButton>
          </li>
        </ul>
      </div>

      <div class="flex items-center gap-4 text-sm">
        Scan report
        <ul class="flex gap-2">
          <li
            v-for="[label, filename] in [
              ['JSON', VulnerabilityReportFile.JSON],
              ['SARIF', VulnerabilityReportFile.SARIF],
              ['CycloneDX', VulnerabilityReportFile.CycloneDX],
              ['Table', VulnerabilityReportFile.TABLE],
            ]"
            :key="label"
          >
            <TButton
              size="sm"
              :icon="isDownloadingReport[filename] ? 'loading' : 'arrow-down-tray'"
              icon-position="left"
              :disabled="isDownloadingReport[filename]"
              @click="downloadReport(filename)"
            >
              {{ label }}
            </TButton>
          </li>
        </ul>
      </div>

      <VulnerabilityList :matches="filteredMatches" />
    </div>
  </Modal>
</template>
