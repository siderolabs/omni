<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-4">
    <div class="flex justify-between h-9">
      <PageHeader :title="`${machine}`" />
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
    <router-view name="inner" class="flex-1"/>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { RouteLocationRaw } from "vue-router";
import TabsHeader from "@/components/common/Tabs/TabsHeader.vue";
import TabButton from "@/components/common/Tabs/TabButton.vue";

import { Runtime } from "@/api/common/omni.pb";
import { Resource, ResourceService } from "@/api/grpc";
import { MachineStatusSpec } from "@/api/omni/specs/omni.pb";
import { withRuntime } from "@/api/options";
import { DefaultNamespace, MachineStatusType } from "@/api/resources";
import PageHeader from "@/components/common/PageHeader.vue";
import { onBeforeMount, ref, watch } from "vue";
import { useRoute } from "vue-router";

const routes = computed((): {name: string, to: RouteLocationRaw }[] => {
  return [
    {
      name: "Logs",
      to: { name: "MachineLogs" },
    },
    {
      name: "Patches",
      to: { name: "MachineConfigPatches" },
    },
  ];
});

const route = useRoute();

const getMachineName = async () => {
  const res: Resource<MachineStatusSpec> = await ResourceService.Get({
    namespace: DefaultNamespace,
    type: MachineStatusType,
    id: route.params.machine! as string,
  }, withRuntime(Runtime.Omni));

  machine.value = res.spec.network?.hostname || res.metadata.id!;
};

const machine = ref(route.params.machine);

onBeforeMount(getMachineName);
watch(() => route.params, getMachineName)
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
