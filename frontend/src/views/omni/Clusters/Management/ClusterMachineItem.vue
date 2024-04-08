<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-list-item>
    <template #default>
      <div class="flex items-center text-naturals-N13">
        <div class="truncate flex-1 flex items-center gap-2">
          <span class="font-bold pr-2">
            <word-highlighter
                :query="(searchQuery ?? '')"
                :textToHighlight="item?.spec?.network?.hostname ?? item?.metadata?.id"
                split-by-space
                highlightClass="bg-naturals-N14"/>
          </span>
          <machine-item-labels :resource="item" :add-label-func="addMachineLabels" :remove-label-func="removeMachineLabels" @filter-label="e => $emit('filterLabel', e)"/>
        </div>
        <div class="flex justify-end flex-initial w-128 gap-4 items-center">
          <template v-if="machineSetIndex !== undefined">
            <div v-if="systemDiskPath" class="pr-8 pl-3 py-1.5 text-naturals-N11 rounded border border-naturals-N6 cursor-not-allowed">
              Install Disk: {{ systemDiskPath }}
            </div>
            <div v-else>
              <t-select-list
                class="h-7"
                title="Install Disk"
                @checkedValue="setInstallDisk"
                :values="disks"
                :defaultValue="disks[0]"/>
            </div>
          </template>
          <div>
            <machine-set-picker :options="options" :machine-set-index="machineSetIndex" @update:machineSetIndex="value => machineSetIndex = value"/>
          </div>
          <div class="flex items-center">
            <icon-button
              class="text-naturals-N14 my-auto"
              @click="openPatchConfig"
              :id="machineSetIndex !== undefined ? options?.[machineSetIndex]?.id : undefined"
              :disabled="machineSetIndex === undefined || options?.[machineSetIndex]?.disabled"
              :icon="machineSetNode.patches[machinePatchID] && machineSetIndex ? 'settings-toggle': 'settings'"/>
          </div>
        </div>
      </div>
    </template>
    <template #details>
      <div class="pl-6 grid grid-cols-5">
        <div class="mb-2 mt-4">Processors</div>
        <div class="mb-2 mt-4">Memory</div>
        <div class="mb-2 mt-4">Block Devices</div>
        <div class="mb-2 mt-4">Addresses</div>
        <div class="mb-2 mt-4">Network Interfaces</div>
        <div>
          <div v-for="(processor, index) in item?.spec?.hardware?.processors" :key="index">
            {{ processor.frequency / 1000 }} GHz, {{ processor.core_count }} {{ pluralize("core", processor.core_count) }}, {{ processor.description }}
          </div>
        </div>
        <div>
          <div v-for="(mem, index) in memoryModules" :key="index">
            {{ formatBytes((mem?.size_mb || 0) * 1024 * 1024) }} {{ mem.description }}
          </div>
        </div>
        <div>
          <div v-for="(dev, index) in item?.spec?.hardware?.blockdevices" :key="index">
            {{ dev.linux_name }} {{ formatBytes(dev.size) }} {{ dev.type }}
          </div>
        </div>
        <div>
          <div>
            {{ item.spec?.network?.addresses?.join(", ") }}
          </div>
        </div>
        <div>
          <div v-for="(link, index) in item?.spec?.network?.network_links" :key="index">
            {{ link.linux_name }} {{ link.hardware_address }} {{ link.link_up ? 'UP' : 'DOWN' }}
          </div>
        </div>
      </div>
    </template>
  </t-list-item>
</template>

<script setup lang="ts">
import { computed, Ref, ref, toRefs, watch } from "vue";
import { formatBytes } from "@/methods";
import {
  LabelControlPlaneRole, PatchBaseWeightClusterMachine, PatchWeightInstallDisk,
} from "@/api/resources";
import pluralize from 'pluralize';
import { showModal } from "@/modal";
import {
  MachineStatusSpec,
  MachineStatusSpecHardwareStatusBlockDevice,
  MachineConfigGenOptionsSpec,
} from "@/api/omni/specs/omni.pb";
import { SiderolinkSpec } from "@/api/omni/specs/siderolink.pb";
import { ResourceTyped } from "@/api/grpc";

import TListItem from "@/components/common/List/TListItem.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import ConfigPatchEdit from "@/views/omni/Modals/ConfigPatchEdit.vue";
import MachineItemLabels from "@/views/omni/ItemLabels/ItemLabels.vue";
import WordHighlighter from "vue-word-highlighter";
import {addMachineLabels, removeMachineLabels} from "@/methods/machine";
import { MachineSet, MachineSetNode, state, PatchID } from "@/states/cluster-management";
import MachineSetPicker, { PickerOption } from "./MachineSetPicker.vue";

import yaml from "js-yaml";

type MemModule = {
  size_mb?: number,
  description?: string
}

defineEmits(['filterLabel']);

const props = defineProps<{
  item: ResourceTyped<MachineStatusSpec & SiderolinkSpec & MachineConfigGenOptionsSpec>,
  reset?: number,
  searchQuery?: string,
  talosVersionNotAllowed: boolean
}>();

const { item, reset, talosVersionNotAllowed } = toRefs(props);

