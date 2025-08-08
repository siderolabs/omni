<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex gap-1 flex-wrap">
    <tooltip v-for="condition in conditions" :description="condition.reason" :key="condition.name">
      <div
        class="rounded-md py-1 px-3 cursor-pointer bg-naturals-N8 text-naturals-N12 bg-opacity-25 font-bold text-xs flex gap-1 items-center"
      >
        {{ condition.name }}
        <t-icon icon="warning" class="w-3 h-3 -mr-1 text-yellow-Y1" />
      </div>
    </tooltip>
  </div>
</template>

<script setup lang="ts">
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

defineProps<{
  conditions: {
    name: string
    reason: string
  }[]
}>()
</script>
