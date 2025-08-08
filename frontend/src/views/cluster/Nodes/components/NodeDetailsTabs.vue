<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <tabs-header class="border-b border-naturals-N4 pb-3.5">
    <tab-button
      is="router-link"
      v-for="route in routes"
      :key="route.name"
      :to="route.to"
      :selected="$route.name === route.to"
    >
      {{ route.name }}
    </tab-button>
  </tabs-header>
</template>

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

<style scoped>
.content {
  @apply w-full border-b border-naturals-N4 flex;
}

.router-link-active {
  @apply text-naturals-N13 relative;
}

.router-link-active::before {
  @apply block absolute bg-primary-P3 w-full animate-fadein;
  content: '';
  height: 2px;
  bottom: -15px;
}
</style>
