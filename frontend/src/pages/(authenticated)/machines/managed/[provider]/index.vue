<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { useTitle } from '@/methods/title'
import Machines from '@/views/Machines/Machines.vue'

definePage({ name: 'MachinesManagedProvider' })

const route = useRoute()

useTitle(() => ['Machines', 'Auto-Provisioned', route.params.provider])
</script>

<template>
  <Machines :provider="$route.params.provider" />
</template>
