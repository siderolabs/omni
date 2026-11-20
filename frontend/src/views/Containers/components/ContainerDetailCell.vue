<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import StatusGlyph from '@/components/Status/StatusGlyph.vue'
import type { ContainerRow } from '@/views/Containers/lib/containerRows'
import { stateAppearance, waitingText } from '@/views/Containers/lib/containerStatus'

const { row } = defineProps<{
  row: ContainerRow
}>()

const error = computed(
  () => row.status.error || (row.status.exitCode ? `Exit ${row.status.exitCode}` : undefined),
)
const waiting = computed(() => waitingText(row.status.waitingFor))

// A container that still runs while its error stands (a failing health check) needs attention,
// as its pill says; anything else with an error has failed
const needsAttention = computed(
  () => stateAppearance(row.status.state, row.status.health).tone === 'warning',
)
</script>

<template>
  <!-- An inline message is neutral text: the state's colour sits on the glyph alone -->
  <span v-if="error" class="flex min-w-0 items-center gap-micro">
    <StatusGlyph
      :glyph="needsAttention ? 'warning' : 'danger'"
      class="shrink-0"
      :class="needsAttention ? 'text-status-warning-default' : 'text-status-danger-default'"
      aria-hidden="true"
    />
    <span class="truncate font-mono">{{ error }}</span>
  </span>

  <span v-else-if="waiting" class="block truncate">
    Waiting for
    <span class="font-mono">{{ waiting }}</span>
  </span>
</template>
