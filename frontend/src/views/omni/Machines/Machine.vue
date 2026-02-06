<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, type RouterView, useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import PageHeader from '@/components/common/PageHeader.vue'
import TabButton from '@/components/common/Tabs/TabButton.vue'
import TabContent from '@/components/common/Tabs/TabContent.vue'
import Tabs from '@/components/common/Tabs/Tabs.vue'
import { useResourceGet } from '@/methods/useResourceGet'

const routes = computed(() => {
  return [
    {
      name: 'Logs',
      to: { name: 'MachineLogs' },
    },
    {
      name: 'Patches',
      to: { name: 'MachineConfigPatches' },
    },
  ]
})

const route = useRoute()

const { data: machine } = useResourceGet<MachineStatusSpec>(() => ({
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
    id: route.params.machine as string,
  },
  runtime: Runtime.Omni,
}))

const machineName = computed(
  () =>
    machine.value?.spec.network?.hostname ||
    machine.value?.metadata.id ||
    (route.params.machine as string),
)

/**
 * Some child routes do not match any tab, e.g. MachinePatchEdit
 */
const hasMatchingTab = computed(() =>
  routes.value.some((r) => r.to.name === route.name?.toString()),
)
</script>

<template>
  <div class="flex h-full flex-col gap-4">
    <div class="flex h-9 justify-between">
      <PageHeader :title="machineName" />
    </div>

    <Tabs :model-value="$route.name?.toString()" :class="{ grow: hasMatchingTab }">
      <template #triggers>
        <TabButton v-for="{ name, to } in routes" :key="name" :as="RouterLink" :value="to.name" :to>
          {{ name }}
        </TabButton>
      </template>

      <template #contents>
        <TabContent v-for="{ name, to } in routes" :key="name" class="mt-4 grow" :value="to.name">
          <RouterView class="h-full" />
        </TabContent>
      </template>
    </Tabs>

    <RouterView v-if="!hasMatchingTab" class="grow" />
  </div>
</template>
