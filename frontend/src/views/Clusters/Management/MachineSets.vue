<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { MachineClassSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, LabelWorkerRole, MachineClassType } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { state } from '@/states/cluster-management'

import MachineSetConfig from './MachineSetConfig.vue'

const { data: machineClasses } = useResourceWatch<MachineClassSpec>({
  resource: {
    type: MachineClassType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
})

const addWorkers = () => {
  state.value.addMachineSet(LabelWorkerRole)
}
</script>

<template>
  <div>
    <TButton icon="plus" class="h-8" size="sm" @click="addWorkers">Add Worker Machine Sets</TButton>
    <ul>
      <MachineSetConfig
        v-for="(machineSet, index) in state.machineSets"
        :key="machineSet.name"
        v-model="state.machineSets[index]"
        :talos-version="state.cluster.talosVersion"
        :machine-classes="machineClasses"
        :no-remove="index < 2"
        @on-remove="state.removeMachineSet(index)"
      />
    </ul>
  </div>
</template>
