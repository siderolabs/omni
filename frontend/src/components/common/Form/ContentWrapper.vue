<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import TIcon from '../Icon/TIcon.vue'
import Tooltip from '../Tooltip/Tooltip.vue'

const props = defineProps<{
  control: {
    label: string
    errors: string
    description?: string
  }
}>()

const description = computed(() => {
  return props.control.description ? ` (${props.control.description})` : ''
})
</script>

<template>
  <div v-if="control.label" class="flex items-center justify-between gap-2 px-3 py-3">
    <div class="flex items-center gap-2 text-xs text-naturals-N11">
      {{ control.label }}{{ description }}
      <Tooltip v-if="control.errors" :description="control.errors">
        <TIcon icon="warning" class="h-4 w-4 text-yellow-Y1" />
      </Tooltip>
    </div>
    <slot />
  </div>
  <div v-else class="flex items-center gap-3 px-3 py-4">
    <div class="flex-1">
      <slot />
    </div>
    <Tooltip v-if="control.errors" :description="control.errors">
      <TIcon icon="warning" class="-my-1.5 h-4 w-4 text-yellow-Y1" />
    </Tooltip>
  </div>
</template>
