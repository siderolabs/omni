<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import StatusGlyph from '@/components/Status/StatusGlyph.vue'
import type { ContainerRow } from '@/views/Containers/lib/containerRows'
import { stateAppearance, waitingText } from '@/views/Containers/lib/containerStatus'

const { row, search = '' } = defineProps<{
  row: ContainerRow
  /** Highlights where the error matches the search. */
  search?: string
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
  <!-- An inline message is neutral text, with the state's colour on the glyph alone. It wraps
  rather than truncating: there's no tooltip, and a phone has no hover anyway. -->
  <span v-if="error" class="flex items-start gap-micro">
    <StatusGlyph
      :glyph="needsAttention ? 'warning' : 'danger'"
      :tone="needsAttention ? 'warning' : 'danger'"
      class="mt-px shrink-0"
      aria-hidden="true"
    />
    <WordHighlighter
      :query="search.trim()"
      :text-to-highlight="error"
      highlight-class="search-match"
      class="font-mono wrap-anywhere"
    />
  </span>

  <span v-else-if="waiting" class="block wrap-anywhere">
    Waiting for
    <span class="font-mono">{{ waiting }}</span>
  </span>
</template>
