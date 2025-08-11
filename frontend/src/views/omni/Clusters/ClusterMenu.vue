<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed, toRefs } from 'vue'

import TButton from '@/components/common/Button/TButton.vue'

type Props = {
  action: string
  controlPlanes?: number | string
  workers?: number | string
  onSubmit: () => void
  onReset?: () => void
  warning?: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {})

const { controlPlanes, workers } = toRefs(props)

const controlPlaneCount = computed(() => {
  if (typeof controlPlanes?.value === 'number') {
    return `${controlPlanes.value} Control ${pluralize('Plane', controlPlanes.value as number, false)}`
  }

  return `Control Planes: ${controlPlanes?.value}`
})

const workersCount = computed(() => {
  if (typeof workers?.value === 'number') {
    return `${workers.value} ${pluralize('Worker', workers.value as number, false)}`
  }

  return `Workers: ${workers?.value}`
})
</script>

<template>
  <div class="menu flex items-center">
    <div class="flex flex-1 flex-col">
      <div class="menu-amount-box flex-1">
        <span class="menu-amount-box-light">{{ controlPlaneCount }},</span>
        <span class="menu-amount-box-light">{{ workersCount }}</span> selected
      </div>
      <div v-if="warning" class="text-xs text-yellow-y1">{{ warning }}</div>
    </div>
    <TButton v-if="onReset" type="secondary" @click="onReset"> Cancel </TButton>
    <TButton icon-position="left" type="highlighted" :disabled="disabled" @click="onSubmit">
      {{ action }}
    </TButton>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.menu {
  @apply flex gap-4;
}
.menu-amount-box {
  @apply flex items-center text-xs text-naturals-n8;
}
.menu-amount-box-light {
  @apply mr-1 text-naturals-n13;
}
.menu-buttons-box {
  @apply flex items-center;
}
.menu-exit-button {
  @apply h-6 w-6 cursor-pointer fill-current text-naturals-n7 transition-colors hover:text-naturals-n8;
}
</style>
