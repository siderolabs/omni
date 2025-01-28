<!--
Copyright (c) 2025 Sidero Labs, Inc.

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
      <div class="flex gap-2 items-center">
        <span>{{ route.name }}</span>
        <span v-if="route.count" class="min-w-5 px-1.5 py-0.5 font-bold text-xs -my-2 rounded-md bg-naturals-N4">
          {{ route.count }}
        </span>
      </div>
    </tab-button>
  </tabs-header>
  <router-view name="inner"/>
</template>

<script setup lang="ts">
import TabsHeader from "@/components/common/Tabs/TabsHeader.vue";
import TabButton from "@/components/common/Tabs/TabButton.vue";
import { RouteLocationRaw } from "vue-router";
import { computed } from "vue";
import { Resource } from "@/api/grpc";
import { MachineStatusMetricsSpec } from "@/api/omni/specs/omni.pb";
import Watch from "@/api/watch";
import { EphemeralNamespace, MachineStatusMetricsID, MachineStatusMetricsType } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { ref } from "vue";

const machineMetrics = ref<Resource<MachineStatusMetricsSpec>>();
const machineMetricsWatch = new Watch(machineMetrics);

machineMetricsWatch.setup({
  resource: {
    namespace: EphemeralNamespace,
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
  },
  runtime: Runtime.Omni,
});

type r = {name: string, to: RouteLocationRaw, count?: number};

const routes = computed((): r[] => {
  const routes: r[] = [
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
  ];

  if (machineMetrics.value?.spec.pending_machines_count) {
    routes.push(
      {
        name: "Pending",
        to: { name: "MachinesPending" },
        count: machineMetrics.value?.spec.pending_machines_count,
      },
    );
  }

  return routes;
});
</script>
