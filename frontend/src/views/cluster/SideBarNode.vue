<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <div class="border-b border-naturals-N4">
      <cluster-side-bar />
    </div>
    <p class="text-xs text-naturals-N8 mt-5 mb-2 px-6">Node</p>
    <p class="text-xs text-naturals-N13 px-6 truncate">{{ node }}</p>
    <t-sidebar-list :items="items" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { getContext } from '@/context'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { Runtime } from '@/api/common/omni.pb'
import {
  ClusterMachineIdentityType,
  DefaultNamespace,
  TalosRuntimeNamespace,
  TalosServiceType,
} from '@/api/resources'
import { withRuntime } from '@/api/options'

import type { SideBarItem } from '@/components/SideBar/TSideBarList.vue'
import TSidebarList from '@/components/SideBar/TSideBarList.vue'
import ClusterSideBar from '@/views/cluster/SideBar.vue'
import Watch from '@/api/watch'

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
