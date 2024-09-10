<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="text-xs px-2 py-2 pr-3 border rounded my-1 border-naturals-N5 bg-naturals-N3 text-naturals-N13 flex gap-2 items-center">
    <machine-set-label :color="modelValue.color" class="w-10" :machine-set-id="modelValue.id"/>
    <div class="flex-1 flex flex-wrap items-center gap-x-4 gap-y-1">
      <div class="w-32 truncate" :title="modelValue.name">
        {{ modelValue.name }}
      </div>
      <div class="flex gap-2 items-center">
        Allocation Mode: <t-button-group :options="allocationModes" v-model="allocationMode"/>
      </div>
      <template v-if="useMachineClasses">
        <t-select-list v-if="machineClasses"
            class="h-6 w-48"
            @checkedValue="(value: string) => { machineClass = value }"
            title="Name"
            :defaultValue="machineClass ?? machineClassOptions[0]"
            :values="machineClassOptions"
          />
        <t-spinner v-else class="h-4 w-4"/>
      </template>
      <t-checkbox v-if="useMachineClasses" :checked="unlimited" label="Use All Available Machines" @click="unlimited = !unlimited" class="h-6"/>
      <div class="w-32" v-if="!unlimited">
        <t-input class="h-6" title="Size" v-if="useMachineClasses" type="number" :min="0" v-model="machineCount" compact/>
        <div v-else>{{ pluralize('Machines', Object.keys(modelValue.machines).length, true) }}</div>
      </div>
    </div>
    <div class="w-24 flex justify-end items-center gap-2">
      <t-button v-if="!noRemove" class="h-6" type="compact" @click="onRemove">Remove</t-button>
      <div class="flex gap-1 justify-center">
        <icon-button
          v-if="modelValue.role === LabelWorkerRole"
          @click="openMachineSetConfig"
          icon="chart-bar"/>
        <icon-button
          @click="openPatchConfig"
          :icon="patches[PatchID.Default] ? 'settings-toggle' : 'settings'"/>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, toRefs, watch, Ref } from "vue";
import { Resource } from "@/api/grpc";
import { MachineSet, PatchID, ConfigPatch } from "@/states/cluster-management";
import { showModal } from "@/modal";

import TButtonGroup from "@/components/common/Button/TButtonGroup.vue";
import MachineSetLabel from "@/views/omni/Clusters/Management/MachineSetLabel.vue";
import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import TButton from "@/components/common/Button/TButton.vue";
import ConfigPatchEdit from "@/views/omni/Modals/ConfigPatchEdit.vue";
import IconButton from "@/components/common/Button/IconButton.vue";

import pluralize from "pluralize";
import { LabelWorkerRole, PatchBaseWeightMachineSet } from "@/api/resources";
import MachineSetConfigEdit from "../../Modals/MachineSetConfigEdit.vue";

const emit = defineEmits(["update:modelValue"]);

enum AllocationMode {
  Manual = "Manual",
  MachineClass = "Machine Class",
  RequestSet = "Machine Request Set"
}

const allocationMode = ref(AllocationMode.Manual);

const allocationModes = computed(() => {
  const res = [
    {
      label: AllocationMode.Manual,
      value: AllocationMode.Manual,
    },
    {
      label: AllocationMode.MachineClass,
      value: AllocationMode.MachineClass,
      disabled: !machineClasses?.value?.length,
      tooltip: !machineClasses?.value?.length ? "No Machine Classes Available" : undefined
    },
  ];

  return res;
});

const props = defineProps<{
  noRemove?: boolean,
  onRemove?: () => void,
  machineClasses?: Resource[],
  modelValue: MachineSet
}>();

const { machineClasses, modelValue } = toRefs(props);

const machineClassOptions = computed(() => {
  return machineClasses?.value?.map((r: Resource) => r.metadata.id!) || [];
});

const useMachineClasses = computed(() => allocationMode.value === AllocationMode.MachineClass);
const machineClass = ref(modelValue.value.machineAllocation?.name);
const machineCount = ref(modelValue.value.machineAllocation?.size ?? 1);
const patches: Ref<Record<string, ConfigPatch>> = ref(modelValue.value.patches);
const unlimited = ref(modelValue.value.machineAllocation?.size === "unlimited");

watch(modelValue, () => {
  machineClass.value = modelValue.value.machineAllocation?.name;
  machineCount.value = typeof modelValue.value.machineAllocation?.size === 'number' ? modelValue.value.machineAllocation?.size : 1;
  patches.value = modelValue.value.patches;

  if (modelValue.value.machineAllocation) {
    allocationMode.value = AllocationMode.MachineClass;
  }
});

watch([machineClass, machineCount, useMachineClasses, patches, unlimited], () => {
  if (useMachineClasses.value && !machineClass.value && machineClassOptions.value.length > 0) {
    machineClass.value = machineClassOptions.value[0];
  }

  const mc = useMachineClasses.value && machineClass.value !== undefined ? {
    name: machineClass.value,
    size: unlimited.value ? 'unlimited' : machineCount.value,
  } : undefined;

  const machineSet: MachineSet = {
    ...modelValue.value,
    machineClass: mc,
    patches: patches.value,
  }

  emit("update:modelValue", machineSet);
});

const openMachineSetConfig = () => {
  showModal(
    MachineSetConfigEdit,
    {
      machineSet: modelValue.value,
    },
  );
}

const openPatchConfig = () => {
  showModal(
    ConfigPatchEdit,
    {
      tabs: [
        {
          config: patches.value[PatchID.Default]?.data ?? "",
          id: `Machine Set ${modelValue.value.name}`,
        }
      ],
      onSave(config: string) {
        if (!config) {
          delete patches.value[PatchID.Default];

          return;
        }

        patches.value = {
          [PatchID.Default]: {
            data: config,
            weight: PatchBaseWeightMachineSet,
          },
          ...patches.value
        }
      }
    },
  )
};
</script>
