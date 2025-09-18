<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, ref, toRefs } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterDiagnosticsSpec, MachineSetSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterDiagnosticsType,
  DefaultNamespace,
  LabelCluster,
  LabelControlPlaneRole,
  MachineSetNodeType,
  MachineSetStatusType,
  MachineSetType,
} from '@/api/resources'
import WatchResource, { itemID } from '@/api/watch'
import Watch from '@/components/common/Watch/Watch.vue'
import { sortMachineSetIds } from '@/methods/machineset'

import MachineSet from './MachineSet.vue'

const props = defineProps<{
  clusterID: string
}>()

const { clusterID } = toRefs(props)

const machineSets: Ref<Resource<MachineSetSpec>[]> = ref([])
const clusterDiagnostics: Ref<Resource<ClusterDiagnosticsSpec> | undefined> = ref()

const machineSetsWatch = new WatchResource(machineSets)
const clusterDiagnosticsWatch = new WatchResource(clusterDiagnostics)

machineSetsWatch.setup(
  computed(() => {
    return {
      resource: {
        type: MachineSetType,
        namespace: DefaultNamespace,
      },
      runtime: Runtime.Omni,
      selectors: [`${LabelCluster}=${clusterID.value}`],
    }
  }),
)

clusterDiagnosticsWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterDiagnosticsType,
    id: clusterID.value,
  },
})

const nodesWithDiagnostics = computed(() => {
  const nodes = clusterDiagnostics.value?.spec?.nodes?.map((node) => node.id ?? '') ?? []
  return new Set(nodes)
})

const watches = computed(() =>
  sortMachineSetIds(
    clusterID.value,
    machineSets.value.map((machineSet) => machineSet?.metadata?.id ?? ''),
  ),
)

const controlPlaneNodes = ref<Resource[]>([])

const machineNodesWatch = new WatchResource(controlPlaneNodes)
machineNodesWatch.setup(
  computed(() => {
    return {
      resource: {
        type: MachineSetNodeType,
        namespace: DefaultNamespace,
      },
      runtime: Runtime.Omni,
      selectors: [`${LabelControlPlaneRole}`],
    }
  }),
)
</script>

<template>
  <div>
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
        />
      </template>
    </Watch>
  </div>
</template>
