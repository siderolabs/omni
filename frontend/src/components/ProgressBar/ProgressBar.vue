<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ProgressIndicator, ProgressRoot } from 'reka-ui'

const { color = 'var(--color-green-g1)' } = defineProps<{
  color?: string
}>()

const progress = defineModel<number>({ required: true })
</script>

<template>
  <div class="flex items-center gap-2">
    <ProgressRoot v-model="progress" class="h-2 flex-1 overflow-hidden rounded-sm bg-naturals-n5">
      <ProgressIndicator
        class="transition-color h-full w-full origin-left transition-transform"
        :style="{ transform: `scaleX(${progress}%)`, 'background-color': color }"
      />
    </ProgressRoot>
    <div class="w-8 text-naturals-n12">{{ progress.toFixed(0) }}%</div>
  </div>
</template>
