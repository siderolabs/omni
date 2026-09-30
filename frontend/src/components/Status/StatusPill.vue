<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
export type StatusTone = 'success' | 'warning' | 'danger' | 'info'

/**
 * What a state means, drawn as a shape so it reads without its colour.
 * The set is closed: a new state takes the glyph of the kind it belongs to.
 */
export type StatusGlyph = 'success' | 'danger' | 'warning' | 'progress' | 'neutral' | 'unknown'

/** How a state is drawn as a pill. */
export interface StatusAppearance {
  tone: StatusTone
  glyph: StatusGlyph
}
</script>

<script setup lang="ts">
import {
  ArrowPathIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  MinusCircleIcon,
  QuestionMarkCircleIcon,
  XCircleIcon,
} from '@heroicons/vue/16/solid'
import { type ClassValue, type Component, computed } from 'vue'

import { cn } from '@/methods/utils'

const { tone, glyph } = defineProps<{
  tone: StatusTone
  /** A level or a ranking, such as a severity, takes `dot`: the word carries it, and a state glyph would read as pass or fail. */
  glyph?: StatusGlyph | 'dot'
  class?: ClassValue
}>()

const statusGlyphIcon: Record<StatusGlyph, Component> = {
  success: CheckCircleIcon,
  danger: XCircleIcon,
  warning: ExclamationTriangleIcon,
  progress: ArrowPathIcon,
  neutral: MinusCircleIcon,
  unknown: QuestionMarkCircleIcon,
}

/** The glyph a pill takes when it is given only a tone. */
const glyphFromTone: Record<StatusTone, StatusGlyph> = {
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

const marker = computed(() => glyph ?? glyphFromTone[tone])
</script>

<template>
  <span
    :class="
      cn(
        'inline-flex w-fit items-center rounded-full py-px pr-2 text-xs font-semibold whitespace-nowrap ring-1 ring-inset',
        marker === 'dot' ? 'gap-1.5 pl-1.75' : 'gap-1 pl-1',
        statusPillClass[tone],
        $props.class,
      )
    "
  >
    <span
      v-if="marker === 'dot'"
      class="size-2 shrink-0 rounded-full bg-current"
      aria-hidden="true"
    />
    <component :is="statusGlyphIcon[marker]" v-else class="size-3 shrink-0" aria-hidden="true" />
    <slot />
  </span>
</template>
