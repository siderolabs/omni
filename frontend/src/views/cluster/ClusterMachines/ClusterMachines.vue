<!--
Copyright (c) 2026 Sidero Labs, Inc.

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
  MachineSetType,
} from '@/api/resources'
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

const machineSetIds = computed(() =>
  sortMachineSetIds(
    clusterID,
    machineSets.value.map((machineSet) => machineSet.metadata.id ?? ''),
  ),
)
</script>

<template>
  <div :class="isSubgrid && 'col-span-full grid grid-cols-subgrid'">
    <MachineSet
      v-for="id in machineSetIds"
      :key="id"
      :machine-set-id="id"
      :nodes-with-diagnostics="nodesWithDiagnostics"
      :is-subgrid
    />
  </div>
</template>
