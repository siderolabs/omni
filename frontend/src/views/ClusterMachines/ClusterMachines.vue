<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'
import { AccordionRoot } from 'reka-ui'
import { computed, watchEffect } from 'vue'

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

const { clusterID } = defineProps<{
  clusterID: string
  isSubgrid?: boolean
}>()

const { data: machineSets, loading: machineSetsLoading } = useResourceWatch<MachineSetSpec>(() => ({
  resource: {
    type: MachineSetType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
  selectors: [`${LabelCluster}=${clusterID}`],
}))

const { data: clusterDiagnostics, loading: diagnosticsLoading } =
  useResourceWatch<ClusterDiagnosticsSpec>(() => ({
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ClusterDiagnosticsType,
      id: clusterID,
    },
  }))

const loading = computed(() => machineSetsLoading.value || diagnosticsLoading.value)

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

const wasSet = useSessionStorage(() => `cluster-machine-sets-expanded-${clusterID}-set`, false)
const expandedMachineSetIds = useSessionStorage<string[]>(
  () => `cluster-machine-sets-expanded-${clusterID}`,
  [],
)

watchEffect(() => {
  if (machineSetIds.value.length && !wasSet.value) {
    expandedMachineSetIds.value = machineSetIds.value
    wasSet.value = true
  }
})
</script>

<template>
  <AccordionRoot
    v-if="!loading"
    v-model="expandedMachineSetIds"
    type="multiple"
    :class="isSubgrid && 'col-span-full grid grid-cols-subgrid'"
    class="collapsible-content-child"
  >
    <MachineSet
      v-for="id in machineSetIds"
      :key="id"
      :machine-set-id="id"
      :nodes-with-diagnostics="nodesWithDiagnostics"
      :is-subgrid
    />
  </AccordionRoot>
</template>

<style scoped>
.collapsible-content-child {
  animation: slideDown 200ms ease-out;
}

@keyframes slideDown {
  from {
    clip-path: inset(0 0 100% 0);
  }
  to {
    clip-path: inset(0 0 0% 0);
  }
}
</style>
