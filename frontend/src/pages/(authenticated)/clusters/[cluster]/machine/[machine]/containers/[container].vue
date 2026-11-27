<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { useTitle } from '@/methods/title'
import MachineContainer from '@/views/Containers/MachineContainer.vue'

definePage({ name: 'NodeContainer' })

const route = useRoute('NodeContainer')

useTitle(() => route.params.container)
</script>

<template>
  <MachineContainer
    :cluster-id="route.params.cluster"
    :machine-id="route.params.machine"
    :container-id="route.params.container"
  />
</template>
