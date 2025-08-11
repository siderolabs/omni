<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import Watch from '@/api/watch'
import Patches from '@/views/cluster/Config/Patches.vue'

const machine = ref<Resource>()
const route = useRoute()

const machineWatch = new Watch(machine)

machineWatch.setup(
  computed(() => {
    return {
      resource: {
        id: route.params.machine as string,
        type: MachineStatusType,
        namespace: DefaultNamespace,
      },
      runtime: Runtime.Omni,
    }
  }),
)
</script>

<template>
  <Patches v-if="machine" :machine="machine" />
</template>
