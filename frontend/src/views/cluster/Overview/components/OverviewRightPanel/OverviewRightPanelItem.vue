<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
defineProps<{
  name: string
  value?: string | number | boolean
}>()
</script>

<template>
  <div class="overview-details-item">
    <div class="overview-item-name">{{ name }}</div>
    <div class="overview-item-value">
      <span v-if="value" class="overview-item-text">{{ value }}</span>
      <slot />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../../index.css";

.overview-details-item {
  @apply flex w-full justify-between;
  align-items: flex-start !important;
}
.overview-item-name {
  @apply text-xs text-naturals-n11;
  min-width: 40px;
}
.overview-item-value {
  @apply flex items-center gap-1 text-xs text-naturals-n13;
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
