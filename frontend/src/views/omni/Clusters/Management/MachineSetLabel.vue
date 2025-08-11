<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
defineProps<{ machineSetId: string; color: string; disabled?: boolean }>()
</script>

<template>
  <div
    class="flex select-none items-center text-center"
    :class="{ 'cursor-pointer': !disabled, 'cursor-not-allowed': disabled }"
  >
    <div
      class="resource-label rounded px-1 py-0.5 font-roboto font-bold"
      :class="{ [`label-${color}`]: true, 'label-grayed-out': disabled, 'opacity-50': disabled }"
    >
      {{ machineSetId }}
    </div>
  </div>
</template>
