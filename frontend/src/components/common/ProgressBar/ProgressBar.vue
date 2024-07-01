<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex gap-2 items-center">
    <div class="flex-1 rounded-sm bg-naturals-N5 h-2 overflow-hidden">
      <div class="w-full h-full transition-transform transition-color origin-left" :style="{ transform: `scaleX(${progress}%)`, 'background-color': color }"/>
    </div>
    <div class="text-naturals-N12 w-8">{{ progress.toFixed(0) }}%</div>
  </div>
</template>

<script setup lang="ts">
import { green } from '@/vars/colors';

withDefaults(defineProps<{
  progress: number
  color?: string
}>(),
  {
    color: green.G1,
  }
)
</script>
