<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->

<script setup lang="ts">
import { ref } from 'vue'

import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { deleteJoinToken } from '@/methods/auth'
import { showError } from '@/notification'
import JoinTokenWarnings from '@/views/Modals/components/JoinTokenWarnings.vue'

const { token } = defineProps<{
  token: string
}>()

const open = defineModel<boolean>('open', { default: false })

const isReady = ref(false)

const deleteToken = async () => {
  try {
    await deleteJoinToken(token)
  } catch (e) {
    showError('Failed to Delete Token', e.message)
  }

  open.value = false
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title="`Delete the token ${token} ?`"
    action-label="Delete"
    :disabled="!isReady"
    @confirm="deleteToken"
  >
    <JoinTokenWarnings :id="token" class="mb-2 flex-1" @ready="isReady = true" />

    <p class="text-xs text-primary-p2">
      This action CANNOT be undone. This will permanently delete the Join Token.
    </p>
  </ConfirmModal>
</template>
