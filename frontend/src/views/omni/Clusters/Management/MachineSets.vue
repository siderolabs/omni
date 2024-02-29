<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <t-button icon="plus" class="h-8" @click="addWorkers" type="compact">
      Add Worker Machine Sets
    </t-button>
    <machine-set-config
      v-for="(machineSet, index) in state.machineSets"
      :machine-classes="machineClasses"
      :model-value="machineSet"
      :key="machineSet.name"
      @update:model-value="(value) => state.machineSets[index] = value"
      :no-remove="index < 2"
      :onRemove="() => {
        state.removeMachineSet(index)
      }"/>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb';
import { Resource } from '@/api/grpc';
import { DefaultNamespace, LabelWorkerRole, MachineClassType } from '@/api/resources';
import Watch from '@/api/watch';
import { ref } from 'vue';
import MachineSetConfig from './MachineSetConfig.vue';
import TButton from '@/components/common/Button/TButton.vue';
import { state } from "@/states/cluster-management";

const machineClasses = ref<Resource[]>([]);
const machineClassesWatch = new Watch(machineClasses);
machineClassesWatch.setup({
  resource: {
    type: MachineClassType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni
});

const addWorkers = () => {
  state.value.addMachineSet(LabelWorkerRole);
}
</script>