<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onBeforeMount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import MachineLogsContainer from '@/views/omni/Machines/MachineLogsContainer.vue'

const route = useRoute()

const getMachineName = async () => {
  const res: Resource<MachineStatusSpec> = await ResourceService.Get(
    {
      namespace: DefaultNamespace,
      type: MachineStatusType,
      id: route.params.machine! as string,
    },
    withRuntime(Runtime.Omni),
  )

  machine.value = res.spec.network?.hostname || res.metadata.id!
}

const machine = ref(route.params.machine)

onBeforeMount(getMachineName)
watch(() => route.params, getMachineName)
</script>

<template>
  <div class="flex flex-col gap-2">
    <MachineLogsContainer class="flex-1" />
  </div>
</template>
