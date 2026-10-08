<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import type { StatusGlyphType } from '@/components/Status/StatusGlyph.vue'

export type StatusTone = 'success' | 'warning' | 'danger' | 'info'

/** How a state is drawn as a pill. */
export interface StatusAppearance {
  tone: StatusTone
  glyph: StatusGlyphType
}
</script>

<script setup lang="ts">
import { type ClassValue } from 'vue'

import StatusGlyph from '@/components/Status/StatusGlyph.vue'
import { cn } from '@/methods/utils'

const { tone, glyph } = defineProps<{
  tone: StatusTone
  /** A level or a ranking, such as a severity, takes `dot`: the word carries it, and a state glyph would read as pass or fail. */
  glyph?: StatusGlyphType | 'dot'
  class?: ClassValue
}>()

/** The glyph a pill takes when it is given only a tone. */
const glyphFromTone: Record<StatusTone, StatusGlyphType> = {
  success: 'success',
  danger: 'danger',
  warning: 'warning',
  info: 'neutral',
}

const statusPillClass: Record<StatusTone, string> = {
  success: 'bg-status-success-subtle text-status-success-text ring-status-success-subtle-border',
  warning: 'bg-status-warning-subtle text-status-warning-text ring-status-warning-subtle-border',
  danger: 'bg-status-danger-subtle text-status-danger-text ring-status-danger-subtle-border',
  info: 'bg-status-info-subtle text-status-info-text ring-status-info-subtle-border',
}

/** The glyph is a graphic: it takes the state's `default`, which holds 3:1 on the chip's pastel as well as on a surface. */
const statusGlyphClass: Record<StatusTone, string> = {
  success: 'text-status-success-default',
  warning: 'text-status-warning-default',
  danger: 'text-status-danger-default',
  info: 'text-status-info-default',
}
</script>

<template>
  <span
    :class="
      cn(
        'inline-flex w-fit items-center rounded-full py-px pr-tight text-xs font-semibold whitespace-nowrap ring-1 ring-inset',
        glyph === 'dot' ? 'gap-1.5 pl-1.75' : 'gap-micro pl-micro',
        statusPillClass[tone],
        $props.class,
      )
    "
  >
    <span
      v-if="glyph === 'dot'"
      class="size-2 shrink-0 rounded-full bg-current"
      aria-hidden="true"
    />
    <StatusGlyph
      v-else
      :glyph="glyph ?? glyphFromTone[tone]"
      class="shrink-0"
      :class="statusGlyphClass[tone]"
      aria-hidden="true"
    />
    <slot />
  </span>
</template>
