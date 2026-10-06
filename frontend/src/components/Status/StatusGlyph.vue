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
</script>

<script setup lang="ts">
import type { ClassValue } from 'vue'

import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'
import { cn } from '@/methods/utils'

const { glyph } = defineProps<{
  glyph: StatusGlyphType
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
</script>

<template>
  <TIcon :icon="statusGlyphIcon[glyph]" :class="cn('size-3', $props.class)" />
</template>
