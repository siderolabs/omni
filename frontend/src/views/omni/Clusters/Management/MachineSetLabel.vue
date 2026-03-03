<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
defineProps<{ machineSetId: string; labelClass?: string; disabled?: boolean }>()
</script>

<template>
  <div
    class="flex items-center text-center select-none"
    :class="{ 'cursor-pointer': !disabled, 'cursor-not-allowed': disabled }"
  >
    <div class="resource-label" :class="disabled ? 'opacity-50' : labelClass">
      {{ machineSetId }}
    </div>
  </div>
</template>
