<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterDiagnosticsSpec, MachineSetSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterDiagnosticsType,
  DefaultNamespace,
  LabelCluster,
  MachineSetStatusType,
  MachineSetType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import Watch from '@/components/common/Watch/Watch.vue'
import { sortMachineSetIds } from '@/methods/machineset'
import { useResourceWatch } from '@/methods/useResourceWatch'

import MachineSet from './MachineSet.vue'

const { clusterID, pauseWatches } = defineProps<{
  clusterID: string
  pauseWatches?: boolean
  isSubgrid?: boolean
}>()

const { data: machineSets } = useResourceWatch<MachineSetSpec>(() => ({
  skip: pauseWatches,
  resource: {
    type: MachineSetType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
  selectors: [`${LabelCluster}=${clusterID}`],
}))

const { data: clusterDiagnostics } = useResourceWatch<ClusterDiagnosticsSpec>(() => ({
  skip: pauseWatches,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterDiagnosticsType,
    id: clusterID,
  },
}))

const nodesWithDiagnostics = computed(() => {
  const nodes = clusterDiagnostics.value?.spec.nodes?.map((node) => node.id ?? '') ?? []
  return new Set(nodes)
})

const watches = computed(() =>
  sortMachineSetIds(
    clusterID,
    machineSets.value.map((machineSet) => machineSet.metadata.id ?? ''),
  ),
)
</script>

<template>
  <div :class="isSubgrid && 'col-span-full grid grid-cols-subgrid'">
    <Watch
      v-for="id in watches"
      :key="id"
      :opts="{
        resource: {
          namespace: DefaultNamespace,
          type: MachineSetStatusType,
          id,
        },
        runtime: Runtime.Omni,
      }"
    >
      <template #default="{ data }">
        <MachineSet
          v-if="data"
          :key="itemID(data)"
          :machine-set="data"
          :nodes-with-diagnostics="nodesWithDiagnostics"
          :is-subgrid
        />
      </template>
    </Watch>
  </div>
</template>
