<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="overview-details-item">
    <div class="overview-item-name">{{ name }}</div>
    <div class="overview-item-value">
      <span class="overview-item-text" v-if="value">{{ value }}</span> <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  name: string
  value?: string | number | boolean
}>();
</script>

<style scoped>
.overview-details-item {
  @apply w-full flex justify-between;
  align-items: flex-start !important;
}
.overview-item-name {
  @apply text-xs text-naturals-N11;
  min-width: 40px;
}
.overview-item-value {
  @apply text-xs text-naturals-N13 flex items-center gap-1;
}
.overview-item-text {
  margin-right: 2px;
  text-align: right;
  word-break: break-all;
}
.overview-item-text svg {
  margin-right: 2px;
  text-align: right;
  word-break: break-all;
}
</style>
