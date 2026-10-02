<!--
Copyright (c) 2026 Sidero Labs, Inc.

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
  <div v-if="control.label" class="flex items-center justify-between gap-tight px-snug py-snug">
    <div class="flex items-center gap-tight text-xs text-content-secondary">
      {{ control.label }}
      <Tooltip v-if="control.description" :description="control.description">
        <TIcon icon="question-mark-circle" class="h-4 w-4" />
      </Tooltip>
      <Tooltip v-if="control.errors" :description="control.errors">
        <TIcon icon="exclamation-triangle" class="h-4 w-4 text-status-warning-default" />
      </Tooltip>
    </div>
    <slot />
  </div>
  <div v-else class="flex items-center gap-snug px-snug py-compact">
    <div class="flex-1">
      <slot />
    </div>
    <Tooltip v-if="control.errors" :description="control.errors">
      <TIcon icon="exclamation-triangle" class="-my-tight h-4 w-4 text-status-warning-default" />
    </Tooltip>
  </div>
</template>
