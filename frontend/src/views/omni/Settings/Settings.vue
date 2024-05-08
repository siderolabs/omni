<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-4">
    <div class="flex gap-1 items-start">
      <page-header title="Settings" class="flex-1"/>
    </div>
    <tabs-header class="border-b border-naturals-N4 pb-3.5">
      <tab-button is="router-link"
        v-for="route in routes"
        :key="route.name"
        :to="route.to"
        :selected="$route.name === route.to"
      >
        {{ route.name }}
      </tab-button>
    </tabs-header>
    <router-view name="inner"/>
  </div>
</template>

<script setup lang="ts">
import { Component, computed } from "vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { RouteLocationRaw } from "vue-router";
import TabsHeader from "@/components/common/Tabs/TabsHeader.vue";
import TabButton from "@/components/common/Tabs/TabButton.vue";

type Props = {
  inner: Component,
};

const routes = computed((): {name: string, to: RouteLocationRaw }[] => {
  return [
    {
      name: "Users",
      to: { name: "Users" },
    },
    {
      name: "Backups",
      to: { name: "BackupStorage" },
    },
  ];
})

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

