<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import TabButton from '@/components/common/Tabs/TabButton.vue'
import TabContent from '@/components/common/Tabs/TabContent.vue'
import Tabs from '@/components/common/Tabs/Tabs.vue'
import NodesHeader from '@/views/cluster/Nodes/NodesHeader.vue'

const route = useRoute()
const machine = computed(() => route.params.machine as string)

const routes = computed(() => {
  return [
    {
      name: 'Overview',
      to: { name: 'NodeOverview', params: { machine: machine.value } },
    },
    {
      name: 'Monitor',
      to: { name: 'NodeMonitor', params: { machine: machine.value } },
    },
    {
      name: 'Console Logs',
      to: { name: 'NodeLogs', params: { machine: machine.value, service: 'machine' } },
    },
    {
      name: 'Config',
      to: { name: 'NodeConfig', params: { machine: machine.value } },
    },
    {
      name: 'Pending Updates',
      to: { name: 'NodePendingUpdates', params: { machine: machine.value } },
    },
    {
      name: 'Config History',
      to: { name: 'NodeConfigDiffs', params: { machine: machine.value } },
    },
    {
      name: 'Patches',
      to: { name: 'NodePatches', params: { machine: machine.value } },
    },
    {
      name: 'Mounts',
      to: { name: 'NodeMounts', params: { machine: machine.value } },
    },
    {
      name: 'Extensions',
      to: { name: 'NodeExtensions', params: { machine: machine.value } },
    },
  ]
})
</script>

<template>
  <div class="flex h-full flex-col pt-6">
    <NodesHeader class="px-4 md:px-6" />

    <Tabs
      :model-value="$route.name?.toString()"
      class="grow overflow-y-hidden"
      tabs-list-class="px-4 md:px-6"
    >
      <template #triggers>
        <TabButton v-for="{ name, to } in routes" :key="name" :as="RouterLink" :value="to.name" :to>
          {{ name }}
        </TabButton>
      </template>

      <template #contents>
        <TabContent
          v-for="{ name, to } in routes"
          :key="name"
          class="mt-4 grow overflow-y-auto"
          :class="{ 'px-4 md:px-6': to.name !== 'NodeExtensions' }"
          :value="to.name"
        >
          <RouterView class="h-full" />
        </TabContent>
      </template>
    </Tabs>
  </div>
</template>
