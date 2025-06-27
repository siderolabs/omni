<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div v-if="hasRoleNone" class="flex flex-col items-center justify-center gap-2">
    <div class="w-16 h-16 relative mb-6">
      <div class="absolute w-full h-full top-0 left-0 rounded-lg bg-naturals-N2 rotate-12 translate-x-1.5 -translate-y-1.5"/>
      <div class="flex items-center justify-center absolute w-full h-full top-0 left-0 rounded-lg bg-naturals-N3">
        <t-icon icon="warning" class="w-6 h-6 text-naturals-N11" />
      </div>
    </div>
    <p class="text-lg text-naturals-N14">You don't have access to Omni Web</p>
    <p class="text-xs text-naturals-N10">At least Reader role is required</p>
    <div class="flex gap-3 mt-3">
      <t-button type="primary" icon="talos-config" iconPosition="left" @click="() => downloadOmniconfig()">
        Download <code>omniconfig</code></t-button>
      <t-button type="primary" icon="talos-config" iconPosition="left" @click="() => downloadOmnictl()">
        Download omnictl</t-button>
    </div>
  </div>
  <div v-else class="flex flex-col">
    <page-header title="Home"/>
    <watch :opts="{
        resource: {
          type: ConnectionParamsType,
          namespace: DefaultNamespace,
          id: ConfigID
        },
        runtime: Runtime.Omni
      }"
      errorsAlert
      spinner>
      <template #default="{ items }">
        <div class="flex gap-6">
          <div class="space-y-6 w-full">
            <div class="overview-card p-6 flex flex-1 flex-wrap gap-3">
              <div class="flex flex-col gap-3 flex-1" style="min-width: 200px">
                <div class="text-naturals-N14 text-sm">General Information</div>
                <div class="overview-general-info-grid">
                  <div>Backend Version</div>
                  <watch :opts="{ resource: sysVersionResource, runtime: Runtime.Omni }" class="text-right">
                    <template #default="{ items }">
                      {{ items[0]?.spec?.backend_version }}
                    </template>
                  </watch>
                  <div>API Endpoint</div>
                  <div>
                    {{ items[0]?.spec?.api_endpoint }}
                    <t-icon icon="copy" class="overview-copy-icon"
                      @click="() => copyValue(items[0]?.spec?.api_endpoint)" />
                  </div>
                  <div>WireGuard Endpoint</div>
                  <div>
                    {{ items[0]?.spec?.wireguard_endpoint }}
                    <t-icon icon="copy" class="overview-copy-icon"
                      @click="() => copyValue(items[0]?.spec?.wireguard_endpoint)" />
                  </div>
                  <div>Join Token</div>
                  <div>
                    <div class="flex-1 truncate text-right cursor-pointer select-none token" @click="() => showJoinToken = !showJoinToken">{{ showJoinToken ? items[0]?.spec?.join_token : items[0]?.spec?.join_token.replace(/./g, "•") }}</div>
                    <t-icon icon="copy" class="overview-copy-icon"
                      @click="() => copyValue(items[0]?.spec?.join_token)" />
                  </div>
                </div>
              </div>
              <div class="flex px-12" v-if="canReadClusters">
                <watch :opts="{ resource: machineStatusMetricsResource, runtime: Runtime.Omni }">
                  <template #default="{ items }">
                    <overview-circle-chart-item class="text-naturals-N14 text-sm"
                      :chartFillPercents="computePercentOfMachinesAssignedToClusters(items)" name="Machines"
                      :usageName="(items[0]?.spec.allocated_machines_count ?? 0) + ' Used'"
                      :usagePercents="computePercentOfMachinesAssignedToClusters(items)" :usageTotal="items[0]?.spec.registered_machines_count ?? 0" />
                  </template>
                </watch>
              </div>
            </div>
            <div v-if="canReadClusters" class="overview-card flex-1">
              <div class="flex flex-auto gap-6 p-6">
                <div class="text-naturals-N14 text-sm">Recent Clusters</div>
                <div class="grow" />
                <t-button @click="openClusterCreate" :disabled="!canCreateClusters" iconPosition="left" icon="plus" type="subtle">Create Cluster
                </t-button>
                <t-button @click="openClusters" iconPosition="left" icon="clusters" type="subtle">View All</t-button>
              </div>
              <watch :opts="{ resource: resource, runtime: Runtime.Omni, sortByField: 'created', sortDescending: true }">
                <template #norecords>
                  <div class="px-6 pb-6">No clusters</div>
                </template>
                <template #default="{ items }">
                  <div class="recent-clusters-row" v-for="item in items.slice(0, 5)" :key="itemID(item)">
                    <div class="flex-1 grid grid-cols-3">
                      <router-link :to="{ name: 'ClusterOverview', params: { cluster: item.metadata.id }}" class="list-item-link">
                        {{ item.metadata.id }}
                      </router-link>
                      <div>
                        {{ item.spec.machines.total }}
                        {{ pluralize("Node", item.spec.machines.total) }}
                      </div>
                      <cluster-status :cluster="item" />
                    </div>
                  </div>
                </template>
              </watch>
            </div>
          </div>
          <div class="flex-col space-y-6">
            <div class="overview-card p-6 flex flex-col gap-5 place-items-stretch" style="width: 350px">
              <div class="text-naturals-N14 text-sm">Add Machines</div>
              <t-button icon="long-arrow-down" iconPosition="left" @click="openDownloadIso">Download Installation Media</t-button>
              <t-button icon="long-arrow-down" iconPosition="left" @click="downloadMachineJoinConfig">Download Machine Join Config</t-button>
              <t-button icon="copy" iconPosition="left" @click="copyKernelArgs">Copy Kernel Parameters</t-button>
            </div>
            <div class="overview-card p-6 flex flex-col gap-5 place-items-stretch" style="width: 350px">
              <div class="text-naturals-N14 text-sm">CLI</div>
              <t-button type="primary" icon="document" iconPosition="left" @click="() => downloadTalosconfig()">
                Download <code>talosconfig</code></t-button>
              <t-button type="primary" icon="talos-config" iconPosition="left" @click="() => downloadTalosctl()">
                Download talosctl</t-button>
              <t-button type="primary" icon="document" iconPosition="left" @click="() => downloadOmniconfig()">
                Download <code>omniconfig</code></t-button>
              <t-button type="primary" icon="talos-config" iconPosition="left" @click="() => downloadOmnictl()">
                Download omnictl</t-button>
            </div>
            <div class="overview-card p-6 flex flex-col gap-5 place-items-stretch" style="width: 350px" v-if="canReadAuditLog && auditLogAvailable">
              <div class="text-naturals-N14 text-sm">Tools</div>
              <t-button type="primary" icon="document" iconPosition="left" @click="() => downloadAuditLog()">
                Get audit logs</t-button>
            </div>
          </div>
        </div>
      </template>
    </watch>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import pluralize from "pluralize";
