<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'
import { useRouter } from 'vue-router'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec, MachineSetStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  LabelCluster,
  LabelHostname,
  LabelIsManagedByStaticInfraProvider,
  LabelWorkerRole,
  MachineLocked,
  UpdateLocked,
} from '@/api/resources'
import { ClusterMachineStatusLabelNodeName } from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { updateMachineLock } from '@/methods/machine'
import { showError } from '@/notification'
import NodeContextMenu from '@/views/common/NodeContextMenu.vue'

import ClusterMachinePhase from './ClusterMachinePhase.vue'

const props = defineProps<{
  machineSet: Resource<MachineSetStatusSpec>
  machine: Resource<ClusterMachineStatusSpec>
  deleteDisabled?: boolean
  hasDiagnosticInfo?: boolean
}>()

const { machine, machineSet } = toRefs(props)

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
  return (
    machineSet?.value.spec?.machine_allocation === undefined &&
    machine?.value.metadata?.labels?.[LabelWorkerRole] !== undefined
  )
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
    class="flex h-3 cursor-pointer items-center gap-1 py-6 pl-3 pr-4 text-xs text-naturals-N14 hover:bg-naturals-N3"
  >
    <div class="pointer-events-none w-5" />
    <div class="-mr-3 grid flex-1 grid-cols-4 items-center" @click="openNodeInfo">
      <div class="col-span-2 flex items-center gap-2">
        <TIcon :icon="icon" class="ml-2 h-4 w-4" />
        <router-link
          :to="{
            name: 'NodeOverview',
            params: { cluster: clusterName, machine: machine.metadata.id },
          }"
          class="list-item-link truncate"
        >
          {{ nodeName }}
        </router-link>
      </div>
      <div class="col-span-2 flex items-center justify-between gap-2 pr-2">
        <div class="flex items-center gap-2">
          <ClusterMachinePhase :machine="machine" />
          <div v-if="lockedUpdate" class="flex items-center gap-1 truncate text-light-blue-400">
            <TIcon icon="time" class="h-4 w-4 min-w-max" />
            <div class="flex-1 truncate">Pending Config Update</div>
          </div>
        </div>
        <div class="flex items-center">
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
        </div>
      </div>
    </div>
    <NodeContextMenu
      :cluster-machine-status="machine"
      :cluster-name="clusterName"
      :delete-disabled="deleteDisabled!"
    />
  </div>
</template>
