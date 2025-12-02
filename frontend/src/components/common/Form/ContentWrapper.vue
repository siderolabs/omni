<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon from '../Icon/TIcon.vue'
import Tooltip from '../Tooltip/Tooltip.vue'

defineProps<{
  control: {
    label: string
    errors: string
    description?: string
  }
}>()
</script>

<template>
  <div v-if="control.label" class="flex items-center justify-between gap-2 px-3 py-3">
    <div class="flex items-center gap-2 text-xs text-naturals-n11">
      {{ control.label }}
      <Tooltip v-if="control.description" :description="control.description">
        <TIcon icon="question" class="h-4 w-4" />
      </Tooltip>
      <Tooltip v-if="control.errors" :description="control.errors">
        <TIcon icon="warning" class="h-4 w-4 text-yellow-y1" />
      </Tooltip>
    </div>
    <slot />
  </div>
  <div v-else class="flex items-center gap-3 px-3 py-4">
    <div class="flex-1">
      <slot />
    </div>
    <Tooltip v-if="control.errors" :description="control.errors">
      <TIcon icon="warning" class="-my-1.5 h-4 w-4 text-yellow-y1" />
    </Tooltip>
  </div>
</template>
