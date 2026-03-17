<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineSetNodeSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineSetNodeType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TBreadcrumbs from '@/components/TBreadcrumbs.vue'
import { setupClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { nodeName, clusterId, machineId } = defineProps<{
  nodeName: string
  clusterId: string
  machineId: string
}>()

const route = useRoute()
const router = useRouter()

const shutdownNode = () => {
  router.push({
    query: {
      modal: 'shutdown',
      machine: machineId,
      ...route.query,
    },
  })
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

const { canRebootMachines, canRemoveMachines, canAddClusterMachines } = setupClusterPermissions(
  computed(() => clusterId),
)
</script>

<template>
  <div class="mb-7 flex flex-wrap items-start justify-between">
    <TBreadcrumbs :node-name="nodeName" />
    <div class="flex">
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
  </div>
</template>

<style scoped>
@reference "../../../index.css";

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
