<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TListItem from '@/components/List/TListItem.vue'
import Modal from '@/components/Modals/Modal.vue'
import { useFeatures } from '@/methods/features'
import { cn } from '@/methods/utils'
import {
  countBySeverity,
  getCveId,
  getPreferredVulnURL,
  getScore,
  sortMatches,
} from '@/views/InstallationMedia/vulnerabilities/matchUtils'
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

const sortedMatches = computed(() =>
  sortMatches(matches).map((m) => ({
    ...m,
    score: getScore(m),
    url: getPreferredVulnURL(m.vulnerability, m.relatedVulnerabilities),
    cveID: getCveId(m),
  })),
)

const counts = computed(() => countBySeverity(matches))
</script>

<template>
  <Modal v-model:open="open" title="Scan details" cancel-label="Close">
    <template #description>
      <ul v-if="counts" class="mt-2 flex flex-wrap items-center gap-1.5">
        <li
          v-for="[sev, count] in counts"
          :key="sev"
          :class="
            cn('rounded-sm bg-naturals-n4 px-2 py-1 text-xs text-naturals-n11', {
              'bg-red-600 text-white': sev === 'Critical',
              'bg-orange-700 text-white': sev === 'High',
              'bg-orange-500 text-white': sev === 'Medium',
              'bg-yellow-500 text-black': sev === 'Low',
            })
          "
        >
          {{ count }} {{ sev }}
        </li>
      </ul>
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

      <div class="flex items-center gap-1 bg-naturals-n2 px-2 py-2 text-xs text-naturals-n13">
        <div class="w-6 shrink-0"></div>
        <div class="grid flex-1 grid-cols-5 items-center gap-x-4 text-xs uppercase">
          <div>CVE</div>
          <div>Score</div>
          <div>Package</div>
          <div>Version</div>
          <div>Fix</div>
        </div>
      </div>

      <TListItem
        v-for="{ vulnerability, artifact, url, cveID, score } in sortedMatches"
        :key="cveID"
        disable-border-on-expand
      >
        <template #default>
          <div class="grid grid-cols-5 items-center gap-x-4 text-xs">
            <div class="whitespace-nowrap">
              <a
                v-if="url"
                target="_blank"
                rel="noopener noreferrer"
                :href="url"
                class="link-primary inline-flex items-center gap-1 font-mono"
              >
                {{ cveID }}
                <TIcon icon="external-link" class="size-3 shrink-0" aria-hidden="true" />
              </a>
              <span v-else class="font-mono text-naturals-n11">{{ cveID }}</span>
            </div>

            <div class="font-mono">
              {{ score?.toFixed(1) ?? '---' }}

              <span
                :class="
                  cn(
                    'rounded-sm bg-naturals-n7 px-1 py-0.5 text-xs font-medium text-naturals-n11',
                    {
                      'bg-red-600 text-white': vulnerability.severity === 'Critical',
                      'bg-orange-700 text-white': vulnerability.severity === 'High',
                      'bg-orange-500 text-white': vulnerability.severity === 'Medium',
                      'bg-yellow-500 text-black': vulnerability.severity === 'Low',
                    },
                  )
                "
              >
                {{ vulnerability.severity.charAt(0).toUpperCase() }}
              </span>
            </div>

            <div class="truncate font-medium">{{ artifact.name }}</div>

            <div class="whitespace-nowrap">
              <span class="resource-label label-red font-mono">{{ artifact.version }}</span>
            </div>

            <div class="flex flex-wrap gap-1">
              <template v-if="vulnerability.fix.versions.length">
                <span
                  v-for="v in vulnerability.fix.versions"
                  :key="v"
                  class="resource-label label-green font-mono"
                >
                  {{ v }}
                </span>
              </template>
              <span v-else class="text-naturals-n8">—</span>
            </div>
          </div>
        </template>

        <template v-if="vulnerability.description" #details>
          <p class="text-xs whitespace-pre-wrap text-naturals-n11">
            {{ vulnerability.description }}
          </p>
        </template>
      </TListItem>
    </div>
  </Modal>
</template>
