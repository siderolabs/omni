<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import TIcon from '@/components/Icon/TIcon.vue'
import { cn } from '@/methods/utils'
import { countBySeverity } from '@/views/InstallationMedia/vulnerabilities/matchUtils'
import type { Match } from '@/views/InstallationMedia/vulnerabilities/ReportTypes'

const { matches } = defineProps<{ matches: Match[] }>()

const counts = computed(() => countBySeverity(matches))
</script>

<template>
  <p v-if="!matches.length" class="flex items-center gap-1.5 text-xs text-green-g1">
    <TIcon icon="check-in-circle" class="size-4 shrink-0" aria-hidden="true" />
    No vulnerabilities
  </p>
  <ul v-else class="flex flex-wrap items-center gap-1.5">
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
