<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { isAfter, isFuture, subDays } from 'date-fns'
import { computed, ref, watchEffect } from 'vue'

import type { ServiceAccountStatusSpecPgpPublicKey } from '@/api/omni/specs/auth.pb'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import TAlert from '@/components/TAlert.vue'
import { relativeISO } from '@/methods/time'
import { revokeServiceAccountKey } from '@/methods/user'
import { showError, showSuccess } from '@/notification'

const {
  identity,
  publicKeyId,
  keys = [],
} = defineProps<{
  identity: string
  publicKeyId: string
  keys?: ServiceAccountStatusSpecPgpPublicKey[]
}>()

const open = defineModel<boolean>('open', { default: false })

const isRevoking = ref(false)

const publicKey = computed(() => keys.find((k) => k.id === publicKeyId))

const recentlyUsed = computed(
  () => !!publicKey.value?.last_used && isAfter(publicKey.value.last_used, subDays(new Date(), 1)),
)

const isLastValidKey = computed(() => {
  const validKeys = keys.filter((k) => !k.expiration || isFuture(k.expiration))

  return validKeys.length === 1 && validKeys[0].id === publicKeyId
})

watchEffect(() => {
  if (open.value) return

  isRevoking.value = false
})

const revoke = async () => {
  try {
    isRevoking.value = true

    await revokeServiceAccountKey(identity, publicKeyId)

    showSuccess('Revoked Service Account Key', publicKeyId)

    open.value = false
  } catch (e) {
    showError('Failed to Revoke Service Account Key', e instanceof Error ? e.message : String(e))
  } finally {
    isRevoking.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Revoke Service Account Key"
    action-label="Revoke key"
    :loading="isRevoking"
    @confirm="revoke"
  >
    <template #description>{{ identity }}</template>

    <div v-if="recentlyUsed || isLastValidKey" class="mb-4 flex max-w-md flex-col gap-2">
      <TAlert v-if="recentlyUsed" type="warn" title="Key used recently">
        This key was last used {{ relativeISO(publicKey!.last_used!) }}.
      </TAlert>

      <TAlert v-if="isLastValidKey" type="warn" title="Last valid key">
        All other keys of this service account have expired. Revoking this key leaves it with no way
        to authenticate.
      </TAlert>
    </div>

    <p class="text-xs">
      Clients using the key
      <code class="font-mono text-content-emphasis">{{ publicKeyId }}</code>
      will immediately lose access. Please confirm the action.
    </p>
  </ConfirmModal>
</template>
