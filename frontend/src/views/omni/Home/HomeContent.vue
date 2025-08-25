<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { canReadClusters, canReadMachines } from '@/methods/auth'
import HomeClustersChart from '@/views/omni/Home/HomeClustersChart.vue'
import HomeGeneralInformation from '@/views/omni/Home/HomeGeneralInformation.vue'
import HomeMachinesChart from '@/views/omni/Home/HomeMachinesChart.vue'
import HomeRecentClusters from '@/views/omni/Home/HomeRecentClusters.vue'
import HomeRecentMachines from '@/views/omni/Home/HomeRecentMachines.vue'

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
        <div class="flex flex-col gap-4">
          <div
            v-if="canReadClusters || canReadMachines"
            class="grid grid-cols-1 gap-2"
            :class="{ 'xl:grid-cols-2': canReadClusters && canReadMachines }"
          >
            <HomeClustersChart v-if="canReadClusters" />
            <HomeMachinesChart v-if="canReadMachines" />
          </div>

          <div v-if="canReadClusters || canReadMachines" class="flex flex-col gap-2">
            <HomeRecentClusters v-if="canReadClusters" />
            <HomeRecentMachines v-if="canReadMachines" />
          </div>
        </div>

        <div v-if="showReleaseNotes" class="bg-yellow-y1 p-2">Release notes</div>
      </div>

      <HomeGeneralInformation />
    </div>
  </div>
</template>
