<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Component } from 'vue'

import PageHeader from '@/components/common/PageHeader.vue'

type Props = {
  inner?: Component
}

defineProps<Props>()
</script>

<template>
  <div class="flex h-full flex-col gap-4">
    <div class="flex items-start gap-1">
      <PageHeader title="Settings" class="flex-1" :subtitle="`${$route.meta.title}`" />
    </div>
    <RouterView name="inner" class="grow" />
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.content {
  @apply flex w-full border-b border-naturals-n4;
}

.router-link-active {
  @apply relative text-naturals-n13;
}

.router-link-active::before {
  @apply absolute block w-full animate-fadein bg-primary-p3;
  content: '';
  height: 2px;
  bottom: -15px;
}
</style>
