<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, LabelCluster, MachineStatusType } from '@/api/resources'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const { data: machineStatuses } = useResourceWatch<MachineStatusSpec>(() => ({
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  selectors: [`${LabelCluster}=${clusterId}`],
  runtime: Runtime.Omni,
}))

const machines = computed(() =>
  machineStatuses.value.map((machine) => ({
    label: getDisplayNameForMachine(machine),
    value: machine.metadata.id!,
  })),
)

const selectedMachine = ref<string>()

watchEffect(() => {
  selectedMachine.value = machineId
})

function getDisplayNameForMachine(machine: Resource<MachineStatusSpec>) {
  return machine.spec.network?.hostname || machine.metadata.id!
}
</script>

<template>
  <div class="items-startflex-row flex flex-wrap">
    <div class="flex items-center">
      <RouterLink
        class="p-2 leading-none font-medium transition hover:opacity-50"
        :to="{ name: 'ClusterOverview', params: { cluster: clusterId } }"
      >
        {{ clusterId }}
      </RouterLink>

      <svg
        class="h-5 w-5 shrink-0 opacity-50"
        xmlns="http://www.w3.org/2000/svg"
        fill="currentColor"
        viewBox="0 0 20 20"
        aria-hidden="true"
      >
        <path d="M5.555 17.776l8-16 .894.448-8 16-.894-.448z" />
      </svg>

      <TSelectList
        v-model="selectedMachine"
        variant="breadcrumb"
        :values="machines"
        searcheable
        @update:model-value="(v) => $router.push({ params: { machine: v } })"
      />
    </div>
  </div>
</template>
