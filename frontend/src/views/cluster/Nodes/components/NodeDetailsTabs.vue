<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

import TabButton from '@/components/common/Tabs/TabButton.vue'
import TabsHeader from '@/components/common/Tabs/TabsHeader.vue'

const props = defineProps<{
  machine: string
}>()

const { machine } = toRefs(props)

const routes = computed((): { name: string; to: RouteLocationRaw }[] => {
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
  <TabsHeader class="border-b border-naturals-n4 pb-3.5">
    <TabButton
      is="router-link"
      v-for="route in routes"
      :key="route.name"
      :to="route.to"
      :selected="$route.name === route.to"
    >
      {{ route.name }}
    </TabButton>
  </TabsHeader>
</template>

<style scoped>
@reference "../../../../index.css";

.content {
  @apply flex w-full border-b border-naturals-n4;
}

.router-link-active {
  @apply relative text-naturals-n13;
}

.router-link-active::before {
  @apply absolute block w-full animate-fadein bg-primary-p3;
  content: '';
  height: 2px;
  bottom: -15px;
}
</style>
