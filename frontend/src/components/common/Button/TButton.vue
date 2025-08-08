<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
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
      class="t-button-text whitespace-nowrap"
      v-if="$slots.default"
      :style="textOrder"
      :class="{ 'text-red-R1': danger }"
    >
      <slot />
    </span>
    <t-icon :icon="icon" v-if="icon" class="button-icon" :class="{ 'text-red-R1': danger, type }" />
  </button>
</template>

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
  toggle: false,
  disabled: false,
  danger: false,
  iconPosition: 'right',
  isLightHover: false,
})

const { type, toggle, disabled, icon, danger, iconPosition, isLightHover } = toRefs(props)

const textOrder = computed((): StyleValue => {
  if (iconPosition.value === 'left') {
    return { order: 1 }
  }

  return {}
})
</script>

<style scoped>
.t-button {
  @apply flex
    items-center
    justify-center
    gap-1
    text-sm
    transition-colors
    duration-200
    border
    rounded
    px-4
    py-1.5;
}

.t-button.subtle-xs {
  @apply bg-transparent
    p-0
    text-xs
    border-none
    text-naturals-N13
    hover:text-primary-P3
    focus:text-primary-P2
    focus:underline
    active:text-primary-P4
    active:no-underline
    disabled:text-naturals-N6
    disabled:cursor-not-allowed;
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
  @apply text-naturals-N14
    border-primary-P2
    bg-primary-P4
    hover:bg-primary-P3
    hover:border-primary-P3
    hover:text-naturals-N14
    focus:text-naturals-N14
    focus:border-primary-P2
    focus:bg-primary-P2
    active:bg-primary-P4
    active:border-primary-P4
    active:text-naturals-N14
    disabled:border-naturals-N6
    disabled:bg-naturals-N4
    disabled:text-naturals-N7
    disabled:cursor-not-allowed;
}

.t-button.primary {
  @apply text-naturals-N12
    border-naturals-N5
    bg-naturals-N3
    hover:bg-primary-P3
    hover:border-primary-P3
    hover:text-naturals-N14
    focus:text-naturals-N14
    focus:border-primary-P2
    focus:bg-primary-P2
    active:bg-primary-P4
    active:border-primary-P4
    active:text-naturals-N14
    disabled:border-naturals-N6
    disabled:bg-naturals-N4
    disabled:text-naturals-N7
    disabled:cursor-not-allowed;
}

.t-button.primary.toggle {
  @apply bg-primary-P4
    border-primary-P4
    text-naturals-N14;
}

.t-button.secondary {
  @apply bg-transparent
    text-naturals-N10
    border-naturals-N5
    hover:bg-naturals-N5
    hover:text-naturals-N14
    focus:bg-naturals-N5
    focus:text-naturals-N14
    focus:border-naturals-N7
    active:bg-naturals-N4
    active:text-naturals-N14
    active:border-naturals-N5
    disabled:bg-transparent
    disabled:text-naturals-N6
    disabled:border-naturals-N6
    disabled:cursor-not-allowed;
}

.t-button.subtle {
  @apply bg-transparent
    p-0
    border-none
    text-naturals-N13
    hover:text-primary-P3
    focus:text-primary-P2
    focus:underline
    active:text-primary-P4
    active:no-underline
    disabled:text-naturals-N6
    disabled:cursor-not-allowed;
}

.t-button.compact {
  @apply bg-naturals-N4
    py-0.5
    px-2
    h-6
    text-naturals-N10
    border-naturals-N5
    hover:bg-naturals-N5
    hover:text-naturals-N14
    focus:bg-naturals-N5
    focus:text-naturals-N14
    focus:border-naturals-N7
    active:bg-primary-P4
    active:border-primary-P4
    active:text-naturals-N14
    disabled:border-naturals-N6
    disabled:bg-naturals-N4
    disabled:text-naturals-N7
    disabled:cursor-not-allowed;
}

.t-button.compact.toggle {
  @apply bg-primary-P4
    border-primary-P4
    text-naturals-N14;
}

.t-button-group > * {
  @apply rounded-none border-r-0;
  border-radius: 0;
}

.t-button-group > *:first-child {
  @apply rounded-l;
}

.t-button.subtle.lightHover {
  @apply bg-transparent
    p-0
    border-none
    hover:text-naturals-N12
    focus:text-naturals-N13
    focus:underline
    active:text-naturals-N13
    active:no-underline
    disabled:text-naturals-N6
    disabled:cursor-not-allowed;
}

.button-icon {
  @apply w-4 h-4;
}

.t-button.subtle-xs .button-icon {
  @apply w-3 h-3;
}
</style>
