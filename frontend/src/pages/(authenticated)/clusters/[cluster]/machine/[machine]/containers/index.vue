<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useTitle } from '@/methods/title'
import MachineContainers from '@/views/Containers/MachineContainers.vue'

definePage({ name: 'NodeContainers' })

useTitle('Containers')
</script>

<template>
  <MachineContainers :cluster-id="$route.params.cluster" :machine-id="$route.params.machine" />
</template>
