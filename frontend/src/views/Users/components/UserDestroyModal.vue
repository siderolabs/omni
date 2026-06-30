<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'

import { ManagementService } from '@/api/omni/management/management.pb'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { showError, showSuccess } from '@/notification'

const { identity, isServiceAccount } = defineProps<{
  identity: string
  isServiceAccount?: boolean
}>()

const open = defineModel<boolean>('open', { default: false })

const object = computed(() => (isServiceAccount ? 'Service Account' : 'User'))

const isDestroying = ref(false)

watchEffect(() => {
  if (open.value) return

  isDestroying.value = false
})

const destroy = async () => {
  try {
    isDestroying.value = true

    if (isServiceAccount) {
      const [name, domain] = identity.split('@')

      await ManagementService.DestroyServiceAccount({
        name: domain.includes('infra-provider') ? `infra-provider:${name}` : name,
      })
    } else {
      await ManagementService.DestroyUser({
        email: identity,
      })
    }

    showSuccess(`Deleted ${object.value} ${identity}`)

    open.value = false
  } catch (e) {
    showError(
      `Failed to Delete ${object.value} ${identity}`,
      e instanceof Error ? e.message : String(e),
    )
  } finally {
    isDestroying.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title="`Delete the ${object} ${identity} ?`"
    action-label="Delete"
    :loading="isDestroying"
    @confirm="destroy"
  >
    <p class="text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
