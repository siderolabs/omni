<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { ManagementService } from '@/api/omni/management/management.pb'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { useClusterPermissions } from '@/methods/auth'
import { useMachineName } from '@/methods/node'
import { showError, showSuccess } from '@/notification'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })
const isShuttingDown = ref(false)

const nodeName = useMachineName(
  () => machineId,
  () => ({ skip: !open.value }),
)

const { canRebootMachines } = useClusterPermissions(() => clusterId)

watchEffect(() => {
  if (open.value) return

  isShuttingDown.value = false
})

async function shutdown() {
  isShuttingDown.value = true

  try {
    await ManagementService.MachinePowerOff({ machine_id: machineId })

    showSuccess('Machine Shutdown', `Shutdown request sent for machine ${nodeName.value}.`)
    open.value = false
  } catch (e) {
    showError('Failed to Issue Shutdown', e instanceof Error ? e.message : String(e))
  } finally {
    isShuttingDown.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Shutdown machine"
    action-label="Shutdown"
    :loading="isShuttingDown"
    :disabled="!canRebootMachines"
    @confirm="shutdown"
  >
    <template #description>Node {{ nodeName }}</template>

    <p class="text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
