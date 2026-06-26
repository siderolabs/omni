<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, MachineClassType } from '@/api/resources'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { showError, showSuccess } from '@/notification'

const { machineClassId } = defineProps<{
  machineClassId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const isDeleting = ref(false)

watchEffect(() => {
  if (open.value) return

  isDeleting.value = false
})

const destroy = async () => {
  isDeleting.value = true

  try {
    await ResourceService.Delete(
      {
        id: machineClassId,
        namespace: DefaultNamespace,
        type: MachineClassType,
      },
      withRuntime(Runtime.Omni),
    )

    showSuccess(`The Machine Class ${machineClassId} was Destroyed`)

    open.value = false
  } catch (e) {
    showError('Failed to remove the machine class', e instanceof Error ? e.message : String(e))
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Destroy Machine Class"
    action-label="Destroy"
    :loading="isDeleting"
    @confirm="destroy"
  >
    <template #description>Machine Class {{ machineClassId }}</template>

    <p class="py-2 text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
