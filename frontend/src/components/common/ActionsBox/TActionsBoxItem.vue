<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  icon: IconType
  danger?: boolean
}

withDefaults(defineProps<Props>(), {})
</script>

<template>
  <div class="action-item" :class="{ 'action-item-danger': danger }">
    <TIcon class="action-item-icon" :icon="icon" />
    <span class="action-item-text">
      <slot />
    </span>
  </div>
</template>

<style scoped>
.action-item {
  @apply flex w-full cursor-pointer items-center gap-2 px-3 py-2;
}

.action-item:hover > * {
  @apply text-naturals-N12;
}

.action-item-icon {
  @apply h-4 w-4 transition-colors;
}

.action-item-text {
  @apply text-xs transition-colors;
}

.action-item-danger > * {
  @apply text-red-R1;
}

.action-item-danger:hover > * {
  @apply text-primary-P1;
}
</style>
