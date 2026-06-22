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
import { ConfigPatchType, DefaultNamespace } from '@/api/resources'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { showError, showSuccess } from '@/notification'

const { patchId } = defineProps<{
  patchId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const isDeleting = ref(false)

watchEffect(() => {
  if (open.value) return

  isDeleting.value = false
})

async function destroyPatch() {
  try {
    isDeleting.value = true

    await ResourceService.Delete(
      {
        namespace: DefaultNamespace,
        type: ConfigPatchType,
        id: patchId,
      },
      withRuntime(Runtime.Omni),
    )

    showSuccess(`The Config Patch ${patchId} was Destroyed`)
    open.value = false
  } catch (e) {
    showError('Failed to Destroy The Config Patch', e instanceof Error ? e.message : String(e))
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Destroy Config Patch"
    action-label="Destroy"
    :loading="isDeleting"
    @confirm="destroyPatch"
  >
    <template #description>Patch {{ patchId }}</template>

    <ManagedByTemplatesWarning warning-style="popup" />

    <p class="text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
