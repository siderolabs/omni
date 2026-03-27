<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed } from 'vue'

import TButton from '@/components/common/Button/TButton.vue'

const { controlPlanes, workers } = defineProps<{
  action: string
  controlPlanes?: number | string
  workers?: number | string
  onSubmit: () => void
  onReset?: () => void
  warning?: string
  disabled?: boolean
}>()

const controlPlaneCount = computed(() => {
  if (typeof controlPlanes === 'number') {
    return `${controlPlanes} Control ${pluralize('Plane', controlPlanes, false)}`
  }

  return `Control Planes: ${controlPlanes}`
})

const workersCount = computed(() => {
  if (typeof workers === 'number') {
    return `${workers} ${pluralize('Worker', workers, false)}`
  }

  return `Workers: ${workers}`
})
</script>

<template>
  <div class="flex items-center gap-4">
    <div class="flex grow flex-col">
      <p class="text-xs text-naturals-n8">
        <span class="text-naturals-n13">{{ controlPlaneCount }}, {{ workersCount }}</span>
        selected
      </p>

      <div v-if="warning" class="text-xs text-yellow-y1">{{ warning }}</div>
    </div>

    <div class="flex shrink-0 items-center gap-2">
      <TButton v-if="onReset" variant="secondary" @click="onReset">Cancel</TButton>
      <TButton icon-position="left" variant="highlighted" :disabled="disabled" @click="onSubmit">
        {{ action }}
      </TButton>
    </div>
  </div>
</template>
