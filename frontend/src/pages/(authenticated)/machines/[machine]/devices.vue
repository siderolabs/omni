<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useTitle } from '@/methods/title'
import MachineDevices from '@/views/Machines/MachineDevices.vue'

definePage({ name: 'MachineDevices' })

useTitle('Devices')
</script>

<template>
  <MachineDevices :machine-id="$route.params.machine" />
</template>
