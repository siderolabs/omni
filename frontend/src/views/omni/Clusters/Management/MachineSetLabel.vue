<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex items-center text-center select-none" :class="{ 'cursor-pointer': !disabled, 'cursor-not-allowed': disabled }">
    <div class="font-roboto rounded font-bold px-1 py-0.5 resource-label" :class="{[`label-${color}`]: true, 'label-grayed-out': disabled, 'opacity-50': disabled }">{{ machineSetId }}</div>
  </div>
</template>

<script setup lang="ts">
defineProps<{machineSetId: string, color: string, disabled?: boolean}>();
</script>