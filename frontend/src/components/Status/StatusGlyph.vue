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
import {
  ArrowPathIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  MinusCircleIcon,
  QuestionMarkCircleIcon,
  XCircleIcon,
} from '@heroicons/vue/16/solid'
import type { ClassValue, Component } from 'vue'

import { cn } from '@/methods/utils'

const { glyph } = defineProps<{
  glyph: StatusGlyphType
  class?: ClassValue
}>()

const statusGlyphIcon: Record<StatusGlyphType, Component> = {
  success: CheckCircleIcon,
  danger: XCircleIcon,
  warning: ExclamationTriangleIcon,
  progress: ArrowPathIcon,
  neutral: MinusCircleIcon,
  unknown: QuestionMarkCircleIcon,
}
</script>

<template>
  <component :is="statusGlyphIcon[glyph]" :class="cn('size-3', $props.class)" />
</template>
