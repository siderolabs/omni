<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import MachineExtensions from '@/views/Machines/MachineExtensions.vue'

definePage({ name: 'NodeExtensions' })
</script>

<template>
  <MachineExtensions :cluster-id="$route.params.cluster" :machine-id="$route.params.machine" />
</template>
