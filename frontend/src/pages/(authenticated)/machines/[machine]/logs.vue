<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import MachineLogsContainer from '@/views/omni/Machines/MachineLogsContainer.vue'

definePage({ name: 'MachineLogs' })
</script>

<template>
  <PageContainer>
    <MachineLogsContainer :machine-id="$route.params.machine" class="h-full" />
  </PageContainer>
</template>
