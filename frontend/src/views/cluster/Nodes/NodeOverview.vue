<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="overview">
    <t-alert :dismiss="{name: 'Dismiss', action: () => alertDismissed = true}" class="mb-4" v-if="clusterMachineStatus?.spec?.last_config_error && !alertDismissed" title="Configuration Error" type="error">
      {{ clusterMachineStatus?.spec?.last_config_error }}
    </t-alert>
    <ul class="overview-data-list">
      <li class="overview-data-item">
        <h4 class="overview-data-heading">Kubernetes</h4>
        <div class="overview-data-row">
          <p class="overview-data-name">Hostname</p>
          <p class="overview-data">{{ nodename?.spec?.nodename }}</p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Kubelet Version</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.kubeletVersion }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">OS</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.operatingSystem }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Roles</p>
          <div class="overview-data-roles">
            <t-tag
              class="overview-data-role"
              v-for="role in roles"
              :key="role"
              :name="role"
            />
          </div>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Status</p>
          <div class="overview-data">
            <t-status :title="status" />
          </div>
        </div>
      </li>
      <li class="overview-data-item">
        <h4 class="overview-data-heading">Node</h4>
        <div class="overview-data-row">
          <p class="overview-data-name">Addresses</p>
          <p class="overview-data">{{ nodeaddress?.spec.addresses.join(", ") }}</p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">OS</p>
          <p class="overview-data">{{ node?.status?.nodeInfo?.osImage }}</p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Architecture</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.architecture }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Kernel Version</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.kernelVersion }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Container Runtime Version</p>
          <p class="overview-data">
            {{ node?.status?.nodeInfo?.containerRuntimeVersion }}
          </p>
        </div>
        <div class="overview-data-row">
          <p class="overview-data-name">Config Status</p>
          <t-status :title="configApplyStatusToConfigApplyStatusName(clusterMachineStatus?.spec?.config_apply_status)" />
        </div>

      </li>
    </ul>
    <watch
      :opts="{
        resource: { type: TalosServiceType, namespace: TalosRuntimeNamespace },
        runtime: Runtime.Talos,
        context: getContext(),
      }"
      errorsAlert
      noRecordsAlert
      spinner
    >
      <template #default="services">
        <div class="overview-services">
          <div class="overview-services-heading">
            <span class="overview-services-title">Services</span>
            <span class="overview-services-amount">{{
              services.items.length
            }}</span>
          </div>
          <ul class="overview-table-header">
            <li class="overview-table-name overview-table-name-id">ID</li>
            <li class="overview-table-name overview-table-name-running">
              Running
            </li>
            <li class="overview-table-name overview-table-name-health">
              Health
            </li>
          </ul>
          <t-group-animation>
            <ul
              v-for="service in services.items"
              :key="service?.metadata?.id"
              class="overview-table-item"
            >
              <li class="overview-item-value overview-item-value-id">
                <router-link :to="{
                        name: 'NodeLogs',
                        query: {
                          cluster: $route.query.cluster,
                          namespace: $route.query.namespace,
                          uid: $route.query.uid,
                        },
                        params: {
                          machine: $route.params.machine,
                          service: service.metadata.id,
                        },
                      }" class="list-item-link" :id="service.metadata.id!">
                  {{ service?.metadata?.id }}
                </router-link>
              </li>
              <li class="overview-item-value overview-item-value-running">
                <t-status
                  class="overview-status"
                  :title="service?.spec?.running ? 'Running' : 'Not Running'"
                />
              </li>
              <li class="overview-item-value overview-item-value-health">
                <t-status
                  class="overview-status"
                  :title="getServiceHealthStatus(service)"
                />
              </li>
            </ul>
          </t-group-animation>
        </div>
      </template>
    </watch>
  </div>
</template>

<script setup lang="ts">
import { computed, Ref, ref } from "vue";
import { getContext } from "@/context";
import { getServiceHealthStatus, getStatus } from "@/methods";
import {
  kubernetes,
  ClusterMachineStatusType,
  DefaultNamespace,
  TalosAddressRoutedNoK8s,
  TalosNodeAddressType,
  TalosNetworkNamespace,
  TalosServiceType,
  TalosRuntimeNamespace,
  TalosK8sNamespace,
  TalosNodenameID,
  TalosNodenameType,
} from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import WatchResource, { WatchOptions } from "@/api/watch";
import { NodeSpec } from "kubernetes-types/core/v1";
import { ResourceTyped } from "@/api/grpc";
import { ClusterMachineStatusSpec, ConfigApplyStatus } from "@/api/omni/specs/omni.pb";
import { useRoute } from "vue-router";

import TAlert from "@/components/TAlert.vue";
import TGroupAnimation from "@/components/common/Animation/TGroupAnimation.vue";
import TTag from "@/components/common/Tag/TTag.vue";
import TStatus from "@/components/common/Status/TStatus.vue";
import Watch from "@/components/common/Watch/Watch.vue";

type Nodename = {
  nodename: string,
};

type NodeAddress = {
  addresses: Array<string>;
};

const configApplyStatusToConfigApplyStatusName = (status?: ConfigApplyStatus): string => {
  switch (status) {
    case ConfigApplyStatus.APPLIED:
      return "Applied";
    case ConfigApplyStatus.FAILED:
      return "Failed";
    case ConfigApplyStatus.PENDING:
      return "Pending";
    default:
      return "Unknown";
  }
}

