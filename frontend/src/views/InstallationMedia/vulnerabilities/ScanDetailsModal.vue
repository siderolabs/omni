<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import TButton from '@/components/Button/TButton.vue'
import Modal from '@/components/Modals/Modal.vue'
import { useFeatures } from '@/methods/features'
import SeverityBadges from '@/views/ClusterSecurity/components/SeverityBadges.vue'
import VulnerabilityList from '@/views/ClusterSecurity/components/VulnerabilityList.vue'
import type { Match } from '@/views/InstallationMedia/vulnerabilities/ReportTypes'

const {
  matches,
  schematicId,
  talosVersion,
  arch: archEnum,
} = defineProps<{
  matches: Match[]
  schematicId: string
  talosVersion: string
  arch: PlatformConfigSpecArch
}>()

const open = defineModel<boolean>('open', { default: false })

const { data: features } = useFeatures()

const arch = computed(() => {
  switch (archEnum) {
    case PlatformConfigSpecArch.ARM64:
      return 'arm64'
    case PlatformConfigSpecArch.AMD64:
      return 'amd64'
    default:
      throw new Error(`Unexpected arch "${archEnum}" received`)
  }
})
</script>

<template>
  <Modal v-model:open="open" title="Scan details" cancel-label="Close">
    <template #description>
      <SeverityBadges :matches class="mt-2" />
    </template>

    <div class="flex flex-col">
      <div class="mb-4 flex items-center gap-4 text-sm">
        Download report
        <ul class="flex gap-2">
          <li
            v-for="[label, filename] in [
              ['JSON', 'report.json'],
              ['SARIF', 'report.sarif'],
              ['CycloneDX', 'report.cdx'],
              ['Table', 'report.table'],
            ]"
            :key="label"
          >
            <TButton
              is="a"
              :href="`${features?.spec.image_factory_base_url}/scans/${schematicId}/v${talosVersion}/${arch}/${filename}`"
              target="_blank"
              rel="noopener noreferrer"
              size="sm"
              icon="arrow-down-tray"
              icon-position="left"
            >
              {{ label }}
            </TButton>
          </li>
        </ul>
      </div>

      <VulnerabilityList :matches />
    </div>
  </Modal>
</template>
