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
  <div class="rounded border border-border-strong bg-surface-card">
    <div class="flex flex-wrap items-center gap-snug px-compact py-snug">
      <TIcon icon="upgrade" class="size-5 shrink-0 text-content-secondary" aria-hidden="true" />

      <div class="flex flex-1 flex-col">
        <span class="text-sm font-medium text-content-emphasis">Upgrade to {{ version }}</span>
        <span class="text-xs text-content-secondary">
          {{ isPatch ? 'Latest patch' : 'Next version' }}
        </span>
      </div>

      <TSpinner v-if="scan.loading" class="size-4" />

      <template v-else-if="diff">
        <ul class="flex flex-wrap items-center gap-1.5 text-xs">
          <li
            v-if="diff.resolved.length"
            class="rounded-sm bg-status-success-subtle px-tight py-micro font-medium text-status-success-text ring-1 ring-status-success-subtle-border ring-inset"
          >
            {{ diff.resolved.length }} fixed
          </li>
          <li class="rounded-sm bg-surface-hover px-tight py-micro text-content-default">
            {{ diff.remaining.length }} remaining
          </li>
          <li
            v-if="diff.introduced.length"
            class="rounded-sm bg-status-danger-subtle px-tight py-micro font-medium text-status-danger-text ring-1 ring-status-danger-subtle-border ring-inset"
          >
            {{ diff.introduced.length }} new
          </li>
          <li
            v-if="!diff.resolved.length && !diff.introduced.length"
            class="rounded-sm bg-surface-hover px-tight py-micro text-content-default"
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

    <TAlert v-if="scan.error" type="error" title="Scan failed" class="mx-compact mb-snug">
      {{ scan.error }}
    </TAlert>

    <TAlert
      v-else-if="scan.notFound"
      type="info"
      title="No report available"
      class="mx-compact mb-snug"
    >
      No vulnerability report was found for this schematic on Talos {{ version }}. The schematic may
      not exist on the image factory yet, or its scan may not have been published yet.
    </TAlert>

    <div
      v-else-if="expanded && diff"
      class="flex flex-col gap-compact border-t border-border-strong px-compact py-snug"
    >
      <section v-if="diff.resolved.length" class="flex flex-col gap-tight">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-status-success-text">
          <TIcon icon="check-circle" class="size-4 shrink-0" aria-hidden="true" />
          Fixed by this upgrade ({{ diff.resolved.length }})
        </h4>
        <VulnerabilityList :matches="diff.resolved" />
      </section>

      <section v-if="diff.introduced.length" class="flex flex-col gap-tight">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-status-danger-text">
          <TIcon icon="exclamation-triangle" class="size-4 shrink-0" aria-hidden="true" />
          Introduced by this upgrade ({{ diff.introduced.length }})
        </h4>
        <VulnerabilityList :matches="diff.introduced" />
      </section>
    </div>
  </div>
</template>
