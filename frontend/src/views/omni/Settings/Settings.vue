<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-4">
    <div class="flex gap-1 items-start">
      <page-header title="Settings" class="flex-1" :subtitle="$route.meta.title"/>
    </div>
    <router-view name="inner"/>
  </div>
</template>

<script setup lang="ts">
import { Component } from "vue";
import PageHeader from "@/components/common/PageHeader.vue";

type Props = {
  inner?: Component,
};

defineProps<Props>();
</script>

<style scoped>
.content {
  @apply w-full border-b border-naturals-N4 flex;
}

.router-link-active {
  @apply text-naturals-N13 relative;
}

.router-link-active::before {
  @apply block absolute bg-primary-P3 w-full animate-fadein;
  content: "";
  height: 2px;
  bottom: -15px;
}
</style>

