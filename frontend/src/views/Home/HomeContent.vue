<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { ClusterStatusSpec, MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterStatusType, DefaultNamespace, MachineStatusType } from '@/api/resources'
import { usePermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import HomeClustersChart from '@/views/Home/HomeClustersChart.vue'
import HomeClustersTutorialCard from '@/views/Home/HomeClustersTutorialCard.vue'
import HomeGeneralInformation from '@/views/Home/HomeGeneralInformation.vue'
import HomeMachinesChart from '@/views/Home/HomeMachinesChart.vue'
import HomeMachinesTutorialCard from '@/views/Home/HomeMachinesTutorialCard.vue'
import HomeRecentClusters from '@/views/Home/HomeRecentClusters.vue'
import HomeRecentMachines from '@/views/Home/HomeRecentMachines.vue'

const { canReadClusters, canReadMachines } = usePermissions()

const { data: clusters, loading: clustersLoading } = useResourceWatch<ClusterStatusSpec>(() => ({
  skip: !canReadClusters.value,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
  },
  runtime: Runtime.Omni,
  sortByField: 'created',
  sortDescending: true,
}))

const { data: machines, loading: machinesLoading } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !canReadMachines.value,
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  runtime: Runtime.Omni,
  sortByField: 'created',
  sortDescending: true,
}))

// Disabled entirely until we decide where to get the information from
const showReleaseNotes = false
</script>

<template>
  <div class="flex flex-col gap-6">
    <header>
      <h1 class="text-xl font-medium text-naturals-n14">Home</h1>
    </header>

    <div class="grid grid-cols-1 items-start gap-4 lg:grid-cols-[1fr_auto]">
      <div
        class="grid grid-cols-1 gap-4"
        :class="{
          'min-[118.75rem]:grid-cols-2': showReleaseNotes && canReadClusters && canReadMachines,
          'xl:grid-cols-2': showReleaseNotes && (!canReadClusters || !canReadMachines),
        }"
      >
        <div v-if="canReadClusters || canReadMachines" class="flex flex-col gap-4">
          <template
            v-if="canReadClusters && canReadMachines && !machinesLoading && !clustersLoading"
          >
            <HomeMachinesTutorialCard v-if="!machines.length" />
            <HomeClustersTutorialCard v-else-if="!clusters.length" />
          </template>

          <div
            class="grid grid-cols-1 gap-2"
            :class="{ 'xl:grid-cols-2': canReadClusters && canReadMachines }"
          >
            <HomeClustersChart v-if="canReadClusters" />
            <HomeMachinesChart v-if="canReadMachines" />
          </div>

          <div class="flex flex-col gap-2">
            <HomeRecentClusters v-if="canReadClusters" :clusters :loading="clustersLoading" />
            <HomeRecentMachines v-if="canReadMachines" :machines :loading="machinesLoading" />
          </div>
        </div>

        <div v-if="showReleaseNotes" class="bg-yellow-y1 p-2">Release notes</div>
      </div>

      <HomeGeneralInformation class="lg:w-72" />
    </div>
  </div>
</template>
