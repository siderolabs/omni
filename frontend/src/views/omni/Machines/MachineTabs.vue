<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <tabs-header class="border-b border-naturals-N4 pb-3.5 mb-6">
    <tab-button is="router-link"
      v-for="route in routes"
      :key="route.name"
      :to="route.to"
      :selected="$route.name === route.to.name"
    >
      {{ route.name }}
    </tab-button>
  </tabs-header>
  <router-view name="inner"/>
</template>

<script setup lang="ts">
import TabsHeader from "@/components/common/Tabs/TabsHeader.vue";
import TabButton from "@/components/common/Tabs/TabButton.vue";
import { RouteLocationRaw } from "vue-router";
import { computed } from "vue";

const routes = computed((): {name: string, to: RouteLocationRaw }[] => {
  return [
    {
      name: "All",
      to: { name: "Machines" },
    },
    {
      name: "Manual",
      to: { name: "MachinesManual" },
    },
    {
      name: "Autoprovisioned",
      to: { name: "MachinesProvisioned" },
    },
    {
      name: "PXE Booted",
      to: { name: "MachinesPXE" },
    },
    {
      name: "Pending",
      to: { name: "MachinesPending" },
    },
  ];
});
</script>
