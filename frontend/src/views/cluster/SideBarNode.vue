<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <div class="border-b border-naturals-N4">
      <cluster-side-bar/>
    </div>
    <p class="text-xs text-naturals-N8 mt-5 mb-2 px-6">Node</p>
    <p class="text-xs text-naturals-N13 px-6 truncate">{{ node }}</p>
    <t-sidebar-list :items="items"/>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, Ref } from "vue";
import { useRoute } from "vue-router";
import { getContext } from "@/context";
import { ResourceService, Resource } from "@/api/grpc";
import { Runtime } from "@/api/common/omni.pb";
import { TalosK8sNamespace, TalosNodenameID, TalosNodenameType, TalosRuntimeNamespace, TalosServiceType } from "@/api/resources"
import { withContext, withRuntime } from "@/api/options";

import TSidebarList, { SideBarItem } from "@/components/SideBar/TSideBarList.vue";
import ClusterSideBar from "@/views/cluster/SideBar.vue";

const items: Ref<SideBarItem[]> = ref([]);
const node = ref();
const context = getContext();
const route = useRoute();

onMounted(async () => {
  const response: Resource[] = await ResourceService.List(
    {
      type: TalosServiceType,
      namespace: TalosRuntimeNamespace,
    },
    withRuntime(Runtime.Talos),
    withContext(context),
  );

  const nodename: Resource<{ nodename: string }> = await ResourceService.Get(
    {
      type: TalosNodenameType,
      id: TalosNodenameID,
      namespace: TalosK8sNamespace,
    },
    withRuntime(Runtime.Talos),
    withContext(context),
  );
  node.value = nodename.spec.nodename;

  items.value = [];

  for (const service of response) {
    items.value.push({
      name: service.metadata.id!,
      route: {
        name: "NodeLogs",
        params: {
          machine: route.params.machine as string,
          service: service.metadata.id!
        },
      },
    })
  }
});
</script>
