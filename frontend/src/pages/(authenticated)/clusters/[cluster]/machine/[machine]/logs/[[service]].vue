<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { TalosContainersLogPrefix } from '@/api/resources'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import { useTitle } from '@/methods/title'
import { injectMachineContainers } from '@/views/Containers/lib/machineContainers'
import MachineLogsContainer from '@/views/Machines/MachineLogsContainer.vue'

definePage({ name: 'NodeLogs' })

const route = useRoute()

// A Talos container's log goes by its name, without the prefix Talos registers it under
useTitle(() => {
  const service = route.params.service

  return [
    'Logs',
    service?.startsWith(TalosContainersLogPrefix)
      ? service.slice(TalosContainersLogPrefix.length)
      : service,
  ]
})

// The machine page watches its containers for the Containers tab; offer their logs too
const { data: containers } = injectMachineContainers()
</script>

<template>
  <PageContainer class="flex h-full max-h-[calc(100vh-150px)] flex-col overflow-hidden">
    <MachineLogsContainer
      :cluster-id="$route.params.cluster"
      :machine-id="$route.params.machine"
      class="flex grow flex-col"
      :service="$route.params.service"
      :containers
    />
  </PageContainer>
</template>
