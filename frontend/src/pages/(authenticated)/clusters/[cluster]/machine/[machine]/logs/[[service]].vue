<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import MachineLogsContainer from '@/views/Machines/MachineLogsContainer.vue'

definePage({ name: 'NodeLogs' })
</script>

<template>
  <PageContainer class="flex h-full max-h-[calc(100vh-150px)] flex-col overflow-hidden">
    <MachineLogsContainer
      :cluster-id="$route.params.cluster"
      :machine-id="$route.params.machine"
      class="flex grow flex-col"
      :service="$route.params.service"
    />
  </PageContainer>
</template>
