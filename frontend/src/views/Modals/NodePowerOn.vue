<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  DefaultNamespace,
} from '@/api/resources'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { useClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })
const loading = ref(false)

const { data: clusterMachineStatus } = useResourceWatch<ClusterMachineStatusSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineStatusType,
    id: machineId,
  },
  runtime: Runtime.Omni,
}))

const nodeName = computed(
  () =>
    clusterMachineStatus.value?.metadata.labels?.[ClusterMachineStatusLabelNodeName] ?? machineId,
)

const close = () => {
  open.value = false
}

const { canRebootMachines } = useClusterPermissions(computed(() => clusterId))

const powerOn = async () => {
  loading.value = true

  try {
    await ManagementService.MachinePowerOn({ machine_id: machineId })
  } catch (e) {
    close()

    showError('Failed to Issue Power On', e instanceof Error ? e.message : String(e))

    return
  } finally {
    loading.value = false
  }

  close()

  showSuccess('Machine Power On', `Power on request sent for machine ${nodeName.value}.`)
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title="`Power on the Machine ${nodeName} ?`"
    action-label="Power On"
    :disabled="!canRebootMachines"
    :loading="loading"
    @confirm="powerOn"
  >
    <p class="text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
