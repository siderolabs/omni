<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { type ButtonHTMLAttributes, computed } from 'vue'

import TIcon from '@/components/Icon/TIcon.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import type { Label } from '@/methods/labels'

defineOptions({ inheritAttrs: false })

interface Props extends /* @vue-ignore */ ButtonHTMLAttributes {
  label: Label
  small?: boolean
}

const { label } = defineProps<Props>()

defineEmits<{
  selectLabel: []
  removeLabel: []
}>()

const chipClass = computed(() => {
  if (label.tone === 'danger') {
    return 'border-status-danger-subtle-border bg-status-danger-subtle text-status-danger-text hover:border-status-danger-default'
  }

  if (label.system)
    return 'border-transparent bg-surface-hover text-content-default hover:bg-surface-inert'

  return 'border-border-strong text-content-default hover:bg-surface-hover'
})

const keyClass = computed(() => (label.tone === 'danger' ? undefined : 'text-content-secondary'))

const valueClass = computed(() =>
  label.tone === 'danger' ? 'font-medium' : 'font-medium text-content-emphasis',
)

const iconClass = computed(() => (label.tone === 'danger' ? undefined : 'text-content-secondary'))

const description = computed(() => {
  const fullLabel = [label.id, label.value].filter(Boolean).join(':')

  if (!label.description) return fullLabel

  return `${fullLabel}\n\n${label.description}`
})
</script>

<template>
  <Tooltip :description="description" :delay-duration="500" placement="bottom-start">
    <button
      class="inline-flex items-center gap-micro rounded-sm border px-tight py-micro text-xs transition-colors"
      :class="[chipClass, small ? 'max-w-50' : 'max-w-75']"
      v-bind="$attrs"
      @click.stop="$emit('selectLabel')"
    >
      <TIcon v-if="label.icon" :icon="label.icon" class="size-3.5 shrink-0" :class="iconClass" />
      <span class="truncate">
        <template v-if="label.value">
          <span :class="keyClass">{{ label.id }}:</span>
          <span :class="valueClass">{{ label.value }}</span>
        </template>
        <template v-else>{{ label.id }}</template>
      </span>
      <TIcon
        v-if="label.removable"
        icon="close"
        class="-mr-micro size-3 shrink-0 cursor-pointer rounded-full transition-all hover:bg-surface-inert hover:text-content-emphasis"
        @click.stop="$emit('removeLabel')"
      />
    </button>
  </Tooltip>
</template>
