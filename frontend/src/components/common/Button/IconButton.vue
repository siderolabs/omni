<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { toRefs } from 'vue'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  icon: IconType
  disabled?: boolean
  danger?: boolean
  iconClasses?: Record<string, boolean>
}

const props = withDefaults(defineProps<Props>(), {})
const { icon, disabled } = toRefs(props)
</script>

<template>
  <button :disabled="disabled" class="icon-button" :class="{ danger }">
    <slot v-if="$slots.default" />
    <TIcon v-else class="icon-button-icon" :class="iconClasses" :icon="icon" />
  </button>
</template>

<style scoped>
button {
  @apply h-6 w-6;
}
.icon-button-icon {
  @apply h-full w-full cursor-pointer p-1;
}

.icon-button {
  @apply rounded text-naturals-N11 transition-all duration-100 hover:bg-naturals-N7 hover:text-naturals-N13 disabled:pointer-events-none disabled:cursor-not-allowed disabled:text-naturals-N7;
}

.icon-button.danger {
  @apply text-red-R1;
}

.icon-button.highlighted {
  @apply bg-naturals-N7;
}
</style>
