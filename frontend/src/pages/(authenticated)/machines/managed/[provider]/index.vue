<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import Machines from '@/views/omni/Machines/Machines.vue'

definePage({ name: 'MachinesManagedProvider' })
</script>

<template>
  <Machines :provider="$route.params.provider" />
</template>
