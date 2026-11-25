<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import type { Resource } from '@/api/grpc'
import {
  type ClusterMachineStatusSpec,
  type DryRunStatusSpec,
  DryRunStatusSpecOutcome,
  type MachineOperationStatusSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusLabelNodeName,
  LabelCluster,
  LabelHostname,
  LabelIsManagedByStaticInfraProvider,
  LabelWorkerRole,
  MachineLocked,
  UpdateLocked,
} from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { updateMachineLock } from '@/methods/machine'
import { showError } from '@/notification'
import NodeContextMenu from '@/views/common/NodeContextMenu.vue'

import ClusterMachineDryRunResult from './ClusterMachineDryRunResult.vue'
import ClusterMachinePhase from './ClusterMachinePhase.vue'
import DryRunUnlockModal from './DryRunUnlockModal.vue'

const { machine, dryRun } = defineProps<{
  machine: Resource<ClusterMachineStatusSpec>
  operation?: MachineOperationStatusSpec
  dryRun?: DryRunStatusSpec
  clusterLocked?: boolean
  lockedOnItsOwn?: boolean
  removeDisabled?: boolean
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

const pendingUpdatesRoute = computed(() => ({
  name: 'NodePendingUpdates' as const,
  params: { cluster: clusterName.value, machine: machine.metadata.id! },
}))

const openNodeInfo = async () => {
  router.push({
    name: 'NodeOverview',
    params: { cluster: clusterName.value, machine: machine.metadata.id! },
  })
}

const lockedUpdate = computed(() => {
  return machine.metadata.labels?.[UpdateLocked] !== undefined
})

const lockIcon = computed(() => {
  if (!locked.value) {
    return 'unlocked'
  }

  return lockedUpdate.value ? 'locked-toggle' : 'locked'
})

const dryRunFailed = computed(() => dryRun?.outcome === DryRunStatusSpecOutcome.FAILED)

const unlockModalOpen = ref(false)
const updatingLock = ref(false)

const updateLock = async (lock: boolean) => {
  if (!machine.metadata.id) {
    return
  }

  try {
    updatingLock.value = true

    await updateMachineLock(machine.metadata.id, lock)

    unlockModalOpen.value = false
  } catch (e) {
    showError('Failed To Update Machine Lock', e.message)
  } finally {
    updatingLock.value = false
  }
}

const onLockClick = () => {
  if (locked.value && dryRunFailed.value) {
    unlockModalOpen.value = true

    return
  }

  updateLock(!locked.value)
}
</script>

<template>
  <div
    class="col-span-full grid cursor-pointer grid-cols-subgrid p-tight pr-compact text-xs text-content-emphasis hover:bg-surface-raised"
    @click="openNodeInfo"
  >
    <div class="col-span-2 ml-base flex items-center gap-tight">
      <TIcon :icon="icon" class="size-4 shrink-0" />
      <RouterLink
        :to="{
          name: 'NodeOverview',
          params: { cluster: clusterName, machine: machine.metadata.id! },
        }"
        class="list-item-link truncate"
        @click.stop
      >
        {{ nodeName }}
      </RouterLink>
    </div>

    <div class="col-span-2 flex items-center gap-tight">
      <ClusterMachinePhase :machine="machine" :operation="operation" />
      <RouterLink
        v-if="lockedUpdate"
        :to="pendingUpdatesRoute"
        class="flex items-center gap-micro truncate text-status-info-text hover:text-content-emphasis"
        @click.stop
      >
        <TIcon icon="time" class="size-4 min-w-max shrink-0" />
        <div class="flex-1 truncate">Pending Config Update</div>
      </RouterLink>
      <ClusterMachineDryRunResult
        v-if="dryRun && (lockedUpdate || dryRunFailed)"
        :dry-run="dryRun"
        :standalone="!lockedUpdate"
        :to="pendingUpdatesRoute"
      />
    </div>

    <div class="flex items-center justify-end">
      <Tooltip
        v-if="machine.spec.last_config_error"
        :description="machine.spec.last_config_error.trim()"
      >
        <TIcon icon="x-circle" class="mx-micro size-4 shrink-0 text-status-danger-default" />
      </Tooltip>

      <Tooltip
        v-if="hasDiagnosticInfo"
        description="This node has diagnostic warnings. Click to see the details."
      >
        <TIcon
          icon="exclamation-triangle"
          class="mx-micro size-4 shrink-0 text-status-warning-default"
        />
      </Tooltip>

      <Tooltip
        v-if="lockable && (!clusterLocked || lockedOnItsOwn)"
        :description="
          clusterLocked
            ? 'The machine is locked on its own. Unlocking the cluster does not unlock it.'
            : 'Lock machine config. Pause Kubernetes and Talos updates on the machine.'
        "
      >
        <!-- a disabled button gets no pointer events, so the wrapper opens the tooltip -->
        <span
          v-if="clusterLocked"
          class="flex shrink-0 rounded"
          tabindex="0"
          role="img"
          aria-label="Locked on its own"
          @click.stop
        >
          <IconButton :icon="lockIcon" disabled data-testid="cluster-machine-lock" />
        </span>
        <IconButton
          v-else
          class="shrink-0"
          :icon="lockIcon"
          data-testid="cluster-machine-lock"
          @click.stop="onLockClick"
        />
      </Tooltip>

      <NodeContextMenu
        class="shrink-0"
        :cluster-machine-status="machine"
        :cluster-name="clusterName"
        :remove-disabled="removeDisabled!"
      />
    </div>

    <DryRunUnlockModal
      v-model:open="unlockModalOpen"
      :title="`Unlock machine ${nodeName}?`"
      action-label="Unlock machine"
      :cluster-id="clusterName ?? ''"
      :failures="[{ id: machine.metadata.id!, name: nodeName ?? '', error: dryRun?.error }]"
      :loading="updatingLock"
      @confirm="updateLock(false)"
    >
      Its pending config would fail. Unlocking applies it anyway.
    </DryRunUnlockModal>
  </div>
</template>