const alertDismissed = ref(false);
const node: Ref<ResourceTyped<NodeSpec> | undefined> = ref();
const nodename: Ref<ResourceTyped<Nodename> | undefined> = ref();
const nodeaddress: Ref<ResourceTyped<NodeAddress> | undefined> = ref();
const clusterMachineStatus: Ref<ResourceTyped<ClusterMachineStatusSpec> | undefined> = ref();

const nodeWatch = new WatchResource(node);
const nodenameWatch = new WatchResource(nodename);
const nodeaddressWatch = new WatchResource(nodeaddress);
const clusterMachineStatusesWatch = new WatchResource(clusterMachineStatus);

const route = useRoute();
const context = getContext();

nodeWatch.setup(computed(() => {
  const id = nodename.value?.spec?.nodename;
  if (!id) {
    return;
  }

  return {
    runtime: Runtime.Kubernetes,
    resource: {
      id: id,
      type: kubernetes.node,
    },
    context,
  };
}));

nodenameWatch.setup({
  runtime: Runtime.Talos,
  resource: {
    id: TalosNodenameID,
    type: TalosNodenameType,
    namespace: TalosK8sNamespace,
  },
  context,
});

nodeaddressWatch.setup({
  runtime: Runtime.Talos,
  resource: {
    id: TalosAddressRoutedNoK8s,
    type: TalosNodeAddressType,
    namespace: TalosNetworkNamespace,
  },
  context,
});

const clusterMachineStatusWatchOpts: Ref<WatchOptions | undefined> = computed(() => {
  return {
    runtime: Runtime.Omni,
    resource: {
      type: ClusterMachineStatusType,
      namespace: DefaultNamespace,
      id: route.params.machine as string,
    },
  }
});

clusterMachineStatusesWatch.setup(clusterMachineStatusWatchOpts);

const roles = computed(() => {
  const roles: any = [];
  for (const label in node.value?.metadata?.labels) {
    if (label.indexOf("node-role.kubernetes.io/") != -1) {
      roles.push(label.split("/")[1]);
    }
  }
  return roles;
});

const status = computed(() => getStatus(node.value));
</script>

<style scoped>
.overview {
  @apply w-full;
}
.overview-heading {
  @apply flex justify-between items-start mb-7 flex-wrap;
}
.overview-breadcrumbs {
  @apply xl:mb-0 mb-3;
}
.overview-heading-buttons {
  @apply flex justify-end;
}
.overview-heading-button {
  @apply text-naturals-N13;
}
.overview-heading-button:nth-child(1) {
  border-radius: 4px 0 0 4px;
}
.overview-heading-button:nth-child(2) {
  border-radius: 0 4px 4px 0;
}
.overview-content-types {
  @apply mb-6;
}
.overview-data-list {
  @apply flex flex-col justify-start items-stretch xl:flex-row xl:justify-between xl:items-start mb-10;
}
.overview-data-item {
  @apply bg-naturals-N2 w-full rounded mb-6;
  align-self: stretch;
  max-width: 100%;
}
@media screen and (min-width: 1280px) {
  .overview-data-item {
    max-width: 49%;
    @apply mb-0;
  }
}
.overview-data-heading {
  @apply px-4 py-3 border-b border-naturals-N4 w-full text-xs text-naturals-N13;
  font-size: 13px;
}
.overview-data-row {
  @apply w-full flex justify-between items-center;
  padding: 14px 16px 10px 16px;
}
.overview-data-row:last-of-type {
  padding-bottom: 12px;
}
.overview-data-name {
  @apply text-xs text-naturals-N11;
}
.overview-data {
  @apply text-xs text-naturals-N13;
}
.overview-data-roles {
  @apply flex;
}
.overview-data-role:not(:last-of-type) {
  margin-right: 6px;
}
.overview-services-heading {
  @apply mb-4;
}
.overview-services-title {
  @apply text-base text-naturals-N13 mr-2;
}
.overview-services-amount {
  @apply text-xs text-naturals-N12 bg-naturals-N5;
  border-radius: 30px;
  padding: 3px 7px;
}
.overview-table-header {
  @apply flex bg-naturals-N2 rounded-sm;
  padding: 10px 16px;
}
.overview-table-name {
  @apply text-xs text-naturals-N13 w-full;
}
.overview-table-name-id {
  width: 100%;
  max-width: 55%;
}
.overview-table-name-running {
  width: 18%;
}
.overview-table-name-health {
  width: 18%;
}
.overview-table-item {
  @apply flex items-center border-b border-naturals-N4;
  padding: 19px 16px;
}
.overview-item-value {
  @apply text-xs text-naturals-N14 w-full;
}
.overview-item-value-id {
  width: 100%;
  max-width: 55%;
}
.overview-item-value-running {
  width: 18%;
}
.overview-item-value-health {
  width: 18%;
}
@media screen and (max-width: 1280px) {
  .overview-table-name-running {
    width: 50%;
  }
  .overview-table-name-health {
    width: 73%;
  }
  .overview-item-value-running {
    width: 50%;
  }
  .overview-item-value-health {
    width: 50%;
  }
}
.overview-item-value-logs {
  @apply flex justify-end;
  width: auto;
}
.overview-item-button {
  @apply min-w-max;
}
</style>
