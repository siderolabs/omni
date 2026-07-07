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
  filterLabel: [label: Label]
  remove: [key: string]
}>()

const description = computed(() => {
  const fullLabel = [label.id, label.value].filter(Boolean).join(':')

  if (!label.description) return fullLabel

  return `${fullLabel}\n\n${label.description}`
})
</script>

<template>
  <Tooltip :description="description" :delay-duration="500" placement="bottom-start">
    <button
      class="inline-flex items-center gap-1"
      :class="['resource-label', label.labelClass, small ? 'max-w-50' : 'max-w-75']"
      v-bind="$attrs"
      @click.stop="$emit('filterLabel', label)"
    >
      <TIcon v-if="label.icon" :icon="label.icon" class="-ml-1 size-3.5 shrink-0" />
      <span class="truncate">
        {{ label.value ? `${label.id}:${label.value}` : label.id }}
      </span>
      <TIcon
        v-if="label.removable"
        icon="close"
        class="-mr-1 size-3 shrink-0 cursor-pointer rounded-full transition-all hover:bg-naturals-n14 hover:text-naturals-n1"
        @click.stop="$emit('remove', label.key)"
      />
    </button>
  </Tooltip>
</template>
