<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterMachineStatusSpec, MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusType,
  DefaultNamespace,
  LabelCluster,
  MachineStatusType,
} from '@/api/resources'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useClusterPermissions } from '@/methods/auth'
import { getMachineName } from '@/methods/node'
import { useTitle } from '@/methods/title'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { provideMachineContainers } from '@/views/Containers/lib/machineContainers'
import MachinePageNav, { type MachinePageTab } from '@/views/Machines/components/MachinePageNav.vue'
import NodesHeader from '@/views/Nodes/NodesHeader.vue'

definePage({ name: 'NodeDetails' })

const route = useRoute()

const clusterId = computed(() => route.params.cluster)
const machineId = computed(() => route.params.machine)

const configErrorDismissed = ref(false)

const { canReadMachineConfig, canReadConfigPatches, canReadMachinePendingUpdates } =
  useClusterPermissions(clusterId)

const { data: machine, loading: machineLoading } = useResourceWatch<MachineStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: MachineStatusType,
    namespace: DefaultNamespace,
    id: machineId.value,
  },
}))

const { data: clusterMachine } = useResourceWatch<ClusterMachineStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: ClusterMachineStatusType,
    namespace: DefaultNamespace,
    id: machineId.value,
  },
}))

const isPartOfCluster = computed(
  () => machine.value?.metadata.labels?.[LabelCluster] === clusterId.value,
)

const { data: containers } = provideMachineContainers(
  machineId,
  () => machine.value?.spec.talos_version,
)

// Containers shows only when the machine has some, or when its page is open by URL
const containersTab = computed<MachinePageTab[]>(() =>
  containers.value.length || route.name === 'NodeContainers'
    ? [
        {
          name: 'Containers',
          count: containers.value.length || undefined,
          pages: [{ name: 'Containers', route: 'NodeContainers' }],
        },
      ]
    : [],
)

const tabs = computed<MachinePageTab[]>(() => [
  { name: 'Overview', pages: [{ name: 'Overview', route: 'NodeOverview' }] },
  ...containersTab.value,
  { name: 'Monitor', pages: [{ name: 'Monitor', route: 'NodeMonitor' }] },
  { name: 'Logs', pages: [{ name: 'Logs', route: 'NodeLogs' }] },
  {
    name: 'Configuration',
    pages: [
      { name: 'Current', route: 'NodeConfig', disabled: !canReadMachineConfig.value },
      { name: 'Patches', route: 'NodePatches', disabled: !canReadConfigPatches.value },
      {
        name: 'Pending Updates',
        route: 'NodePendingUpdates',
        disabled: !canReadMachinePendingUpdates.value,
      },
      { name: 'Change History', route: 'NodeConfigDiffs', disabled: !canReadMachineConfig.value },
    ],
  },
  {
    name: 'System',
    pages: [
      { name: 'Disks', route: 'NodeDisks' },
      { name: 'Devices', route: 'NodeDevices' },
      { name: 'Extensions', route: 'NodeExtensions' },
      { name: 'Kernel Args', route: 'NodeKernelArgs' },
      { name: 'KubeSpan', route: 'NodeKubeSpanStatus' },
    ],
  },
])

useTitle(() => getMachineName(machine.value) || machineId.value)
</script>

<template>
  <div v-if="machine && isPartOfCluster" class="flex h-full flex-col pt-base">
    <div class="mb-base flex flex-col gap-compact px-compact md:px-base">
      <NodesHeader :cluster-id="clusterId" :machine-id="machineId" />

      <TAlert
        v-if="clusterMachine?.spec.last_config_error?.trim() && !configErrorDismissed"
        :dismiss="{ name: 'Dismiss', action: () => (configErrorDismissed = true) }"
        title="Configuration Error"
        type="error"
      >
        {{ clusterMachine.spec.last_config_error.trim() }}
      </TAlert>
    </div>

    <MachinePageNav :tabs />

    <div class="min-h-0 grow overflow-y-auto">
      <RouterView v-if="machine && clusterMachine" :machine :cluster-machine />
    </div>
  </div>

  <PageContainer v-else class="font-sm flex-1">
    <TSpinner v-if="machineLoading" class="size-5" />

    <TAlert v-else-if="!machine" title="Machine Not Found" type="error">
      Machine {{ machineId }} does not exist
    </TAlert>

    <TAlert v-else-if="!isPartOfCluster" title="Invalid machine" type="error">
      Machine {{ machineId }} is not a member of cluster {{ clusterId }}
    </TAlert>
  </PageContainer>
</template>
