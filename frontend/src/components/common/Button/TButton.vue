<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { StyleValue } from 'vue'
import { computed, toRefs } from 'vue'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  type?: 'primary' | 'secondary' | 'subtle' | 'subtle-xs' | 'compact' | 'highlighted'
  toggle?: boolean
  disabled?: boolean
  icon?: IconType
  danger?: boolean
  iconPosition?: 'left' | 'middle' | 'right'
  isLightHover?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  type: 'primary',
  iconPosition: 'right',
})

const { type, toggle, disabled, icon, danger, iconPosition, isLightHover } = toRefs(props)

const textOrder = computed((): StyleValue => {
  if (iconPosition.value === 'left') {
    return { order: 1 }
  }

  return {}
})
</script>

<template>
  <button
    type="button"
    :disabled="disabled"
    class="flex items-center justify-center gap-1 rounded border px-4 py-1.5 text-sm transition-colors duration-200"
    :class="{
      'border-naturals-n5 bg-naturals-n3 text-naturals-n12 hover:border-primary-p3 hover:bg-primary-p3 hover:text-naturals-n14 focus:border-primary-p2 focus:bg-primary-p2 focus:text-naturals-n14 active:border-primary-p4 active:bg-primary-p4 active:text-naturals-n14 disabled:cursor-not-allowed disabled:border-naturals-n6 disabled:bg-naturals-n4 disabled:text-naturals-n7':
        type === 'primary',
      'border-naturals-n5 bg-transparent text-naturals-n10 hover:bg-naturals-n5 hover:text-naturals-n14 focus:border-naturals-n7 focus:bg-naturals-n5 focus:text-naturals-n14 active:border-naturals-n5 active:bg-naturals-n4 active:text-naturals-n14 disabled:cursor-not-allowed disabled:border-naturals-n6 disabled:bg-transparent disabled:text-naturals-n6':
        type === 'secondary',
      'border-none bg-transparent p-0 text-naturals-n13 hover:text-primary-p3 focus:text-primary-p2 focus:underline active:text-primary-p4 active:no-underline disabled:cursor-not-allowed disabled:text-naturals-n6':
        type === 'subtle',
      'border-none bg-transparent p-0 text-xs text-naturals-n13 hover:text-primary-p3 focus:text-primary-p2 focus:underline active:text-primary-p4 active:no-underline disabled:cursor-not-allowed disabled:text-naturals-n6':
        type === 'subtle-xs',
      'h-6 border-naturals-n5 bg-naturals-n4 px-2 py-0.5 text-naturals-n10 hover:bg-naturals-n5 hover:text-naturals-n14 focus:border-naturals-n7 focus:bg-naturals-n5 focus:text-naturals-n14 active:border-primary-p4 active:bg-primary-p4 active:text-naturals-n14 disabled:cursor-not-allowed disabled:border-naturals-n6 disabled:bg-naturals-n4 disabled:text-naturals-n7':
        type === 'compact',
      'border-primary-p2 bg-primary-p4 text-naturals-n14 hover:border-primary-p3 hover:bg-primary-p3 hover:text-naturals-n14 focus:border-primary-p2 focus:bg-primary-p2 focus:text-naturals-n14 active:border-primary-p4 active:bg-primary-p4 active:text-naturals-n14 disabled:cursor-not-allowed disabled:border-naturals-n6 disabled:bg-naturals-n4 disabled:text-naturals-n7':
        type === 'highlighted',
      'border-primary-p4 bg-primary-p4 text-naturals-n14':
        toggle && ['primary', 'compact'].includes(type),
      'border-none bg-transparent p-0 hover:text-naturals-n12 focus:text-naturals-n13 focus:underline active:text-naturals-n13 active:no-underline disabled:cursor-not-allowed disabled:text-naturals-n6':
        type === 'subtle' && isLightHover,
    }"
  >
    <span
      v-if="$slots.default"
      class="t-button-text whitespace-nowrap"
      :style="textOrder"
      :class="{ 'text-red-r1': danger }"
    >
      <slot />
    </span>
    <TIcon
      v-if="icon"
      :icon="icon"
      class="h-4 w-4"
      :class="{ 'text-red-r1': danger, 'h-3 w-3': type === 'subtle-xs' }"
    />
  </button>
</template>

<style scoped>
@reference "../../../index.css";

.t-button-group > * {
  @apply rounded-none border-r-0;
  border-radius: 0;
}

.t-button-group > *:first-child {
  @apply rounded-l;
}
</style>
