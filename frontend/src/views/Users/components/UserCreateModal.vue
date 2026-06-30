<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { ManagementService } from '@/api/omni/management/management.pb'
import { RoleAdmin, RoleNone, RoleOperator, RoleReader } from '@/api/resources'
import Modal from '@/components/Modals/Modal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TInput from '@/components/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { showError, showSuccess } from '@/notification'

const open = defineModel<boolean>('open', { default: false })

const { canManageUsers } = usePermissions()

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const identity = ref('')
const role = ref(RoleReader)
const isCreating = ref(false)

watchEffect(() => {
  if (open.value) return

  identity.value = ''
  role.value = RoleReader
  isCreating.value = false
})

const handleUserCreate = async () => {
  try {
    isCreating.value = true

    await ManagementService.CreateUser({ email: identity.value, role: role.value })

    showSuccess(`The User ${identity.value} is created`)

    open.value = false
  } catch (e) {
    showError('Failed to Create User', e instanceof Error ? e.message : String(e))
  } finally {
    isCreating.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Create User"
    action-label="Create User"
    :action-disabled="!identity || (!canManageUsers && authType !== AuthType.SAML)"
    :loading="isCreating"
    @confirm="handleUserCreate"
  >
    <div class="flex flex-wrap items-center gap-2">
      <TInput v-model="identity" title="User Email" class="h-full flex-1" placeholder="..." />

      <TSelectList
        class="h-full"
        title="Role"
        :values="roles"
        :default-value="RoleReader"
        @checked-value="(value) => (role = value)"
      />
    </div>
  </Modal>
</template>
