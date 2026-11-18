<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MaintenanceConfigStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MaintenanceConfigStatusType } from '@/api/resources'
import PageHeader from '@/components/PageHeader.vue'
import TAlert from '@/components/TAlert.vue'
import { usePermissions } from '@/methods/auth'
import { useMachineName } from '@/methods/node'
import { useTitle } from '@/methods/title'
import { useResourceWatch } from '@/methods/useResourceWatch'
import MachinePageNav, { type MachinePageTab } from '@/views/Machines/components/MachinePageNav.vue'

definePage({
  name: 'Machine',
  redirect: {
    name: 'MachineLogs',
  },
})

const { canReadMachineConfigPatches } = usePermissions()

const route = useRoute()

// The same tabs as a machine in a cluster, less the pages that need one
const tabs = computed<MachinePageTab[]>(() => [
  { name: 'Logs', pages: [{ name: 'Logs', route: 'MachineLogs' }] },
  {
    name: 'Configuration',
    pages: [
      {
        name: 'Patches',
        route: 'MachineConfigPatches',
        alsoOn: ['MachinePatchEdit'],
        disabled: !canReadMachineConfigPatches.value,
      },
    ],
  },
  {
    name: 'System',
    pages: [
      { name: 'Disks', route: 'MachineDisks' },
      { name: 'Devices', route: 'MachineDevices' },
      { name: 'Extensions', route: 'MachineExtensions' },
      { name: 'Kernel Args', route: 'MachineKernelArgs' },
    ],
  },
])

const machineName = useMachineName(() => route.params.machine)

const { data: maintenanceConfigStatus } = useResourceWatch<MaintenanceConfigStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: MaintenanceConfigStatusType,
    id: route.params.machine,
  },
}))

useTitle(() => ['Machines', machineName.value])
</script>

<template>
  <div class="flex h-full flex-col pt-base">
    <PageHeader :title="machineName" class="px-compact md:px-base" />

    <TAlert
      v-if="maintenanceConfigStatus?.spec.last_config_error"
      title="Config Error"
      type="error"
      class="mx-compact mb-compact md:mx-base"
    >
      {{ maintenanceConfigStatus.spec.last_config_error.trim() }}
    </TAlert>

    <MachinePageNav :tabs />

    <div class="min-h-0 grow overflow-y-auto">
      <RouterView class="h-full" />
    </div>
  </div>
</template>
