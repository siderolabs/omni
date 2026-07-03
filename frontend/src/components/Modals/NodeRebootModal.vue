<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { withContext, withRuntime } from '@/api/options'
import { MachineService } from '@/api/talos/machine/machine.pb'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { useClusterPermissions } from '@/methods/auth'
import { useMachineName } from '@/methods/node'
import { showError, showSuccess } from '@/notification'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const nodeName = useMachineName(
  () => machineId,
  () => ({ skip: !open.value }),
)

const { canRebootMachines } = useClusterPermissions(() => clusterId)

const isRebooting = ref(false)

watchEffect(() => {
  if (open.value) return

  isRebooting.value = false
})

async function reboot() {
  isRebooting.value = true

  try {
    const res = await MachineService.Reboot(
      {},
      withRuntime(Runtime.Talos),
      withContext({
        cluster: clusterId,
        node: machineId,
      }),
    )

    const error = res.messages
      ?.filter((m) => m.metadata?.error)
      .map((m) => `${m.metadata!.hostname || nodeName.value} ${m.metadata!.error}`)
      .join(', ')

    if (error) throw new Error(error)

    open.value = false
    showSuccess('Machine Reboot', `Machine ${nodeName.value} is rebooting now.`)
  } catch (e) {
    showError('Failed to Issue Reboot', e instanceof Error ? e.message : String(e))

    return
  } finally {
    isRebooting.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Reboot machine"
    action-label="Reboot"
    :loading="isRebooting"
    :disabled="!canRebootMachines"
    @confirm="reboot"
  >
    <template #description>Node {{ nodeName }}</template>

    <p class="text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
