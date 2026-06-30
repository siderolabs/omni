<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import Modal from '@/components/Modals/Modal.vue'
import TInput from '@/components/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { renewServiceAccount } from '@/methods/user'
import { showError, showSuccess } from '@/notification'
import ServiceAccountKey from '@/views/Users/components/ServiceAccountKey.vue'

const { identity } = defineProps<{
  identity: string
}>()

const open = defineModel<boolean>('open', { default: false })

const { canManageUsers } = usePermissions()

const isRenewing = ref(false)
const key = ref('')
const expiration = ref(365)

watchEffect(() => {
  if (open.value) return

  isRenewing.value = false
  key.value = ''
  expiration.value = 365
})

const handleRenew = async () => {
  isRenewing.value = true

  try {
    key.value = await renewServiceAccount(identity, expiration.value)

    showSuccess('Service Account Key Was Renewed')
  } catch (e) {
    showError('Failed to Renew Service Account', e instanceof Error ? e.message : String(e))
  } finally {
    isRenewing.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    :title="`Renew the Key for the Account ${identity}`"
    :cancel-label="key ? 'Close' : 'Cancel'"
    :action-label="key ? undefined : 'Generate New Key'"
    :action-disabled="!canManageUsers && authType !== AuthType.SAML"
    :loading="isRenewing"
    content-class="max-w-xl"
    @confirm="handleRenew"
  >
    <ServiceAccountKey v-if="key" :secret-key="key" />

    <TInput
      v-else
      v-model="expiration"
      title="Expiration Days"
      type="number"
      :min="1"
      class="w-full"
    />
  </Modal>
</template>
