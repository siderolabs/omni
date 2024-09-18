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
          <p class="overview-data-name">Kubernetes Node Status</p>
          <div class="overview-data">
            <t-status :title="status"/>
          </div>
        </div>
      </li>
      <li class="overview-data-item">
        <h4 class="overview-data-heading">Talos</h4>
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
      <li class="overview-data-item">
        <h4 class="overview-data-heading">Machine Status</h4>
        <div class="overview-data-row">
          <p class="overview-data-name">Stage</p>
          <cluster-machine-phase v-if="clusterMachineStatus" :machine="clusterMachineStatus" class="text-xs"/>
        </div>
        <div class="overview-data-row" v-if="talosMachineStatus">
          <p class="overview-data-name">Unmet Conditions</p>
          <node-conditions v-if="talosMachineStatus.spec.status?.unmetConditions.length > 0" :conditions="talosMachineStatus.spec.status?.unmetConditions"/>
          <div v-else class="text-xs flex gap-1 items-center text-green-G1"><t-icon icon="check-in-circle-classic" class="h-4"/>None</div>
        </div>
        <template v-if="machineStatus">
          <div class="overview-data-row">
            <p class="overview-data-name">Last Active</p>
            <p class="overview-data">
              {{ timeGetter(machineStatus.spec.siderolink_counter?.last_alive) }}
            </p>
          </div>
          <div class="overview-data-row">
            <p class="overview-data-name">Secure Boot</p>
            <p class="overview-data">
              <t-status class="overview-status" :title="getSecureBootStatus()"/>
            </p>
          </div>
          <div class="overview-data-row">
            <p class="overview-data-name">CPU</p>
            <p class="overview-data">
              {{ getCPUInfo() }}
            </p>
          </div>
          <div class="overview-data-row">
            <p class="overview-data-name">Memory</p>
            <p class="overview-data">
              {{ getMemInfo() }}
            </p>
          </div>
        </template>
      </li>
    </ul>

    <template v-if="machineStatus">
      <ul class="overview-data-list" v-if="machineStatus.spec.message_status?.diagnostics">
        <li class="bg-naturals-N2 w-full rounded flex flex-col">
          <h4 class="overview-data-heading">Diagnostic Warnings</h4>
            <node-diagnostic-warnings :diagnostics="machineStatus?.spec?.message_status?.diagnostics"/>
        </li>
      </ul>

      <ul class="overview-data-list">
        <li class="bg-naturals-N2 w-full rounded flex flex-col">
          <h4 class="overview-data-heading">Labels</h4>
          <div class="overview-data-row">
            <item-labels :resource="machineStatus"  :add-label-func="addMachineLabels" :remove-label-func="removeMachineLabels"/>
          </div>
        </li>
      </ul>
    </template>

      <div class="overview-services">
        <div class="overview-services-heading">
          <span class="overview-services-title">Services</span>
          <span class="overview-services-amount">{{
            services.length
          }}</span>
        </div>
        <ul class="overview-table-header grid grid-cols-4">
          <li class="overview-table-name col-span-2">ID</li>
          <li class="overview-table-name">
            Running
          </li>
          <li class="overview-table-name">
            Health
          </li>
        </ul>
        <t-group-animation>
          <t-list-item
            v-for="service in services"
            :key="service.name"
          >
            <template #default>
              <div class="grid grid-cols-4 p-1">
                <router-link class="col-span-2 list-item-link" :to="{
                        name: 'NodeLogs',
                        query: {
                          cluster: $route.query.cluster,
                          namespace: $route.query.namespace,
                          uid: $route.query.uid,
                        },
                        params: {
                          machine: $route.params.machine,
                          service: service.name,
                        },
                      }" :id="service.name">
                  {{ service.name }}
                </router-link>
                <t-status class="overview-status" :title="service.state"/>
                <t-status class="overview-status" :title="service.status"/>
              </div>
            </template>
            <template #details>
              <node-service-events :events="service.events"/>
            </template>
          </t-list-item>
        </t-group-animation>
      </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, Ref, ref } from "vue";
import { getContext } from "@/context";
import { formatBytes, getStatus } from "@/methods";
import {
  kubernetes,
  ClusterMachineStatusType,
  DefaultNamespace,
  TalosAddressRoutedNoK8s,
  TalosNodeAddressType,
  TalosNetworkNamespace,
  TalosK8sNamespace,
  TalosNodenameID,
  TalosNodenameType,
  TalosMachineStatusID,
  TalosMachineStatusType,
  TalosRuntimeNamespace,
  MachineStatusLinkType,
  MetricsNamespace,
} from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import Watch, { WatchOptions } from "@/api/watch";
import { V1NodeSpec } from "@kubernetes/client-node";
import { Resource, subscribe } from "@/api/grpc";
import { ClusterMachineStatusSpec, ConfigApplyStatus } from "@/api/omni/specs/omni.pb";
import { useRoute } from "vue-router";
import { TCommonStatuses } from "@/constants";
import { MachineService, ServiceEvent } from "@/api/talos/machine/machine.pb";
import { withAbortController, withContext, withRuntime } from "@/api/options";
import { MachineStatusLinkSpec } from "@/api/omni/specs/ephemeral.pb";
import { DateTime } from "luxon";
import { addMachineLabels, removeMachineLabels } from "@/methods/machine";

