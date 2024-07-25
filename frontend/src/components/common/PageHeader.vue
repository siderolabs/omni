<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-1" :class="{'mb-7': !subtitle, 'mb-2': subtitle}">
    <div class="flex gap-6 items-center">
      <h3 class="title">{{ title }}</h3>
      <div class="max-md:hidden flex gap-6 items-center">
        <slot/>
      </div>
    </div>
    <div v-if="subtitle" class="text-sm text-naturals-N13">{{ subtitle }}</div>
    <div v-if="notes" class="text-sm text-naturals-N10">{{ notes }}</div>
  </div>
</template>

<script setup lang="ts">
type Props = {
  title: string;
  subtitle?: string;
  notes?: string;
};

defineProps<Props>();
</script>

<style scoped>
.title {
  @apply text-xl font-medium text-naturals-N14;
}
</style>
