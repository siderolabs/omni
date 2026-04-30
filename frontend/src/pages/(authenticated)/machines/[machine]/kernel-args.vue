<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import MachineKernelArgs from '@/views/Machines/MachineKernelArgs.vue'

definePage({ name: 'MachineKernelArgs' })
</script>

<template>
  <MachineKernelArgs :machine="$route.params.machine" />
</template>