const machineSetNode = ref<MachineSetNode>({
  patches: {},
});
const machineSetIndex = ref<number | undefined>();
const systemDiskPath: Ref<string | undefined> = ref();
const disks: Ref<string[]> = ref([]);

const computeState = () => {
  const bds: MachineStatusSpecHardwareStatusBlockDevice[] = item?.value?.spec?.hardware?.blockdevices || [];
  const diskPaths: string[] = [];

  const index = bds?.findIndex((device: MachineStatusSpecHardwareStatusBlockDevice) => device.system_disk);
  if (index >= 0) {
    systemDiskPath.value = bds[index].linux_name;
  }

  for (const device of bds) {
    diskPaths.push(device.linux_name!);
  }

  disks.value = diskPaths;

  computeMachineAssignment()
}

const computeMachineAssignment = () => {
  for (var i = 0; i < state.value.machineSets.length; i++) {
    const machineSet = state.value.machineSets[i];

    for(const id in machineSet.machines) {
      if (item.value.metadata.id === id) {
        machineSetIndex.value = i;

        return;
      }
    }
  }

  machineSetIndex.value = undefined;
}

computeState();

watch(item, computeState);
watch(state.value, computeMachineAssignment);

watch(talosVersionNotAllowed, (value: boolean) => {
  if (value) {
    machineSetIndex.value = undefined;
  }
});

watch(machineSetIndex, (val?: number, old?: number) => {
  if (val !== undefined) {
    state.value.setMachine(val, item.value.metadata.id!, machineSetNode.value);
  }

  if (old !== undefined) {
    state.value.removeMachine(old, item.value.metadata.id!);
  }
});

if (reset) {
  watch(reset, () => {
    machineSetIndex.value = undefined;
  });
}

watch(machineSetNode, () => {
  if (!machineSetIndex.value) {
    return;
  }

  state.value.setMachine(machineSetIndex.value, item.value.metadata.id!, machineSetNode.value);
});

const filterValid = (modules: MemModule[]): MemModule[] => {
  return modules.filter((mem) => mem.size_mb);
};

const memoryModules = computed(() => {
  return filterValid(item?.value?.spec?.hardware?.memory_modules || []);
});

const options: Ref<PickerOption[]> = computed(() => {
  let memoryCapacity = 0;
  for (const mem of memoryModules.value) {
    memoryCapacity += mem.size_mb ?? 0;
  }

  const cpMemoryThreshold = 2 * 1024;
  const workerMemoryTheshold = 1024;

  const canUseAsControlPlane = memoryCapacity == 0 || memoryCapacity >= cpMemoryThreshold;
  const canUseAsWorker = memoryCapacity == 0 || memoryCapacity >= workerMemoryTheshold;

  return state.value.machineSets.map((ms: MachineSet) => {
    let disabled = ms.role === LabelControlPlaneRole ? !canUseAsControlPlane : !canUseAsWorker;
    let tooltip: string | undefined;

    if (disabled) {
      if (ms.role === LabelControlPlaneRole) {
        tooltip = `The node must have more than ${formatBytes(cpMemoryThreshold * 1024 * 1024)} of RAM to be used as a control plane`;
      } else {
        tooltip = `The node must have more than ${formatBytes(workerMemoryTheshold * 1024 * 1024)} of RAM to be used as a worker`;
      }
    }

    if (ms.machineClass) {
      disabled = true;
      tooltip = `The machine class ${ms.id} is using machine class so no manual allocation is possible`
    }

    if (talosVersionNotAllowed.value) {
      disabled = true;
      tooltip = `The machine has newer Talos version installed: downgrade is not allowed. Upgrade the machine or change Talos cluster version`
    }

    return {
      id: ms.id,
      disabled: disabled,
      tooltip: tooltip,
      color: ms.color,
    }
  });
});

const machinePatchID = `cm-${item.value.metadata.id!}`;
const installDiskPatchID = `cm-${item.value.metadata.id!}-${PatchID.InstallDisk}`;

const setInstallDisk = (value: string) => {
  machineSetNode.value.patches[installDiskPatchID] = {
    data: yaml.dump({
      machine: {
        install: {
          disk: value,
        }
      }
    }),
    systemPatch: true,
    weight: PatchWeightInstallDisk,
    nameAnnotation: PatchID.InstallDisk,
  };
};

const openPatchConfig = () => {
  showModal(
    ConfigPatchEdit,
    {
      tabs: [
        {
          config: machineSetNode.value.patches[machinePatchID]?.data ?? `# Machine config patch for node "${item.value.metadata.id}"

# You can write partial Talos machine config here which will override the default
# Talos machine config for this machine generated by Omni.

# example (changing the node hostname):
machine:
  network:
    hostname: "${item.value.metadata.id}"
`,
          id: `Node ${item.value.metadata.id}`,
        }
      ],
      onSave(config: string) {
        if (!config) {
          delete machineSetNode.value.patches[machinePatchID];

          return;
        }

        machineSetNode.value.patches[machinePatchID] = {
          data: config,
          weight: PatchBaseWeightClusterMachine,
        };
      }
    },
  )
};
</script>
