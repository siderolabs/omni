<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { RoleAdmin, RoleNone, RoleOperator, RoleReader } from '@/api/resources'
import Modal from '@/components/Modals/Modal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TInput from '@/components/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { createServiceAccount } from '@/methods/user'
import { showError, showSuccess } from '@/notification'
import ServiceAccountKey from '@/views/Users/components/ServiceAccountKey.vue'

const { name: propName, role: propRole } = defineProps<{
  name?: string
  role?: string
}>()

const open = defineModel<boolean>('open', { default: false })

const { canManageUsers } = usePermissions()

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const isCreating = ref(false)
const name = ref('')
const expiration = ref(365)
const role = ref(RoleReader)
const key = ref('')

watchEffect(() => {
  if (open.value) return

  isCreating.value = false
  name.value = ''
  expiration.value = 365
  role.value = RoleReader
  key.value = ''
})

watchEffect(() => {
  if (!open.value) return

  name.value = propName ?? name.value
  role.value = propRole ?? role.value
})

const handleCreate = async () => {
  isCreating.value = true

  try {
    key.value = await createServiceAccount(name.value, role.value, expiration.value)

    showSuccess('Service Account Was Created')
  } catch (e) {
    showError('Failed to Create Service Account', e instanceof Error ? e.message : String(e))
  } finally {
    isCreating.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Create Service Account"
    :cancel-label="key ? 'Close' : 'Cancel'"
    :action-label="key ? undefined : 'Create Service Account'"
    :action-disabled="!name || (!canManageUsers && authType !== AuthType.SAML)"
    content-class="max-w-xl"
    :loading="isCreating"
    @confirm="handleCreate"
  >
    <ServiceAccountKey v-if="key" :secret-key="key" />

    <div v-else class="flex flex-col gap-2">
      <TInput v-if="!propName" v-model="name" title="ID" placeholder="..." />
      <TInput v-model="expiration" title="Expiration Days" type="number" :min="1" />
      <TSelectList v-if="!propRole" v-model="role" title="Role" :values="roles" />
    </div>
  </Modal>
</template>
