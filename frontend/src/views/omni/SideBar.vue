<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-sidebar-list :items="items"/>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute } from "vue-router";

import TSidebarList, { SideBarItem } from "@/components/SideBar/TSideBarList.vue";
import { canManageBackupStore, canManageUsers, canReadClusters, canReadMachines } from "@/methods/auth";
import { setupBackupStatus } from "@/methods";
import { IconType } from "@/components/common/Icon/TIcon.vue";
import { Resource } from "@/api/grpc";
import { MachineStatusMetricsSpec } from "@/api/omni/specs/omni.pb";
import Watch from "@/api/watch";
import { EphemeralNamespace, MachineStatusMetricsID, MachineStatusMetricsType } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import pluralize from "pluralize";

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

const route = useRoute();

const getRoute = (name: string, path: string) => {
  return route.query.cluster ? {
        name: name,
        query: {
          cluster: route.query.cluster,
          namespace: route.query.namespace,
          uid: route.query.uid,
        },
      } : path;
};

const { status: backupStatus } = setupBackupStatus();

const items = computed(() => {
  const result: SideBarItem[] = [{
    name: "Home",
    route: getRoute("Overview", "/omni/"),
    icon: "home" as IconType,
  }];

  if (canReadClusters.value) {
    result.push({
      name: "Clusters",
      route: getRoute("Clusters", "/omni/clusters"),
      icon: "clusters",
    });
  }

  if (canReadMachines.value) {
    const item: SideBarItem = {
      name: "Machines",
      route: getRoute("Machines", "/omni/machines"),
      icon: "nodes",
    };

    if (machineMetrics.value?.spec.pending_machines_count) {
      item.label = machineMetrics.value.spec.pending_machines_count;
      item.tooltip = `${machineMetrics.value.spec.pending_machines_count} ${pluralize('machines', machineMetrics.value.spec.pending_machines_count)} not accepted`;
    }

    result.push(item);
  }

  if (canReadMachines.value) {
    result.push({
      name: "Machine Classes",
      route: getRoute("MachineClasses", "/omni/machine-classes"),
      icon: "code-bracket",
    });
  }

  if (canManageUsers.value || (backupStatus.value.configurable && canManageBackupStore.value)) {
    result.push({
      name: "Settings",
      route: getRoute("Settings", "/omni/settings"),
      icon: "settings",
    });
  }

  return result;
}
);
</script>
