<!--
Copyright (c) 2025 Sidero Labs, Inc.

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
import { EphemeralNamespace, InfraProviderNamespace, InfraProviderStatusType, MachineStatusMetricsID, MachineStatusMetricsType } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { InfraProviderStatusSpec } from "@/api/omni/specs/infra.pb";

const machineMetrics = ref<Resource<MachineStatusMetricsSpec>>();
const machineMetricsWatch = new Watch(machineMetrics);

const infraProviderStatuses = ref<Resource<InfraProviderStatusSpec>[]>([]);
const infraProvidersWatch = new Watch(infraProviderStatuses);

machineMetricsWatch.setup({
  resource: {
    namespace: EphemeralNamespace,
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
  },
  runtime: Runtime.Omni,
});

infraProvidersWatch.setup(computed(() => {
  if (!canReadMachines.value)
    return

  return {
    resource: {
      namespace: InfraProviderNamespace,
      type: InfraProviderStatusType
    },
    runtime: Runtime.Omni
  }
}));

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

const groupProviders = (statuses: Resource<InfraProviderStatusSpec>[]): Record<string, Resource<InfraProviderStatusSpec>[]> => {
  const res: Record<string, Resource<InfraProviderStatusSpec>[]> = {};

  for (const status of statuses) {
    if (!res[status.spec.name!]) {
      res[status.spec.name!] = [];
    }

    res[status.spec.name!].push(status);
  }

  return res;
};

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
    const autoprovisionedMenuItem: SideBarItem = {
      name: "Auto-Provisioned",
      icon: "machines-autoprovisioned",
      route: getRoute("MachinesManaged", "/omni/machines/managed")
    };

    if (infraProviderStatuses.value.length > 0) {
      autoprovisionedMenuItem.subItems = [];

      const items = groupProviders(infraProviderStatuses.value);

      for (const name in items) {
        const values = items[name];

        const item: SideBarItem = {
          name: name,
          iconSvgBase64: values[0].spec.icon,
        }

        if (!item.iconSvgBase64) {
          item.icon = "cloud-connection";
        }

        if (values.length > 1) {
          item.subItems = [];

          for (const provider of values) {
            item.subItems.push({
              name: provider.metadata.id!,
              route: getRoute("MachinesManagedProvider",  `/omni/machines/managed/${provider.metadata.id!}`)
            })
          }
        } else {
          item.route = getRoute("MachinesManagedProvider",  `/omni/machines/managed/${values[0].metadata.id}`)
        }

        autoprovisionedMenuItem.subItems.push(item);
      }
    }

    const item: SideBarItem = {
      name: "Machines",
      route: getRoute("Machines", "/omni/machines"),
      icon: "nodes",
      subItems: [
        {
          name: "Self-Managed",
          route: getRoute("MachinesManual", "/omni/machines/manual"),
          icon: "machines-manual",
        },
        autoprovisionedMenuItem,
      ],
    };

    if (machineMetrics.value?.spec.pending_machines_count) {
      item.subItems!.push({
          name: "Pending",
          route: getRoute("MachinesPending", "/omni/machines/pending"),
          icon: "question",
          label: machineMetrics.value.spec.pending_machines_count
      });
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
      icon: "settings",
      subItems: [
        {
          name: "Users",
          route: getRoute("Users", "/omni/settings/users"),
          icon: "users",
        },
        {
          name: "Service Accounts",
          route: getRoute("ServiceAccounts", "/omni/settings/serviceaccounts"),
          icon: "users",
        },
        {
          name: "Infra Providers",
          route: getRoute("InfraProviders",  "/omni/settings/infraproviders"),
          icon: "machines-autoprovisioned",
        },
        {
          name: "Backups",
          route: getRoute("Backups", "/omni/settings/backups"),
          icon: "rollback",
        },
      ],
    });
  }

  return result;
}
);
</script>
