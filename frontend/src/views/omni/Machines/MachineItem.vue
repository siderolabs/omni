<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-list-item>
    <template #default>
      <div class="flex text-naturals-N13 text-xs items-center" :class="{ 'opacity-50': machine.spec.tearing_down }">
        <div class="flex-1 flex gap-2 items-center">
          <router-link :to="{ name: 'MachineLogs', params: { machine: machine?.metadata?.id } }" class="list-item-link pr-2">
            <word-highlighter
              :query="(searchQuery ?? '')"
              split-by-space
              :textToHighlight="machineName"
              highlightClass="bg-naturals-N14"/>
          </router-link>
          <item-labels :resource="machine"
            :add-label-func="machine.spec.tearing_down ? undefined : addMachineLabels"
            :remove-label-func="removeMachineLabels" @filter-label="(e) => $emit('filterLabels', e)"/>
        </div>
        <div v-if="machine.spec.tearing_down">
          <tooltip description="The machine is being destroyed">
            <t-icon icon="delete" class="w-4 h-4 text-red-R1"/>
          </tooltip>
        </div>
        <div class="flex gap-1" v-else>
          <tooltip :description="maintenanceUpdateDescription" v-if="canAccessMaintenanceNodes">
            <icon-button icon="upgrade" @click="openMaintenanceUpdate" :disabled="!canDoMaintenanceUpdate"/>
          </tooltip>
          <div class="flex justify-end">
            <t-actions-box style="height: 24px">
              <t-actions-box-item icon="settings" @click="openConfigPatches">Config Patches</t-actions-box-item>
              <t-actions-box-item icon="log" @click="machineLogs" v-if="canReadMachineLogs">Logs</t-actions-box-item>
              <t-actions-box-item icon="copy" @click="copyMachineID">Copy Machine ID</t-actions-box-item>
              <t-actions-box-item icon="overview" @click="showCluster" v-if="clusterName && canReadClusters">Show Cluster</t-actions-box-item>
              <t-actions-box-item icon="delete" danger @click="removeMachine" v-if="canRemoveMachines">Remove Machine</t-actions-box-item>
            </t-actions-box>
          </div>
        </div>
      </div>
    </template>
    <template #details>
      <div class="pl-6 grid grid-cols-5 gap-1">
        <div class="mb-2 mt-4">Processors</div>
        <div class="mb-2 mt-4">Memory</div>
        <div class="mb-2 mt-4">Block Devices</div>
        <div class="mb-2 mt-4">Addresses</div>
        <div class="mb-2 mt-4">Network Interfaces</div>
        <div>
          <template v-if="processors.length > 0">
            <div v-for="(processor, index) in processors" :key="index">
              {{ processor }}
            </div>
          </template>
          <template v-else>
            <div>No processors detected</div>
          </template>
        </div>
        <div>
          <template v-if="memorymodules.length > 0">
            <div v-for="(memorymodule, index) in memorymodules" :key="index">
              {{ memorymodule }}
            </div>
          </template>
          <template v-else>
            <div>No memory modules detected</div>
          </template>
        </div>
        <div>
          <div v-for="(dev, index) in machine?.spec?.message_status?.hardware?.blockdevices" :key="index">
            {{ dev.linux_name }} {{ formatBytes(dev.size) }} {{ dev.type }}
          </div>
        </div>
        <div>
          <div>
            {{ machine.spec?.message_status?.network?.addresses?.join(", ") }}
          </div>
        </div>
        <div>
          <div v-for="(link, index) in machine?.spec?.message_status?.network?.network_links" :key="index">
            {{ link.linux_name }} {{ link.hardware_address }} {{ link.link_up ? 'UP' : 'DOWN' }}
          </div>
        </div>
        <div class="mb-2 mt-4">Talos Version</div>
        <div class="mb-2 mt-4">UUID</div>
        <div class="mb-2 mt-4">Bytes Received</div>
        <div class="mb-2 mt-4">Bytes Sent</div>
        <div class="mb-2 mt-4">Cluster</div>
        <div>
          <div>{{ machine?.spec?.message_status?.talos_version }}</div>
        </div>
        <word-highlighter
          :query="(searchQuery ?? '')"
          split-by-space
          :textToHighlight="machine?.metadata?.id"
          highlightClass="bg-naturals-N14"/>
        <div>
          <div>{{ machine.spec.siderolink_counter?.bytes_received ? formatBytes(machine.spec.siderolink_counter.bytes_received) : '0B' }}</div>
        </div>
        <div>
          <div>{{ machine.spec.siderolink_counter?.bytes_sent ? formatBytes(machine.spec.siderolink_counter.bytes_sent) : '0B' }}</div>
        </div>
        <div>
          <router-link :to="{ name: 'ClusterOverview', params: { cluster: clusterName }}" v-if="clusterName"  class="list-item-link">
            {{ clusterName }}
          </router-link>
        </div>
        <div class="mb-2 mt-4">Last Active</div>
        <div class="mb-2 mt-4">Created At</div>
        <div class="mb-2 mt-4">Secure Boot</div>
        <div class="mb-2 mt-4 col-span-2">Power State</div>
        <div>
          <div>{{ machineLastAlive }}</div>
        </div>
        <div>
          <div>{{ machineCreatedAt }}</div>
        </div>
        <div>
          <div>{{ secureBoot }}</div>
        </div>
        <div class="flex gap-0.5 items-center" :class="{
          'text-green-G1': machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_ON,
          'text-naturals-N9': machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_OFF,
          'text-naturals-N8': machine.spec.message_status?.power_state === undefined ||
           machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_UNKNOWN ||
           machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_UNSUPPORTED
        }">
          <template v-if=" machine.spec.message_status?.power_state === undefined ||
          machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_UNSUPPORTED ||
          machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_UNKNOWN">
            <t-icon icon="question" class="w-4 h-4 mr-1"/>
            <div>{{ machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_UNSUPPORTED ? "Unsupported" : "Unknown" }}</div>
          </template>
          <template v-else>
            <t-icon icon="dot" class="w-5 h-5 -ml-1"/>
            <div>{{ machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_ON ? "On" : "Off" }}</div>
          </template>
        </div>
      </div>
    </template>
  </t-list-item>
