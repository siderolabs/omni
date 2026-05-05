<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->

<script setup lang="ts">
import { ref } from 'vue'

import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { revokeJoinToken } from '@/methods/auth'
import { showError } from '@/notification'
import JoinTokenWarnings from '@/views/Modals/components/JoinTokenWarnings.vue'

const { token } = defineProps<{
  token: string
}>()

const open = defineModel<boolean>('open', { default: false })

const isReady = ref(false)

const revoke = async () => {
  try {
    await revokeJoinToken(token)
  } catch (e) {
    showError('Failed to Revoke Token', e.message)
  }

  open.value = false
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title="`Revoke the token ${token} ?`"
    action-label="Revoke"
    :disabled="!isReady"
    @confirm="revoke"
  >
    <JoinTokenWarnings :id="token" class="mb-2 flex-1" @ready="isReady = true" />

    <p class="text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
