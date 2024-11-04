<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="border-t-8 border-naturals-N4" v-if="machines.length > 0 || requests.length > 0">
    <div class="flex items-center border-naturals-N4 border-b pl-3 text-naturals-N14">
      <div class="py-2 clusters-grid flex-1 items-center">
        <div class="flex flex-wrap gap-2 col-span-2 justify-between items-center">
          <div class="flex-1 flex items-center">
            <div class="flex items-center gap-2 bg-naturals-N4 w-40 rounded truncate py-2 px-3">
              <t-icon icon="server-stack" class="w-4 h-4"/>
              <div class="truncate flex-1">{{ machineSetTitle(clusterID, machineSet?.metadata?.id) }}</div>
            </div>
          </div>
          <div class="flex-1 flex max-md:ml-1 md:ml-10">
            <t-spinner class="w-4 h-4" v-if="scaling"/>
            <div class="flex items-center gap-1" v-else-if="!editingMachinesCount">
              <div class="flex items-center">{{ machineSet?.spec?.machines?.healthy || 0 }}/<div :class="{'text-lg mt-0.5': requestedMachines === '∞'}">{{ requestedMachines }}</div></div>
              <icon-button icon="edit" v-if="machineSet.spec.machine_allocation?.name" @click="editingMachinesCount = !editingMachinesCount"/>
            </div>
            <div v-else class="flex items-center gap-1">
              <div class="w-12">
                <t-input :min="0" class="h-6" compact type="number" v-model="machineCount" @keydown.enter="() => updateMachineCount()"/>
              </div>
              <icon-button icon="check" @click="() => updateMachineCount()"/>
              <t-button v-if="canUseAll" type="subtle" @click="() => updateMachineCount(MachineSetSpecMachineAllocationType.Unlimited)">
                Use All
              </t-button>
            </div>
          </div>
        </div>
        <machine-set-phase :item="machineSet" :class="{'col-span-2': !machineSet.spec?.machine_allocation?.name}" class="ml-2"/>
        <div v-if="machineSet.spec?.machine_allocation?.name" class="rounded bg-naturals-N4 px-3 py-2 max-w-min max-md:col-span-4">
          Machine Class: {{ machineSet.spec?.machine_allocation?.name }} ({{ machineClassMachineCount }})
        </div>
      </div>
      <t-actions-box style="height: 24px" v-if="canRemoveMachineSet" @click.stop>
        <t-actions-box-item icon="delete" danger
          @click="() => openMachineSetDestroy(machineSet)">Destroy Machine Set</t-actions-box-item>
      </t-actions-box>
      <div v-else class="w-6"/>
    </div>
    <cluster-machine class="machine-item" :id="machine.metadata.id" :machine-set="machineSet" :has-diagnostic-info="nodesWithDiagnostics?.has(machine.metadata.id!)"
      v-for="machine in machines" :key="itemID(machine)" :machine="machine"
      :deleteDisabled="!canRemoveMachine"/>
    <machine-request class="machine-item"
      v-for="request in pendingRequests"
      :key="itemID(request)"
      :request-status="request"/>
    <div v-if="hiddenMachinesCount > 0" class="text-xs p-4 pl-9 border-t border-naturals-N4 flex gap-1 items-center">
      {{ pluralize("machine", hiddenMachinesCount, true) }} are hidden
      <t-button type="subtle" @click="showMachinesCount = undefined"><span class="text-xs">Show all...</span></t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Resource, ResourceService } from "@/api/grpc";
import { MachineSetStatusSpec, ClusterMachineStatusSpec, MachineSetSpecMachineAllocationType, MachineClassSpec, ClusterMachineRequestStatusSpec } from "@/api/omni/specs/omni.pb";
import { ClusterMachineRequestStatusType, ClusterMachineStatusType, DefaultNamespace, LabelCluster, LabelControlPlaneRole, LabelMachineSet, MachineClassType, LabelManaged } from "@/api/resources";
import { computed, ref, toRefs, watch } from "vue";
import { useRouter } from "vue-router";
import { setupClusterPermissions } from "@/methods/auth";
import Watch, { itemID } from "@/api/watch";
import { Runtime } from "@/api/common/omni.pb";
import { machineSetTitle, scaleMachineSet } from "@/methods/machineset";
import { controlPlaneMachineSetId, defaultWorkersMachineSetId } from "@/methods/machineset";
import { showError } from "@/notification";
import pluralize from "pluralize";

import TActionsBox from "@/components/common/ActionsBox/TActionsBox.vue";
import TActionsBoxItem from "@/components/common/ActionsBox/TActionsBoxItem.vue";
import ClusterMachine from "./ClusterMachine.vue"
import MachineSetPhase from "./MachineSetPhase.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import { withRuntime } from "@/api/options";
import MachineRequest from "./MachineRequest.vue";

const showMachinesCount = ref<number | undefined>(25);

