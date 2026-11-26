<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
/**
 * What a state means, drawn as a shape so it reads without its colour.
 * The set is closed: a new state takes the glyph of the kind it belongs to.
 */
export type StatusGlyphType = 'success' | 'danger' | 'warning' | 'progress' | 'neutral' | 'unknown'

export type StatusTone = 'success' | 'warning' | 'danger' | 'info'
</script>

<script setup lang="ts">
import type { ClassValue } from 'vue'

import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'
import { cn } from '@/methods/utils'

const { glyph, tone } = defineProps<{
  glyph: StatusGlyphType
  /** Colours the glyph as the state's tone; without one it takes the surrounding text colour. */
  tone?: StatusTone
  class?: ClassValue
}>()

const statusGlyphIcon: Record<StatusGlyphType, IconType> = {
  success: 'check-circle-solid',
  danger: 'x-circle-solid',
  warning: 'exclamation-triangle-solid',
  progress: 'arrow-path',
  neutral: 'minus-circle-solid',
  unknown: 'question-mark-circle-solid',
}

/** The glyph is a graphic: it takes the state's `default`, which holds 3:1 on a pill's pastel as well as on a surface. */
const statusGlyphClass: Record<StatusTone, string> = {
  success: 'text-status-success-default',
  warning: 'text-status-warning-default',
  danger: 'text-status-danger-default',
  info: 'text-status-info-default',
}
</script>

<template>
  <TIcon
    :icon="statusGlyphIcon[glyph]"
    :class="cn('size-3', tone && statusGlyphClass[tone], $props.class)"
  />
</template>
