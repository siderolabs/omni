<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window flex flex-col gap-4" style="height: 90%;">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Set Extensions
      </h3>
      <close-button @click="close" />
    </div>

    <div v-if="machineStatus" class="flex flex-col gap-4 flex-1 overflow-hidden">
      <extensions-picker
        v-model="requestedExtensions"
        :talos-version="machineStatus?.spec.talos_version!.slice(1)" class="flex-1"
        />

      <div class="flex justify-between gap-4">
        <t-button @click="close" type="secondary">
          Cancel
        </t-button>
        <div class="flex gap-4">
          <t-button @click="() => updateExtensions()" icon="reset" :disabled="modelValue === undefined">
            Revert
          </t-button>
          <t-button @click="() => updateExtensions(requestedExtensions)" type="highlighted">
            Save
          </t-button>
        </div>
      </div>

    </div>
    <div v-else class="flex items-center justify-center">
      <t-spinner class="w-6 h-6"/>
    </div>
  </div>
</template>

<script setup lang="ts">
import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import ExtensionsPicker from "@/views/omni/Extensions/ExtensionsPicker.vue";
import { computed, ref, toRefs, watch } from "vue";
import Watch from "@/api/watch";
import { Resource } from "@/api/grpc";
import { MachineStatusSpec } from "@/api/omni/specs/omni.pb";
import { DefaultNamespace, MachineStatusType } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import { closeModal } from "@/modal";

const props = defineProps<{
  machine: string
  modelValue?: string[]
  onSave: (e?: string[]) => void
}>();

const {
  machine,
  modelValue,
} = toRefs(props);

const close = () => {
  closeModal();
};

const requestedExtensions = ref<Record<string, boolean>>({});

if (modelValue.value) {
  for (const key of modelValue.value) {
    requestedExtensions.value[key] = true;
  }
}

const machineStatus = ref<Resource<MachineStatusSpec>>();
const machineStatusWatch = new Watch(machineStatus);

watch(machineStatus, () => {
  if (modelValue.value !== undefined) {
    return;
  }

  const extensions = machineStatus.value?.spec.schematic?.extensions;
  if (!extensions) {
    return;
  }

  requestedExtensions.value = {};

  for (const extension of extensions) {
    requestedExtensions.value[extension] = true;
  }
});

machineStatusWatch.setup(computed(() => {
  return {
    resource: {
      id: machine.value,
      namespace: DefaultNamespace,
      type: MachineStatusType,
    },
    runtime: Runtime.Omni
  }
}));

const updateExtensions = (extensions?: Record<string, boolean>) => {
  if (extensions === undefined) {
    props.onSave();
  } else {
    const list: string[] = [];
    for (const key in extensions) {
      if (!extensions[key]) {
        continue;
      }

      list.push(key);
    }

    list.sort();

    props.onSave(list);
  }

  close();
}
</script>

<style scoped>
.modal-window {
  @apply w-1/2 h-auto p-8;
}

.heading {
  @apply flex justify-between items-center text-xl text-naturals-N14;
}
</style>
