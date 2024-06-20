<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex items-center h-3 gap-1 hover:bg-naturals-N3 cursor-pointer text-xs py-6 pl-3 pr-4 text-naturals-N14">
    <div class="w-5 pointer-events-none"/>
    <div class="flex-1 grid grid-cols-4 -mr-3 items-center" @click="openNodeInfo">
      <div class="col-span-2 flex items-center gap-2">
        <t-icon icon="server" class="w-4 h-4 ml-2"/>
        <router-link :to="{ name: 'NodeOverview', params: { cluster: clusterName, machine: machine.metadata.id }}" class="list-item-link">
          {{ nodeName }}
        </router-link>
      </div>
      <div class="flex justify-between gap-2 col-span-2 pr-2">
        <div class="flex gap-2 truncate">
          <cluster-machine-phase :machine="machine"/>
          <div v-if="lockedUpdate" class="flex gap-1 items-center text-light-blue-400 truncate">
            <t-icon icon="time" class="h-4 w-4 min-w-max"/>
            <div class="flex-1 truncate">Pending Config Update</div>
          </div>
        </div>
        <div class="flex gap-2 items-center"
          v-if="machineSet.spec.machine_class == undefined && machine?.metadata?.labels?.[LabelWorkerRole] !== undefined">
          <tooltip
            description="Lock machine config. Pause Kubernetes and Talos updates on the machine.">
            <icon-button :icon="locked ? lockedUpdate ? 'locked-toggle' : 'locked' : 'unlocked'" class="w-4 h-4 ml-1 mt-1" @click.stop="updateLock"/>
          </tooltip>
        </div>
      </div>
    </div>
    <node-context-menu :cluster-machine-status="machine" :cluster-name="clusterName" :delete-disabled="deleteDisabled!"/>
  </div>
</template>

<script setup lang="ts">
import { LabelHostname, LabelCluster, MachineLocked, LabelWorkerRole, UpdateLocked } from "@/api/resources";
import { useRouter } from "vue-router";
import { computed, toRefs } from "vue";
import { Resource } from "@/api/grpc";
import { ClusterMachineStatusLabelNodeName } from "@/api/resources";
import { ClusterMachineStatusSpec, MachineSetStatusSpec } from "@/api/omni/specs/omni.pb";
import { updateMachineLock } from "@/methods/machine";
import { showError } from "@/notification";

import ClusterMachinePhase from "./ClusterMachinePhase.vue";
import NodeContextMenu from "@/views/common/NodeContextMenu.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";

const props = defineProps<{
  machineSet: Resource<MachineSetStatusSpec>,
  machine: Resource<ClusterMachineStatusSpec>,
  deleteDisabled?: boolean,
}>();

const { machine } = toRefs(props);

const locked = computed(() => {
  return machine.value?.metadata?.annotations?.[MachineLocked] !== undefined;
});

const router = useRouter();

const hostname = computed(() => ((props.machine?.metadata?.labels || {})[LabelHostname] ?? props.machine?.metadata.id))
const nodeName = computed(() => (props.machine?.metadata?.labels || {})[ClusterMachineStatusLabelNodeName] || hostname.value);
const clusterName = computed(() => (props.machine?.metadata?.labels || {})[LabelCluster]);

const openNodeInfo = async () => {
  router.push({ name: "NodeOverview", params: { cluster: clusterName.value, machine: props.machine.metadata.id } });
};

const lockedUpdate = computed(() => {
  return machine.value.metadata.labels?.[UpdateLocked] !== undefined
})

const updateLock = async () => {
  if (!props.machine.metadata.id) {
    return;
  }

  try {
    await updateMachineLock(props.machine.metadata.id, !locked.value);
  } catch (e) {
    showError("Failed To Update Machine Lock", e.message);
  }
}
</script>
