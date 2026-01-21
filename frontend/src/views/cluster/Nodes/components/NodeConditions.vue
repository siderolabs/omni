<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'

defineProps<{
  conditions: {
    name: string
    reason: string
  }[]
}>()
</script>

<template>
  <div class="flex flex-wrap gap-1">
    <Tooltip v-for="condition in conditions" :key="condition.name" :description="condition.reason">
      <div
        class="flex cursor-pointer items-center gap-1 rounded-md bg-naturals-n8/25 px-3 py-1 text-xs font-bold text-naturals-n12"
      >
        {{ condition.name }}
        <TIcon icon="warning" class="-mr-1 h-3 w-3 text-yellow-y1" />
      </div>
    </Tooltip>
  </div>
</template>
