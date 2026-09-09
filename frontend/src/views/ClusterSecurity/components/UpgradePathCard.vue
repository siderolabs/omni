<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import VulnerabilityList from '@/views/ClusterSecurity/components/VulnerabilityList.vue'
import { diffMatches } from '@/views/ClusterSecurity/util/matchUtils'
import type { ScanResult } from '@/views/ClusterSecurity/util/useClusterVulnerabilityScans'

const { currentVersionScan, scan, version, isPatch } = defineProps<{
  currentVersionScan: ScanResult
  scan: ScanResult
  version: string
  isPatch?: boolean
}>()

const expanded = ref(false)

const diff = computed(() =>
  currentVersionScan.matches && scan.matches
    ? diffMatches(currentVersionScan.matches, scan.matches)
    : undefined,
)

const canExpand = computed(
  () => !!diff.value && (diff.value.resolved.length > 0 || diff.value.introduced.length > 0),
)
</script>

<template>
  <div class="rounded border border-naturals-n5 bg-naturals-n2">
    <div class="flex flex-wrap items-center gap-3 px-4 py-3">
      <TIcon icon="upgrade" class="size-5 shrink-0 text-naturals-n11" aria-hidden="true" />

      <div class="flex flex-1 flex-col">
        <span class="text-sm font-medium text-naturals-n14">Upgrade to {{ version }}</span>
        <span class="text-xs text-naturals-n11">
          {{ isPatch ? 'Latest patch' : 'Next version' }}
        </span>
      </div>

      <TSpinner v-if="scan.loading" class="size-4" />

      <template v-else-if="diff">
        <ul class="flex flex-wrap items-center gap-1.5 text-xs">
          <li
            v-if="diff.resolved.length"
            class="rounded-sm bg-green-700 px-2 py-1 font-medium text-white"
          >
            {{ diff.resolved.length }} fixed
          </li>
          <li class="rounded-sm bg-naturals-n4 px-2 py-1 text-naturals-n11">
            {{ diff.remaining.length }} remaining
          </li>
          <li
            v-if="diff.introduced.length"
            class="rounded-sm bg-red-700 px-2 py-1 font-medium text-white"
          >
            {{ diff.introduced.length }} new
          </li>
          <li
            v-if="!diff.resolved.length && !diff.introduced.length"
            class="rounded-sm bg-naturals-n4 px-2 py-1 text-naturals-n11"
          >
            No change
          </li>
        </ul>

        <TButton
          v-if="canExpand"
          size="sm"
          variant="secondary"
          :icon="expanded ? 'chevron-up' : 'chevron-down'"
          icon-position="right"
          @click="expanded = !expanded"
        >
          Details
        </TButton>
      </template>
    </div>

    <TAlert v-if="scan.error" type="error" title="Scan failed" class="mx-4 mb-3">
      {{ scan.error }}
    </TAlert>

    <div
      v-else-if="expanded && diff"
      class="flex flex-col gap-4 border-t border-naturals-n5 px-4 py-3"
    >
      <section v-if="diff.resolved.length" class="flex flex-col gap-2">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-green-g1">
          <TIcon icon="check-in-circle" class="size-4 shrink-0" aria-hidden="true" />
          Fixed by this upgrade ({{ diff.resolved.length }})
        </h4>
        <VulnerabilityList :matches="diff.resolved" />
      </section>

      <section v-if="diff.introduced.length" class="flex flex-col gap-2">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-red-r1">
          <TIcon icon="warning" class="size-4 shrink-0" aria-hidden="true" />
          Introduced by this upgrade ({{ diff.introduced.length }})
        </h4>
        <VulnerabilityList :matches="diff.introduced" />
      </section>
    </div>
  </div>
</template>
