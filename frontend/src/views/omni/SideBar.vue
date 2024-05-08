<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-sidebar-list :items="items"/>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import TSidebarList from "@/components/SideBar/TSideBarList.vue";
import { canManageBackupStore, canManageUsers, canReadClusters, canReadMachines } from "@/methods/auth";
import { setupBackupStatus } from "@/methods";
import { IconType } from "@/components/common/Icon/TIcon.vue";

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
  const result = [{
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
    result.push({
      name: "Machines",
      route: getRoute("Machines", "/omni/machines"),
      icon: "nodes",
    });
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
