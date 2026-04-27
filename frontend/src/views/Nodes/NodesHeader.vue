<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineSetNodeSpec, MachineSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  LabelIsManagedByStaticInfraProvider,
  MachineSetNodeType,
  MachineType,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import NodePowerOn from '@/views/Modals/NodePowerOn.vue'
import NodesBreadcrumbs from '@/views/Nodes/NodesBreadcrumbs.vue'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const route = useRoute()
const router = useRouter()
const powerOnModalOpen = ref(false)

const shutdownNode = () => {
  router.push({
    query: {
      modal: 'shutdown',
      machine: machineId,
      ...route.query,
    },
  })
}

const powerOnNode = () => {
  powerOnModalOpen.value = true
}

const rebootNode = () => {
  router.push({
    query: {
      modal: 'reboot',
      machine: machineId,
      ...route.query,
    },
  })
}

const { data: machineSetNode, loading } = useResourceWatch<MachineSetNodeSpec>(() => ({
  resource: {
    type: MachineSetNodeType,
    id: machineId,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const { data: machine } = useResourceWatch<MachineSpec>(() => ({
  resource: {
    type: MachineType,
    id: machineId,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const isManagedByStaticInfraProvider = computed(
  () => machine.value?.metadata.labels?.[LabelIsManagedByStaticInfraProvider] !== undefined,
)

const destroyNode = async () => {
  router.push({
    query: {
      modal: 'nodeDestroy',
      machine: machineId,
      cluster: clusterId,
    },
  })
}

const restoreNode = async () => {
  router.push({
    query: {
      modal: 'nodeDestroyCancel',
      machine: machineId,
      cluster: clusterId,
    },
  })
}

const { canRebootMachines, canRemoveMachines, canAddClusterMachines } = useClusterPermissions(
  computed(() => clusterId),
)
</script>

<template>
  <div class="mb-7 flex flex-wrap items-start justify-between">
    <NodesBreadcrumbs :cluster-id :machine-id />

    <div class="flex">
      <Tooltip
        :description="
          isManagedByStaticInfraProvider
            ? undefined
            : 'Power on is only available for machines managed by a static infra provider. For other machines, power on the machine manually.'
        "
      >
        <TButton
          class="header-button"
          icon="power"
          icon-position="left"
          variant="secondary"
          :disabled="!canRebootMachines || !isManagedByStaticInfraProvider"
          @click="powerOnNode"
        >
          Power On
        </TButton>
      </Tooltip>
      <TButton
        class="header-button"
        icon="power"
        icon-position="left"
        variant="secondary"
        :disabled="!canRebootMachines"
        @click="shutdownNode"
      >
        Shutdown
      </TButton>
      <TButton
        class="header-button"
        icon="reboot"
        icon-position="left"
        variant="secondary"
        :disabled="!canRebootMachines"
        @click="rebootNode"
      >
        Reboot
      </TButton>
      <TButton
        v-if="machineSetNode"
        class="header-button delete-button"
        icon="delete"
        icon-position="left"
        variant="secondary"
        :disabled="!canRemoveMachines"
        @click="destroyNode"
      >
        Destroy
      </TButton>
      <TButton
        v-else-if="!loading"
        class="header-button"
        icon="rollback"
        icon-position="left"
        variant="secondary"
        :disabled="!canAddClusterMachines"
        @click="restoreNode"
      >
        Cancel Destroy
      </TButton>
    </div>

    <NodePowerOn v-model:open="powerOnModalOpen" :cluster-id="clusterId" :machine-id="machineId" />
  </div>
</template>

<style scoped>
@reference "../../index.css";

.header-button:first-child {
  border-radius: 4px 0 0 4px;
}

.header-button:not(:last-child) {
  border-right: none;
}

.header-button:last-child {
  border-radius: 0 4px 4px 0;
}

.header-button:not(:first-child):not(:last-child) {
  border-radius: 0;
}

.delete-button {
  @apply text-red-r1 hover:text-red-r1 active:text-red-r1;
}
</style>
