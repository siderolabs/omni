<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { UnmetCondition } from '@/api/talos/runtime.pb'
import TIcon from '@/components/Icon/TIcon.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'

defineProps<{
  conditions: UnmetCondition[]
}>()
</script>

<template>
  <div class="flex flex-wrap gap-micro">
    <Tooltip v-for="condition in conditions" :key="condition.name" :description="condition.reason">
      <div
        class="flex cursor-pointer items-center gap-micro rounded-md bg-surface-hover px-snug py-micro text-xs font-bold text-content-default"
      >
        {{ condition.name }}
        <TIcon icon="exclamation-triangle" class="-mr-micro h-3 w-3 text-status-warning-default" />
      </div>
    </Tooltip>
  </div>
</template>
