<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex-1">
    <omni-side-bar class="border-b border-naturals-N4" />
    <p class="text-xs text-naturals-N8 mt-5 mb-2 px-6">Cluster</p>
    <p class="text-xs text-naturals-N13 px-6 truncate">
      {{ $route.params.cluster }}
    </p>
    <t-sidebar-list :items="items" />

    <exposed-service-side-bar v-if="workloadProxyingEnabled && cluster?.spec?.features?.enable_workload_proxy" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, Ref } from "vue";
import { useRoute } from "vue-router";

import {
  default as TSidebarList,
  SideBarItem,
} from "@/components/SideBar/TSideBarList.vue";
import OmniSideBar from "@/views/omni/SideBar.vue";
import { Resource } from "@/api/grpc";
import Watch from "@/api/watch";
import { ClusterSpec, KubernetesUpgradeManifestStatusSpec } from "@/api/omni/specs/omni.pb";
import { KubernetesUpgradeManifestStatusType, DefaultNamespace, ClusterType } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { setupClusterPermissions } from "@/methods/auth";
import ExposedServiceSideBar from "@/views/cluster/ExposedService/ExposedServiceSideBar.vue";
import { setupWorkloadProxyingEnabledFeatureWatch } from "@/methods/features";

const route = useRoute();

const getRoute = (path: string) => `/cluster/${route.params.cluster}${path}`;

const kubernetesUpgradeManifestStatus: Ref<
  Resource<KubernetesUpgradeManifestStatusSpec> | undefined
> = ref();
const kubernetesUpgradeManifestStatusWatch = new Watch(
  kubernetesUpgradeManifestStatus
);

kubernetesUpgradeManifestStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesUpgradeManifestStatusType,
    id: route.params.cluster as string,
  },
});

const pendingManifests = computed(() => {
  const pending = kubernetesUpgradeManifestStatus?.value?.spec.last_fatal_error ? "!" : kubernetesUpgradeManifestStatus?.value?.spec.out_of_sync;

  return pending === undefined || pending === 0 ? undefined : pending;
});

const { canSyncKubernetesManifests, canManageClusterFeatures } = setupClusterPermissions(computed(() => route.params.cluster as string));

const cluster: Ref<Resource<ClusterSpec> | undefined> = ref();
const clusterWatch = new Watch(cluster);
clusterWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterType,
    id: route.params.cluster as string,
  }
});

const items = computed(() => {
  const result: SideBarItem[] = [
    {
      name: "Overview",
      route: getRoute("/overview"),
      icon: "overview",
    },
    {
      name: "Nodes",
      route: getRoute("/nodes"),
      icon: "nodes",
    },
    {
      name: "Pods",
      route: getRoute("/pods"),
      icon: "pods",
    },
    {
      name: "Config Patches",
      route: getRoute("/patches"),
      icon: "settings",
    },
  ];

  if (canSyncKubernetesManifests.value) {
    result.push({
      name: "Bootstrap Manifests",
      route: getRoute("/manifests"),
      icon: "bootstrap-manifests",
      label: pendingManifests.value,
      labelColor: pendingManifests.value === "!" ? "red-R1" : undefined,
    });
  }

  if (canManageClusterFeatures.value) {
    result.push({
      name: "Backups",
      route: getRoute("/backups"),
      icon: "rollback",
    });
  }

  return result;
});

const workloadProxyingEnabled = setupWorkloadProxyingEnabledFeatureWatch();
</script>