const props = defineProps<{
  machineSet: Resource<MachineSetStatusSpec>
  nodesWithDiagnostics: Set<string>,
}>();

const { machineSet } = toRefs(props);

const machines = ref<Resource<ClusterMachineStatusSpec>[]>([]);
const machinesWatch = new Watch(machines);

const requests = ref<Resource<ClusterMachineRequestStatusSpec>[]>([]);
const requestsWatch = new Watch(requests);

const clusterID = computed(() => machineSet.value.metadata.labels?.[LabelCluster] ?? "");
const editingMachinesCount = ref(false);
const machineCount = ref(machineSet.value.spec.machine_allocation?.machine_count ?? 1);
const scaling = ref(false);
const canUseAll = ref<boolean | undefined>();

watch(editingMachinesCount, async (enabled: boolean, wasEnabled: boolean) => {
  if (!machineSet.value.spec.machine_allocation?.name) {
    return;
  }

  if (!wasEnabled && enabled && canUseAll.value === undefined) {
    const machineClass: Resource<MachineClassSpec> = await ResourceService.Get({
      type: MachineClassType,
      id: machineSet.value.spec.machine_allocation?.name,
      namespace: DefaultNamespace,
    }, withRuntime(Runtime.Omni));

    canUseAll.value = machineClass.spec.auto_provision === undefined;
  }
});

const hiddenMachinesCount = computed(() => {
  if (showMachinesCount.value === undefined) {
    return 0;
  }

  return Math.max(0, (machineSet.value.spec.machines?.total || 0) - showMachinesCount.value);
});

machinesWatch.setup(computed(() => {
  return {
    resource: {
      namespace: DefaultNamespace,
      type: ClusterMachineStatusType,
    },
    runtime: Runtime.Omni,
    selectors: [
      `${LabelCluster}=${clusterID.value}`,
      `${LabelMachineSet}=${machineSet.value.metadata.id!}`
    ],
    limit: showMachinesCount.value,
  }
}));

requestsWatch.setup(computed(() => {
  if (!machineSet.value.spec.machine_allocation && machineSet.value.metadata.labels?.[LabelManaged] === undefined) {
    return;
  }

  return {
    resource: {
      namespace: DefaultNamespace,
      type: ClusterMachineRequestStatusType,
    },
    selectors: [
      `${LabelCluster}=${clusterID.value}`,
      `${LabelMachineSet}=${machineSet.value.metadata.id!}`
    ],
    runtime: Runtime.Omni,
    limit: showMachinesCount.value,
  };
}));

const pendingRequests = computed(() => {
  return requests.value.filter(item => !machines.value.find(machine => machine.metadata.id === item.spec.machine_uuid))
});

const router = useRouter();

const openMachineSetDestroy = (machineSet: Resource) => {
  router.push({
    query: { modal: "machineSetDestroy", machineSet: machineSet.metadata.id },
  });
};

const { canRemoveClusterMachines } = setupClusterPermissions(clusterID);

const canRemoveMachine = computed(() => {
  if (!canRemoveClusterMachines.value) {
    return false;
  }

  // don't allow destroying machines if the machine set is using automatic allocation
  if (machineSet.value.spec.machine_allocation?.name) {
    return false;
  }

  if (machineSet.value.metadata.labels?.[LabelControlPlaneRole] === undefined) {
    return true;
  }

  return machines.value.length > 1;
});

const canRemoveMachineSet = computed(() => {
  if (!canRemoveClusterMachines.value) {
    return false;
  }

  const deleteProtected = new Set<string>([controlPlaneMachineSetId(clusterID.value), defaultWorkersMachineSetId(clusterID.value)]);

  return !deleteProtected.has(machineSet.value.metadata.id!)
});

const updateMachineCount = async (allocationType: MachineSetSpecMachineAllocationType = MachineSetSpecMachineAllocationType.Static) => {
  scaling.value = true;

  try {
    await scaleMachineSet(machineSet.value.metadata.id!, machineCount.value, allocationType);
  } catch (e) {
    showError(`Failed to Scale Machine Set ${machineSet.value.metadata.id}`, `Error: ${e.message}`);
  }

  scaling.value = false;

  editingMachinesCount.value = false;
};

const requestedMachines = computed(() => {
  if (machineSet.value.spec.machine_allocation?.allocation_type === MachineSetSpecMachineAllocationType.Unlimited) {
    return "∞";
  }

  return machineSet?.value?.spec?.machines?.requested || 0;
})

const machineClassMachineCount = computed(() => {
  if (machineSet.value.spec?.machine_allocation?.allocation_type === MachineSetSpecMachineAllocationType.Unlimited) {
    return "All Machines";
  }

  return pluralize('Machine', machineSet.value.spec?.machine_allocation?.machine_count ?? 0, true)
});
</script>

<style scoped>
.machine-item:not(:last-of-type) {
  @apply border-b border-naturals-N4;
}

.machine-item:last-of-type {
  @apply rounded-b-md;
}
</style>