</template>

<script setup lang="ts">
import { computed, toRefs } from "vue";
import { formatBytes } from "@/methods";
import pluralize from 'pluralize';
import { copyText } from "vue3-clipboard";

import { useRouter } from "vue-router";
import { Resource } from "@/api/grpc";
import { MachineStatusLinkSpec } from "@/api/omni/specs/ephemeral.pb";
import { DateTime } from "luxon";

import TListItem from "@/components/common/List/TListItem.vue";
import TActionsBox from "@/components/common/ActionsBox/TActionsBox.vue";
import TActionsBoxItem from "@/components/common/ActionsBox/TActionsBoxItem.vue";
import ItemLabels from "@/views/omni/ItemLabels/ItemLabels.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import WordHighlighter from "vue-word-highlighter";
import { addMachineLabels, removeMachineLabels } from "@/methods/machine";
import { canReadClusters, canReadMachineLogs, canRemoveMachines, canAccessMaintenanceNodes } from "@/methods/auth";
import { MachineStatusSpecPowerState } from "@/api/omni/specs/omni.pb";
import { MachineStatusLabelInstalled } from "@/api/resources";

type MachineWithLinkCounter = Resource<MachineStatusLinkSpec>;
const props = defineProps<{
  machine: MachineWithLinkCounter
  searchQuery?: string
}>();

defineEmits(["filterLabels"]);

const { machine } = toRefs(props);
const router = useRouter();

const machineName = computed(() => {
  return machine?.value?.spec?.message_status?.network?.hostname ?? machine?.value?.metadata?.id
});

const openConfigPatches = () => {
  router.push({ name: "MachineConfigPatches", params: { machine: machine.value.metadata.id } });
};

const openMaintenanceUpdate = () => {
  router.push({
    query: { modal: "maintenanceUpdate", machine: machine.value.metadata.id, cluster: clusterName.value },
  });
};

const processors = computed(() => {
  const processors = machine?.value?.spec?.message_status?.hardware?.processors || [];

  const format = (proc: { frequency?: number; core_count?: number; description?: string }) => {
    return `${proc.frequency! / 1000} GHz, ${pluralize("core", proc.core_count, true)}, ${proc.description}`;
  };

  return processors.map(format);
});

const memorymodules = computed(() => {
  const memorymodules = machine?.value?.spec?.message_status?.hardware?.memory_modules || [];

  const format = (mem: { description?: string; size_mb?: number; }) => {
    let description = mem.description;

    if (mem.description === undefined) {
      description = "";
    }
    return `${formatBytes(mem.size_mb! * 1024 * 1024)} ${description}`;
  };

  return memorymodules.filter((mem: { size_mb?: number; }) => mem.size_mb !== 0).map(format);
});

const clusterName = computed(() => machine.value?.spec?.message_status?.cluster);

const removeMachine = () => {
  router.push({
    query: { modal: "machineRemove", machine: machine.value.metadata.id, cluster: clusterName.value },
  });
};

const machineLogs = () => {
  router.push({
    name: "MachineLogs",
    params: { machine: machine.value.metadata.id },
  });
};

const showCluster = () => {
  router.push({
    name: "ClusterOverview",
    params: { cluster: clusterName.value },
  });
};

const timeGetter = (fn :() => string|undefined) => {
  return computed(() => {
    const time = fn();
    if (time) {
      const dt: DateTime = /^\d+$/.exec(time) ? DateTime.fromSeconds(parseInt(time)) : DateTime.fromISO(time)

      return dt.setLocale('en').toLocaleString(DateTime.DATETIME_FULL_WITH_SECONDS);
    }

    return "Never";
  });
}

const machineLastAlive = timeGetter(() => machine.value?.spec?.siderolink_counter?.last_alive);
const machineCreatedAt = timeGetter(() => machine.value?.spec?.machine_created_at);
const secureBoot = computed(() => {
  const securityState = machine.value.spec.message_status?.security_state;
  if (!securityState) {
    return "Unknown";
  }

  return securityState.secure_boot ? "Enabled" : "Disabled";
});

const copyMachineID = () => {
  copyText(machine.value.metadata.id!, undefined, () => {});
};

const canDoMaintenanceUpdate = computed(() => {
  if (machine.value.spec.message_status?.cluster) {
    return false;
  }

  return machine.value.metadata.labels?.[MachineStatusLabelInstalled] !== undefined;
});

const maintenanceUpdateDescription = computed(() => {
  if (machine.value.spec.message_status?.cluster) {
    return "Maintenance upgrade is not possible: machine is a part of a cluster";
  }

  if (!canDoMaintenanceUpdate.value) {
    return "Maintenance upgrade is not possible: Talos is not installed";
  }

  return "Change Talos version";
});
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
