<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { green } from '@/vars/colors'

withDefaults(
  defineProps<{
    progress: number
    color?: string
  }>(),
  {
    color: green.G1,
  },
)
</script>

<template>
  <div class="flex items-center gap-2">
    <div class="h-2 flex-1 overflow-hidden rounded-sm bg-naturals-N5">
      <div
        class="transition-color h-full w-full origin-left transition-transform"
        :style="{ transform: `scaleX(${progress}%)`, 'background-color': color }"
      />
    </div>
    <div class="w-8 text-naturals-N12">{{ progress.toFixed(0) }}%</div>
  </div>
</template>
