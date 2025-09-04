<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { withRuntime } from '@/api/options'
import {
  ClusterMachineIdentityType,
  DefaultNamespace,
  TalosRuntimeNamespace,
  TalosServiceType,
} from '@/api/resources'
import Watch from '@/api/watch'
import type { SideBarItem } from '@/components/SideBar/TSideBarList.vue'
import TSidebarList from '@/components/SideBar/TSideBarList.vue'
import { getContext } from '@/context'

const node = ref()
const context = getContext()
const route = useRoute()

const services = ref<Resource[]>([])

const serviceWatch = new Watch(services)

serviceWatch.setup(
  computed(() => {
    return {
      resource: {
        type: TalosServiceType,
        namespace: TalosRuntimeNamespace,
      },
      runtime: Runtime.Talos,
      context: context,
    }
  }),
)

const items = computed(() => {
  const res: SideBarItem[] = []

  for (const service of ['controller-runtime'].concat(
    services.value.map((item) => item.metadata.id!),
  )) {
    res.push({
      name: service,
      route: {
        name: 'NodeLogs',
        params: {
          machine: route.params.machine as string,
          service: service,
        },
      },
    })
  }

  return res
})

onMounted(async () => {
  const nodename: Resource<{ nodename: string }> = await ResourceService.Get(
    {
      type: ClusterMachineIdentityType,
      id: route.params.machine as string,
      namespace: DefaultNamespace,
    },
    withRuntime(Runtime.Omni),
  )

  node.value = nodename.spec.nodename
})
</script>

<template>
  <div>
    <p class="mt-5 mb-2 px-6 text-xs text-naturals-n8">Node</p>
    <p class="truncate px-6 text-xs text-naturals-n13">{{ node }}</p>
    <TSidebarList :items="items" />
  </div>
</template>
