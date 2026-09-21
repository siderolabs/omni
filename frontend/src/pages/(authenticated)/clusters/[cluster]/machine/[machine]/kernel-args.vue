<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useTitle } from '@/methods/title'
import MachineKernelArgs from '@/views/Machines/MachineKernelArgs.vue'

definePage({ name: 'NodeKernelArgs' })

useTitle('Kernel Args')
</script>

<template>
  <MachineKernelArgs :machine-id="$route.params.machine" />
</template>
