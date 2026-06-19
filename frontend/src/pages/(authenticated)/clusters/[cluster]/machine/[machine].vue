<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  LabelCluster,
  MachineStatusType,
  TalosConfigNamespace,
  TalosKubespanConfigID,
  TalosKubeSpanConfigType,
} from '@/api/resources'
import type { ConfigSpec } from '@/api/talos/kubespan.pb'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TabButton from '@/components/Tabs/TabButton.vue'
import TabContent from '@/components/Tabs/TabContent.vue'
import Tabs from '@/components/Tabs/Tabs.vue'
import TAlert from '@/components/TAlert.vue'
import { useClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import NodesHeader from '@/views/Nodes/NodesHeader.vue'

definePage({ name: 'NodeDetails' })

const route = useRoute()

const clusterId = computed(() => route.params.cluster)
const machineId = computed(() => route.params.machine)

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

const isPartOfCluster = computed(
  () => machine.value?.metadata.labels?.[LabelCluster] === clusterId.value,
)

const { data: kubeSpanConfig } = useResourceWatch<ConfigSpec>(() => ({
  runtime: Runtime.Talos,
  resource: {
    namespace: TalosConfigNamespace,
    type: TalosKubeSpanConfigType,
    id: TalosKubespanConfigID,
  },
  context: {
    cluster: clusterId.value,
    machine: machineId.value,
  },
}))

const routes = computed(() => {
  return [
    {
      name: 'Overview',
      to: { name: 'NodeOverview', params: { machine: machineId.value } },
    },
    {
      name: 'Monitor',
      to: { name: 'NodeMonitor', params: { machine: machineId.value } },
    },
    {
      name: 'Logs',
      to: { name: 'NodeLogs', params: { machine: machineId.value } },
    },
    {
      name: 'Config',
      to: { name: 'NodeConfig', params: { machine: machineId.value } },
      disabled: !canReadMachineConfig.value,
    },
    {
      name: 'Pending Updates',
      to: { name: 'NodePendingUpdates', params: { machine: machineId.value } },
      disabled: !canReadMachinePendingUpdates.value,
    },
    {
      name: 'Config History',
      to: { name: 'NodeConfigDiffs', params: { machine: machineId.value } },
      disabled: !canReadMachineConfig.value,
    },
    {
      name: 'Patches',
      to: { name: 'NodePatches', params: { machine: machineId.value } },
      disabled: !canReadConfigPatches.value,
    },
    {
      name: 'KubeSpan',
      to: { name: 'NodeKubeSpanStatus', params: { machine: machineId.value } },
      disabled: !kubeSpanConfig.value?.spec.enabled,
    },
    {
      name: 'Disks',
      to: { name: 'NodeDisks', params: { machine: machineId.value } },
    },
    {
      name: 'Devices',
      to: { name: 'NodeDevices', params: { machine: machineId.value } },
    },
    {
      name: 'Extensions',
      to: { name: 'NodeExtensions', params: { machine: machineId.value } },
    },
    {
      name: 'Kernel Args',
      to: { name: 'NodeKernelArgs' },
    },
  ]
})
</script>

<template>
  <div v-if="machine && isPartOfCluster" class="flex h-full flex-col pt-6">
    <NodesHeader :cluster-id="clusterId" :machine-id="machineId" class="px-4 md:px-6" />

    <Tabs
      :model-value="$route.name?.toString()"
      class="grow overflow-y-hidden"
      tabs-list-class="px-4 md:px-6"
    >
      <template #triggers>
        <TabButton
          v-for="{ name, to, disabled } in routes"
          :key="name"
          :as="RouterLink"
          :value="to.name"
          :to
          :disabled
        >
          {{ name }}
        </TabButton>
      </template>

      <template #contents>
        <TabContent
          v-for="{ name, to } in routes"
          :key="name"
          class="grow overflow-y-auto"
          :value="to.name"
        >
          <RouterView />
        </TabContent>
      </template>
    </Tabs>
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
