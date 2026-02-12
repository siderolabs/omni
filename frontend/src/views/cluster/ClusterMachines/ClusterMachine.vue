<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusLabelNodeName,
  LabelCluster,
  LabelHostname,
  LabelIsManagedByStaticInfraProvider,
  LabelWorkerRole,
  MachineLocked,
  UpdateLocked,
} from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { updateMachineLock } from '@/methods/machine'
import { showError } from '@/notification'
import NodeContextMenu from '@/views/common/NodeContextMenu.vue'

import ClusterMachinePhase from './ClusterMachinePhase.vue'

const { machine } = defineProps<{
  machine: Resource<ClusterMachineStatusSpec>
  deleteDisabled?: boolean
  hasDiagnosticInfo?: boolean
}>()

const icon = computed(() => {
  if (machine.metadata.labels?.[LabelIsManagedByStaticInfraProvider] !== undefined) {
    return 'server-network'
  }

  return Object.keys(machine.spec.provision_status ?? {}).length ? 'cloud-connection' : 'server'
})

const locked = computed(() => {
  return machine.metadata.annotations?.[MachineLocked] !== undefined
})

const lockable = computed(() => {
  return machine.metadata.labels?.[LabelWorkerRole] !== undefined
})

const router = useRouter()

const hostname = computed(() => {
  const labelHostname = machine.metadata.labels?.[LabelHostname]
  return labelHostname && labelHostname !== '' ? labelHostname : machine.metadata.id
})
const nodeName = computed(
  () => machine.metadata.labels?.[ClusterMachineStatusLabelNodeName] || hostname.value,
)
const clusterName = computed(() => (machine.metadata?.labels || {})[LabelCluster])

const openNodeInfo = async () => {
  router.push({
    name: 'NodeOverview',
    params: { cluster: clusterName.value, machine: machine.metadata.id },
  })
}

const lockedUpdate = computed(() => {
  return machine.metadata.labels?.[UpdateLocked] !== undefined
})

const updateLock = async () => {
  if (!machine.metadata.id) {
    return
  }

  try {
    await updateMachineLock(machine.metadata.id, !locked.value)
  } catch (e) {
    showError('Failed To Update Machine Lock', e.message)
  }
}
</script>

<template>
  <div
    class="col-span-full grid cursor-pointer grid-cols-subgrid p-2 pr-4 text-xs text-naturals-n14 hover:bg-naturals-n3"
    @click="openNodeInfo"
  >
    <div class="col-span-2 ml-6 flex items-center gap-2">
      <TIcon :icon="icon" class="h-4 w-4" />
      <RouterLink
        :to="{
          name: 'NodeOverview',
          params: { cluster: clusterName, machine: machine.metadata.id },
        }"
        class="list-item-link truncate"
      >
        {{ nodeName }}
      </RouterLink>
    </div>

    <div class="col-span-2 flex items-center gap-2">
      <ClusterMachinePhase :machine="machine" />
      <RouterLink
        v-if="lockedUpdate"
        :to="{
          name: 'NodePendingUpdates',
          params: { cluster: clusterName, machine: machine.metadata.id },
        }"
        class="flex items-center gap-1 truncate text-sky-400"
        @click.stop
      >
        <TIcon icon="time" class="h-4 w-4 min-w-max" />
        <div class="flex-1 truncate">Pending Config Update</div>
      </RouterLink>
    </div>

    <div class="flex items-center justify-end">
      <Tooltip
        v-if="hasDiagnosticInfo"
        description="This node has diagnostic warnings. Click to see the details."
      >
        <TIcon icon="warning" class="mx-1.5 h-4 w-4 text-yellow-400" />
      </Tooltip>

      <Tooltip
        v-if="lockable"
        description="Lock machine config. Pause Kubernetes and Talos updates on the machine."
      >
        <IconButton
          :icon="locked ? (lockedUpdate ? 'locked-toggle' : 'locked') : 'unlocked'"
          class="mt-0.5 h-4 w-4"
          @click.stop="updateLock"
        />
      </Tooltip>

      <NodeContextMenu
        :cluster-machine-status="machine"
        :cluster-name="clusterName"
        :delete-disabled="deleteDisabled!"
        @click.stop
      />
    </div>
  </div>
</template>