import TAlert from "@/components/TAlert.vue";
import TGroupAnimation from "@/components/common/Animation/TGroupAnimation.vue";
import TTag from "@/components/common/Tag/TTag.vue";
import TStatus from "@/components/common/Status/TStatus.vue";
import TListItem from "@/components/common/List/TListItem.vue";
import NodeServiceEvents from "@/views/cluster/Nodes/NodeServiceEvents.vue";
import ItemLabels from "@/views/omni/ItemLabels/ItemLabels.vue";
import ClusterMachinePhase from "@/views/cluster/ClusterMachines/ClusterMachinePhase.vue";
import NodeConditions from "@/views/cluster/Nodes/components/NodeConditions.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import NodeDiagnosticWarnings from "@/views/cluster/Nodes/NodeDiagnosticWarnings.vue";

const services = ref<{
  name: string
  state: string
  status: TCommonStatuses
  events?: ServiceEvent[]
}[]>([]);

let abortController: AbortController | undefined;

const ctx = getContext();

const fetchServices = async () => {
  if (abortController) {
    abortController.abort();
  }

  abortController = new AbortController();

  const res = await MachineService.ServiceList(
    {},
    withContext(ctx),
    withRuntime(Runtime.Talos),
    withAbortController(abortController),
  );

  abortController = undefined;

  services.value = [];

  for (const message of res.messages!) {
    for (const service of message.services!) {
      services.value.push({
        name: service.id!,
        state: service.state!,
        status: service.health?.unknown ? TCommonStatuses.HEALTH_UNKNOWN : service.health?.healthy ? TCommonStatuses.HEALTHY : TCommonStatuses.UNHEALTHY,
        events: service.events?.events,
      })
    }
  }
}

fetchServices();

const stream = subscribe(MachineService.Events, {}, (event) => {
  if (event.data?.["@type"]?.includes("machine.ServiceStateEvent")) {
    fetchServices();
  }
}, [withRuntime(Runtime.Talos), withContext(getContext())])

onBeforeUnmount(() => {
  stream.shutdown();
})

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
const node: Ref<Resource<V1NodeSpec> | undefined> = ref();
const nodename: Ref<Resource<Nodename> | undefined> = ref();
const nodeaddress: Ref<Resource<NodeAddress> | undefined> = ref();
const clusterMachineStatus: Ref<Resource<ClusterMachineStatusSpec> | undefined> = ref();
const talosMachineStatus = ref<Resource<
{
  stage: string
  status: {
    ready: boolean
    unmetConditions: {
      name: string
      reason: string
    }[]
  }
}>>();
const machineStatus = ref<Resource<MachineStatusLinkSpec>>();

const nodeWatch = new Watch(node);
const nodenameWatch = new Watch(nodename);
const nodeaddressWatch = new Watch(nodeaddress);
const clusterMachineStatusesWatch = new Watch(clusterMachineStatus);
const talosMachineStatusWatch = new Watch(talosMachineStatus);
const machineStatusWatch = new Watch(machineStatus);

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

talosMachineStatusWatch.setup({
  runtime: Runtime.Talos,
  resource: {
    id: TalosMachineStatusID,
    type: TalosMachineStatusType,
    namespace: TalosRuntimeNamespace,
  },
  context,
});

machineStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    id: route.params.machine as string,
    type: MachineStatusLinkType,
    namespace: MetricsNamespace,
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

const status = computed(() => getStatus(node.value!));

const timeGetter = (time: string | undefined) => {
  if (time) {
    const dt: DateTime = /^\d+$/.exec(time) ? DateTime.fromSeconds(parseInt(time)) : DateTime.fromISO(time)

    return dt.setLocale('en').toLocaleString(DateTime.DATETIME_FULL_WITH_SECONDS);
  }

  return "Never";
}

const getCPUInfo = () => {
  const processors = machineStatus.value?.spec.message_status?.hardware?.processors;
  if (!processors) {
    return "Not Detected";
  }

  const cpus = {};

  for (const processor of processors) {
    const id = `${processor.frequency! / 1000} GHz ${processor.manufacturer}, ${processor.core_count} Cores`

    cpus[id] = (cpus[id] ?? 0) + 1
  }

  const parts: string[] = [];

  for (const id in cpus) {
    if (cpus[id] > 1) {
      parts.push(`${cpus[id]} x ${id}`)
    }

    parts.push(`${id}`)
  }

  return parts.join(", ");
}

const getMemInfo = () => {
  const total = machineStatus.value?.spec.message_status?.hardware?.memory_modules?.reduce((prev, current) => {
    return prev + current.size_mb!
  }, 0)

  return formatBytes(total! * 1024 * 1024)
}

const getSecureBootStatus = () => {
  const secureBootStatus = machineStatus?.value?.spec.message_status?.secure_boot_status;
  if (!secureBootStatus) {
    return TCommonStatuses.UNKNOWN;
  }

  if (secureBootStatus.enabled) {
    return TCommonStatuses.ENABLED;
  }

  return TCommonStatuses.DISABLED;
}
</script>

<style scoped>
.overview {
  @apply w-full flex flex-col gap-6;
}

.overview-data-list {
  @apply flex flex-col justify-start items-stretch xl:flex-row xl:justify-between xl:items-start xl:gap-x-6 gap-y-6;
}

.overview-data-item {
  @apply bg-naturals-N2 w-full rounded;
  align-self: stretch;
  max-width: 100%;
}

.overview-data-heading {
  @apply px-4 py-3 border-b border-naturals-N4 w-full text-xs text-naturals-N13;
  font-size: 13px;
}
.overview-data-row {
  @apply w-full flex justify-between items-center gap-2;
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
  @apply bg-naturals-N2 rounded-sm py-2 px-4 pl-11;
}
.overview-table-name {
  @apply text-xs text-naturals-N13 w-full;
}
</style>
