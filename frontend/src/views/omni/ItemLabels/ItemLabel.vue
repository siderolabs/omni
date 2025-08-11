<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'

defineProps<{
  label: {
    key: string
    id: string
    value: string
    color: string
    description?: string
    removable?: boolean
    icon?: string
  }
  removeLabel?: (key: string) => Promise<void>
}>()
</script>

<template>
  <component
    :is="label.description ? Tooltip : 'div'"
    :description="label.description"
    placement="bottom-start"
  >
    <span
      class="flex cursor-pointer items-center transition-all"
      :class="`resource-label label-${label.color}`"
      @click.stop="() => $emit('filterLabel', label)"
    >
      <TIcon v-if="label.icon" :icon="label.icon as IconType" class="mr-1 -ml-1 h-3.5 w-3.5" />
      <template v-if="label.value">
        {{ label.id }}:<span class="font-semibold">{{ label.value }}</span>
      </template>
      <span v-else class="font-semibold">
        {{ label.id }}
      </span>
      <TIcon
        v-if="label.removable && removeLabel"
        icon="close"
        class="destroy-label-button"
        @click.stop="() => removeLabel?.(label.key)"
      />
    </span>
  </component>
</template>

<style scoped>
@reference "../../../index.css";

.destroy-label-button {
  @apply -mr-1 ml-1 inline-block h-3 w-3 cursor-pointer rounded-full transition-all hover:bg-naturals-n14 hover:text-naturals-n1;
}
</style>
