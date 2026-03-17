<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { MachineFilterOption } from '@/methods/machine'
import Machines from '@/views/omni/Machines/Machines.vue'

definePage({ name: 'MachinesManual' })
</script>

<template>
  <Machines :filter="MachineFilterOption.Manual" />
</template>
