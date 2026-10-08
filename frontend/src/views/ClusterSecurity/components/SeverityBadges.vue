<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import TIcon from '@/components/Icon/TIcon.vue'
import type { StatusTone } from '@/components/Status/StatusPill.vue'
import { cn } from '@/methods/utils'
import { countBySeverity, toneFromSeverity } from '@/views/ClusterSecurity/util/matchUtils'
import type { Match } from '@/views/ClusterSecurity/util/ReportTypes'

const { matches, activeFilter } = defineProps<{
  matches: Match[]
  activeFilter?: string
  clickable?: boolean
}>()

const counts = computed(() => countBySeverity(matches))

defineEmits<{
  clickSeverity: [string]
}>()

const statusDotClass: Record<StatusTone, string> = {
  success: 'bg-status-success-default',
  warning: 'bg-status-warning-default',
  danger: 'bg-status-danger-default',
  info: 'bg-status-info-default',
}
</script>

<template>
  <p v-if="!matches.length" class="flex items-center gap-1.5 text-xs text-status-success-text">
    <TIcon icon="check-circle" class="size-4 shrink-0" aria-hidden="true" />
    No vulnerabilities
  </p>
  <ul v-else class="flex flex-wrap items-center gap-1.5">
    <li
      v-for="[sev, count] in counts"
      :key="sev"
      :class="
        cn(
          'flex items-center gap-1.5 rounded-sm border border-border-strong px-tight py-micro text-xs text-content-secondary',
          {
            'cursor-pointer transition-colors hover:bg-surface-hover': clickable,
            'bg-surface-hover text-content-default': activeFilter === sev,
            'text-content-muted': activeFilter && activeFilter !== sev,
          },
        )
      "
      :role="clickable ? 'button' : undefined"
      @click="$emit('clickSeverity', sev)"
    >
      <span
        class="size-2 shrink-0 rounded-full"
        :class="statusDotClass[toneFromSeverity(sev)]"
        aria-hidden="true"
      />
      <span class="font-semibold text-content-emphasis">{{ count }}</span>
      {{ sev }}
    </li>
  </ul>
</template>
