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
import { InfraProviderNamespace, ProviderType } from '@/api/resources'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { showError, showSuccess } from '@/notification'

const { providerId } = defineProps<{
  providerId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const isDeleting = ref(false)

watchEffect(() => {
  if (open.value) return

  isDeleting.value = false
})

async function deleteProvider() {
  isDeleting.value = true

  try {
    await ResourceService.Delete(
      {
        namespace: InfraProviderNamespace,
        type: ProviderType,
        id: providerId,
      },
      withRuntime(Runtime.Omni),
    )

    showSuccess(`Infra Provider ${providerId} Deleted`)
    open.value = false
  } catch (e) {
    showError('Failed to Delete Infra Provider', e instanceof Error ? e.message : String(e))
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Delete Infra Provider"
    action-label="Delete"
    :loading="isDeleting"
    @confirm="deleteProvider"
  >
    <template #description>Provider {{ providerId }}</template>

    <div class="flex flex-col gap-3">
      <p class="text-xs">Please confirm the action.</p>

      <div class="text-xs text-yellow-y1">
        The infra provider service will no longer be able to connect to Omni. And its service
        account key will be removed.
      </div>
    </div>
  </ConfirmModal>
</template>
