<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
type Props = {
  title: string
  subtitle?: string
  notes?: string
}

defineProps<Props>()
</script>

<template>
  <div class="flex flex-col gap-1" :class="{ 'mb-7': !subtitle, 'mb-2': subtitle }">
    <div class="flex items-center gap-6">
      <h3 class="text-xl font-medium text-naturals-n14">{{ title }}</h3>
      <div class="flex items-center gap-6 max-md:hidden">
        <slot />
      </div>
    </div>
    <div v-if="subtitle" class="text-sm text-naturals-n13">{{ subtitle }}</div>
    <div v-if="notes" class="text-sm text-naturals-n10">{{ notes }}</div>
  </div>
</template>
