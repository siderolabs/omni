<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="flex px-8 my-7 items-center">
      <div class="heading">Machine Set Scaling Configuration</div>
    </div>
    <div class="flex-1 flex flex-col">
      <div class="flex gap-2 items-center text-sm border-b border-naturals-N4 px-8 py-2 flex-wrap">
        <div class="w-32">
          Update Strategy
        </div>
        <t-button-group :options="options" v-model="updateStrategy" class="flex-1"/>
        <template v-if="updateStrategy !== MachineSetSpecUpdateStrategy.Unset">
          <div>
            Max Parallelism
          </div>
          <div>
            <t-input type="number" v-model="updateParallelism" class="h-7 w-12"/>
          </div>
        </template>
        <div v-else class="h-7 flex items-center">
          Update All Simultaneously
        </div>
      </div>
      <div class="flex gap-2 items-center text-sm border-b border-naturals-N4 px-8 py-2 flex-wrap">
        <div class="w-32">
          Delete Strategy
        </div>
        <t-button-group :options="options" v-model="deleteStrategy" class="flex-1"/>
        <template v-if="deleteStrategy !== MachineSetSpecUpdateStrategy.Unset">
          <div>
            Max Parallelism
          </div>
          <div>
            <t-input type="number" v-model="deleteParallelism" class="h-7 w-12"/>
          </div>
        </template>
        <div v-else class="h-7 flex items-center">
          <span>Delete All Simultaneously</span>
        </div>
      </div>
    </div>
    <div class="flex p-4 gap-4 bg-naturals-N3 rounded-b justify-between">
      <t-button type="secondary" @click="close">Cancel</t-button>
      <t-button @click="saveAndClose">Save</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, toRefs } from "vue";
import { closeModal } from "@/modal";

import TButton from "@/components/common/Button/TButton.vue";
import TInput from "@/components/common/TInput/TInput.vue";

import { MachineSet, state } from "@/states/cluster-management";
import { MachineSetSpecUpdateStrategy } from "@/api/omni/specs/omni.pb";
import TButtonGroup from "@/components/common/Button/TButtonGroup.vue";

const options = [
  {
    label: "Unrestricted",
    value: MachineSetSpecUpdateStrategy.Unset
  },
  {
    label: "Rolling",
    value: MachineSetSpecUpdateStrategy.Rolling
  },
];

interface Props {
  machineSet: MachineSet
}

const props = defineProps<Props>();

const { machineSet } = toRefs(props);

const updateParallelism = ref(machineSet.value.updateStrategy?.config?.rolling?.max_parallelism ?? 1);
const deleteParallelism = ref(machineSet.value.deleteStrategy?.config?.rolling?.max_parallelism ?? 1);

const updateStrategy = ref<MachineSetSpecUpdateStrategy>(machineSet.value.updateStrategy?.type ?? MachineSetSpecUpdateStrategy.Rolling)
const deleteStrategy = ref<MachineSetSpecUpdateStrategy>(machineSet.value.deleteStrategy?.type ?? MachineSetSpecUpdateStrategy.Unset)

const close = () => {
  closeModal();
};

const saveAndClose = async () => {
  const ms = state.value.machineSets.find(item => item.id === machineSet.value.id);
  if (ms) {
    if (updateStrategy.value !== undefined) {
      ms.updateStrategy = {
        type: updateStrategy.value,
        config: {
          rolling: {
            max_parallelism: updateParallelism.value
          }
        }
      }
    }

    if (deleteStrategy.value !== undefined) {
      ms.deleteStrategy = {
        type: deleteStrategy.value,
        config: {
          rolling: {
            max_parallelism: deleteParallelism.value
          }
        }
      }
    }
  }

  close();
}
</script>

<style scoped>
.modal-window {
  @apply p-0;
}
.heading {
  @apply text-xl text-naturals-N14;
}
</style>
