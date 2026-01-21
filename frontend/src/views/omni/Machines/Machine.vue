<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, onBeforeMount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import PageHeader from '@/components/common/PageHeader.vue'
import TabButton from '@/components/common/Tabs/TabButton.vue'
import TabsHeader from '@/components/common/Tabs/TabsHeader.vue'

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

const getMachineName = async () => {
  const res: Resource<MachineStatusSpec> = await ResourceService.Get(
    {
      namespace: DefaultNamespace,
      type: MachineStatusType,
      id: route.params.machine! as string,
    },
    withRuntime(Runtime.Omni),
  )

  machine.value = res.spec.network?.hostname || res.metadata.id!
}

const machine = ref(route.params.machine)

onBeforeMount(getMachineName)
watch(() => route.params, getMachineName)
</script>

<template>
  <div class="flex h-full flex-col gap-4">
    <div class="flex h-9 justify-between">
      <PageHeader :title="`${machine}`" />
    </div>
    <TabsHeader class="border-b border-naturals-n4 pb-3.5">
      <TabButton
        is="router-link"
        v-for="route in routes"
        :key="route.name"
        :to="route.to"
        :selected="$route.name === route.to.name"
      >
        {{ route.name }}
      </TabButton>
    </TabsHeader>
    <RouterView class="grow" />
  </div>
</template>