import {
  ClusterStatusType,
  ConfigID,
  ConnectionParamsType,
  DefaultNamespace,
  EphemeralNamespace,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
  RoleNone,
  SysVersionID,
  SysVersionType
} from "@/api/resources";
import { computed, onBeforeMount, ref } from "vue";
import { copyText } from "vue3-clipboard";
import { Runtime } from "@/api/common/omni.pb";

import { Resource } from "@/api/grpc";
import { itemID } from "@/api/watch";
import { MachineStatusMetricsSpec } from "@/api/omni/specs/omni.pb";
import { downloadOmniconfig, downloadTalosconfig, downloadAuditLog } from "@/methods";

import OverviewCircleChartItem from "@/views/cluster/Overview/components/OverviewCircleChart/OverviewCircleChartItem.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import ClusterStatus from "@/views/omni/Clusters/ClusterStatus.vue";
import Watch from "@/components/common/Watch/Watch.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { canCreateClusters, canReadAuditLog, canReadClusters, currentUser } from "@/methods/auth";
import { auditLogEnabled } from "@/methods/features";
import { ManagementService } from "@/api/omni/management/management.pb";

const hasRoleNone = computed(() => {
  const role = currentUser.value?.spec?.role

  return !role || role === RoleNone;
});

const router = useRouter();

const showJoinToken = ref(false);
const resource = {
  namespace: DefaultNamespace,
  type: ClusterStatusType,
};

const machineStatusMetricsResource = {
  namespace: EphemeralNamespace,
  type: MachineStatusMetricsType,
  id: MachineStatusMetricsID,
};

const sysVersionResource = {
  type: SysVersionType,
  namespace: EphemeralNamespace,
  id: SysVersionID,
};

const openClusterCreate = () => {
  router.push({ name: "ClusterCreate" });
};

const openClusters = () => {
  router.push({ name: "Clusters" });
};

const computePercentOfMachinesAssignedToClusters = (items: Resource<MachineStatusMetricsSpec>[]): string => {
  return (
    (100 / Math.max(items[0]?.spec.registered_machines_count ?? 0, 1)) *
    (items[0]?.spec.allocated_machines_count ?? 0)
  ).toFixed(0);
};

const copyValue = (value) => {
  return copyText(value, undefined, () => { });
};

const openDownloadIso = () => {
  router.push({
    query: { modal: "downloadInstallationMedia" },
  });
};

const downloadOmnictl = () => {
  router.push({
    query: { modal: "downloadOmnictlBinaries" },
  });
};

const downloadTalosctl = () => {
  router.push({
    query: { modal: "downloadTalosctlBinaries" },
  });
};

const copyKernelArgs = async () => {
  const response = await ManagementService.GetMachineJoinConfig({});

  copyValue(response.kernel_args!.join(" "));
}

const downloadMachineJoinConfig = async () => {
  const response = await ManagementService.GetMachineJoinConfig({});

  const element = document.createElement("a");
  element.setAttribute("href", "data:text/plain;charset=utf-8," + encodeURIComponent(response.config!));
  element.setAttribute("download", "machine-config.yaml");

  element.style.display = "none";
  document.body.appendChild(element);

  element.click();

  document.body.removeChild(element);
};

const auditLogAvailable = ref(false);

onBeforeMount(async () => {
  auditLogAvailable.value = await auditLogEnabled();
})
</script>

<style scoped>
.overview-card {
  font-size: 12px;
  @apply rounded bg-naturals-N2;
}

.overview-copy-icon {
  @apply cursor-pointer w-5 text-primary-P2 p-0.5 rounded w-5 h-5;
}

.overview-copy-icon:hover {
  @apply bg-naturals-N5 text-primary-P1;
}

.overview-general-info-grid {
  @apply grid grid-cols-3 text-xs gap-4;
}

.overview-general-info-grid>*:nth-child(even) {
  @apply flex items-center col-span-2 gap-1 justify-end;
}

.recent-clusters-row {
  @apply w-full py-4 flex place-content-between px-6;
}

.recent-clusters-row:not(:first-of-type) {
  @apply border-t border-naturals-N4;
}

.token {
  font-family: "Roboto Mono", "consolas", monospace, sans-serif;
}
</style>
