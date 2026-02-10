<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { ResourceService } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  DefaultNamespace,
  LabelCluster,
  MachineStatusLabelDisconnected,
  MachineStatusType,
  SiderolinkResourceType,
} from '@/api/resources'
import { ClusterCommandError, clusterDestroy } from '@/methods/cluster'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import ConfirmModal from '@/views/omni/Modals/ConfirmModal.vue'

const { clusterId } = defineProps<{ clusterId: string }>()
const open = defineModel<boolean>('open', { default: false })

const phase = ref('')

const close = () => {
  open.value = false
}

const { data: disconnectedMachines, loading } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  selectors: [MachineStatusLabelDisconnected, `${LabelCluster}=${clusterId}`],
  runtime: Runtime.Omni,
}))

const destroyCluster = async () => {
  destroying.value = true

  // copy machines to avoid updates coming from the watch
  const machines = disconnectedMachines.value.slice(0, disconnectedMachines.value.length)

  for (const machine of machines) {
    phase.value = `Tearing down disconnected machine ${machine.metadata.id}`

    try {
      await ResourceService.Teardown(
        {
          namespace: DefaultNamespace,
          type: SiderolinkResourceType,
          id: machine.metadata.id!,
        },
        withRuntime(Runtime.Omni),
      )
    } catch (e) {
      close()

      if (e.code !== Code.NOT_FOUND) {
        showError('Failed to Destroy the Cluster', e.message)
      }

      return
    }
  }

  try {
    await clusterDestroy(clusterId)
  } catch (e) {
    close()

    if (e instanceof ClusterCommandError) {
      showError(e.errorNotification.title, e.errorNotification.details)

      return
    }

    showError('Failed to Destroy the Cluster', e.message)

    return
  }

  for (const machine of machines) {
    phase.value = `Remove disconnected machine ${machine.metadata.id}`

    try {
      await ResourceService.Delete(
        {
          namespace: DefaultNamespace,
          type: SiderolinkResourceType,
          id: machine.metadata.id!,
        },
        withRuntime(Runtime.Omni),
      )
    } catch (e) {
      close()

      if (e.code !== Code.NOT_FOUND) {
        showError('Failed to Destroy the Cluster', e.message)
      }

      return
    }
  }

  destroying.value = false

  close()

  showSuccess(`The Cluster ${clusterId} is Tearing Down`)
}

const destroying = ref(false)
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title="`Destroy the Cluster ${clusterId} ?`"
    action-label="Destroy"
    :loading="destroying || loading"
    @confirm="destroyCluster"
  >
    <ManagedByTemplatesWarning warning-style="popup" />
    <p v-if="destroying" class="text-xs">{{ phase }}...</p>
    <p v-else-if="loading" class="text-xs">Checking the cluster status...</p>
    <div v-else-if="disconnectedMachines.length > 0" class="text-xs">
      <p class="py-2 text-primary-p3">
        Cluster
        <code>{{ clusterId }}</code>
        has {{ disconnectedMachines.length }} disconnected
        {{ pluralize('machine', disconnectedMachines.length, false) }}. Destroying the cluster now
        will also destroy disconnected machines.
      </p>
      <p class="py-2 font-bold text-primary-p3">
        These machines will need to be wiped and reinstalled to be used with Omni again. If the
        machines can be recovered, you may wish to recover them before destroying the cluster, to
        allow a graceful reset of the machines.
      </p>
    </div>
    <p v-else class="text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
