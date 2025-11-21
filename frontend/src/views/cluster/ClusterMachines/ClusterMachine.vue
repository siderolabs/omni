<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'
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

const props = defineProps<{
  machine: Resource<ClusterMachineStatusSpec>
  deleteDisabled?: boolean
  hasDiagnosticInfo?: boolean
}>()

const { machine } = toRefs(props)

const icon = computed(() => {
  if (machine.value.metadata.labels?.[LabelIsManagedByStaticInfraProvider] !== undefined) {
    return 'server-network'
  }

  return Object.keys(machine.value.spec.provision_status ?? {}).length
    ? 'cloud-connection'
    : 'server'
})

const locked = computed(() => {
  return machine.value?.metadata?.annotations?.[MachineLocked] !== undefined
})

const lockable = computed(() => {
  return machine?.value.metadata?.labels?.[LabelWorkerRole] !== undefined
})

const router = useRouter()

const hostname = computed(() => {
  const labelHostname = props.machine?.metadata?.labels?.[LabelHostname]
  return labelHostname && labelHostname !== '' ? labelHostname : props.machine?.metadata.id
})
const nodeName = computed(
  () =>
    (props.machine?.metadata?.labels || {})[ClusterMachineStatusLabelNodeName] || hostname.value,
)
const clusterName = computed(() => (props.machine?.metadata?.labels || {})[LabelCluster])

const openNodeInfo = async () => {
  router.push({
    name: 'NodeOverview',
    params: { cluster: clusterName.value, machine: props.machine.metadata.id },
  })
}

const lockedUpdate = computed(() => {
  return machine.value.metadata.labels?.[UpdateLocked] !== undefined
})

const updateLock = async () => {
  if (!props.machine.metadata.id) {
    return
  }

  try {
    await updateMachineLock(props.machine.metadata.id, !locked.value)
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
      <div v-if="lockedUpdate" class="flex items-center gap-1 truncate text-sky-400">
        <TIcon icon="time" class="h-4 w-4 min-w-max" />
        <div class="flex-1 truncate">Pending Config Update</div>
      </div>
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
