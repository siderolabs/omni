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
    class="t-button"
    :class="{
      [type]: true,
      lightHover: isLightHover,
      toggle,
    }"
  >
    <span
      v-if="$slots.default"
      class="t-button-text whitespace-nowrap"
      :style="textOrder"
      :class="{ 'text-red-R1': danger }"
    >
      <slot />
    </span>
    <TIcon v-if="icon" :icon="icon" class="button-icon" :class="{ 'text-red-R1': danger, type }" />
  </button>
</template>

<style scoped>
.t-button {
  @apply flex items-center justify-center gap-1 rounded border px-4 py-1.5 text-sm transition-colors duration-200;
}

.t-button.subtle-xs {
  @apply border-none bg-transparent p-0 text-xs text-naturals-N13 hover:text-primary-P3 focus:text-primary-P2 focus:underline active:text-primary-P4 active:no-underline disabled:cursor-not-allowed disabled:text-naturals-N6;
}

.t-button._left {
  padding-left: 14px;
}

.t-button._right {
  padding-right: 14px;
}

.t-button._middle {
  padding-right: 9px;
  padding-left: 9px;
}

.t-button._middle:hover .button-icon._middle {
  @apply text-naturals-N14;
}

.t-button.highlighted {
  @apply border-primary-P2 bg-primary-P4 text-naturals-N14 hover:border-primary-P3 hover:bg-primary-P3 hover:text-naturals-N14 focus:border-primary-P2 focus:bg-primary-P2 focus:text-naturals-N14 active:border-primary-P4 active:bg-primary-P4 active:text-naturals-N14 disabled:cursor-not-allowed disabled:border-naturals-N6 disabled:bg-naturals-N4 disabled:text-naturals-N7;
}

.t-button.primary {
  @apply border-naturals-N5 bg-naturals-N3 text-naturals-N12 hover:border-primary-P3 hover:bg-primary-P3 hover:text-naturals-N14 focus:border-primary-P2 focus:bg-primary-P2 focus:text-naturals-N14 active:border-primary-P4 active:bg-primary-P4 active:text-naturals-N14 disabled:cursor-not-allowed disabled:border-naturals-N6 disabled:bg-naturals-N4 disabled:text-naturals-N7;
}

.t-button.primary.toggle {
  @apply border-primary-P4 bg-primary-P4 text-naturals-N14;
}

.t-button.secondary {
  @apply border-naturals-N5 bg-transparent text-naturals-N10 hover:bg-naturals-N5 hover:text-naturals-N14 focus:border-naturals-N7 focus:bg-naturals-N5 focus:text-naturals-N14 active:border-naturals-N5 active:bg-naturals-N4 active:text-naturals-N14 disabled:cursor-not-allowed disabled:border-naturals-N6 disabled:bg-transparent disabled:text-naturals-N6;
}

.t-button.subtle {
  @apply border-none bg-transparent p-0 text-naturals-N13 hover:text-primary-P3 focus:text-primary-P2 focus:underline active:text-primary-P4 active:no-underline disabled:cursor-not-allowed disabled:text-naturals-N6;
}

.t-button.compact {
  @apply h-6 border-naturals-N5 bg-naturals-N4 px-2 py-0.5 text-naturals-N10 hover:bg-naturals-N5 hover:text-naturals-N14 focus:border-naturals-N7 focus:bg-naturals-N5 focus:text-naturals-N14 active:border-primary-P4 active:bg-primary-P4 active:text-naturals-N14 disabled:cursor-not-allowed disabled:border-naturals-N6 disabled:bg-naturals-N4 disabled:text-naturals-N7;
}

.t-button.compact.toggle {
  @apply border-primary-P4 bg-primary-P4 text-naturals-N14;
}

.t-button-group > * {
  @apply rounded-none border-r-0;
  border-radius: 0;
}

.t-button-group > *:first-child {
  @apply rounded-l;
}

.t-button.subtle.lightHover {
  @apply border-none bg-transparent p-0 hover:text-naturals-N12 focus:text-naturals-N13 focus:underline active:text-naturals-N13 active:no-underline disabled:cursor-not-allowed disabled:text-naturals-N6;
}

.button-icon {
  @apply h-4 w-4;
}

.t-button.subtle-xs .button-icon {
  @apply h-3 w-3;
}
</style>
