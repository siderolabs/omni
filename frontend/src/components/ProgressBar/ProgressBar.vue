<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ProgressIndicator, ProgressRoot } from 'reka-ui'

const { color = 'var(--color-status-success-default)' } = defineProps<{
  color?: string
}>()

const progress = defineModel<number>({ required: true })
</script>

<template>
  <div class="flex items-center gap-tight">
    <ProgressRoot v-model="progress" class="h-2 flex-1 overflow-hidden rounded-sm bg-surface-inert">
      <ProgressIndicator
        class="h-full w-full origin-left transition-colors transition-transform"
        :style="{ transform: `scaleX(${progress}%)`, 'background-color': color }"
      />
    </ProgressRoot>
    <div class="w-8 text-content-default">{{ progress.toFixed(0) }}%</div>
  </div>
</template>
