<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { UserSpec } from '@/api/omni/specs/auth.pb'
import {
  DefaultNamespace,
  RoleAdmin,
  RoleNone,
  RoleOperator,
  RoleReader,
  UserType,
} from '@/api/resources'
import Modal from '@/components/Modals/Modal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import { usePermissions } from '@/methods/auth'
import { useResourceGet } from '@/methods/useResourceGet'
import { showError, showSuccess } from '@/notification'

const { identity, userId, isServiceAccount } = defineProps<{
  identity: string
  userId: string
  isServiceAccount?: boolean
}>()

const open = defineModel<boolean>('open', { default: false })

const { canManageUsers } = usePermissions()

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role = ref<string>()
const isEditing = ref(false)

const object = computed(() => (isServiceAccount ? 'Service Account' : 'User'))

const { data: user } = useResourceGet<UserSpec>(() => ({
  skip: !open.value,
  resource: {
    id: userId,
    namespace: DefaultNamespace,
    type: UserType,
  },
  runtime: Runtime.Omni,
}))

watchEffect(() => {
  if (!open.value || !user.value?.spec.role) return

  role.value = user.value.spec.role
})

watchEffect(() => {
  if (open.value) return

  role.value = undefined
  isEditing.value = false
})

const handleRoleUpdate = async () => {
  isEditing.value = true

  try {
    await ManagementService.UpdateUser({ email: identity, role: role.value })

    showSuccess(`Successfully Updated ${object.value} ${identity}`)

    open.value = false
  } catch (e) {
    showError(`Failed to Update ${object.value}`, e instanceof Error ? e.message : String(e))
  } finally {
    isEditing.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    :title="`Edit ${object} ${identity}`"
    :action-label="`Update ${object}`"
    :action-disabled="!role || !userId || !canManageUsers"
    :loading="isEditing"
    @confirm="handleRoleUpdate"
  >
    <TSelectList v-if="role" v-model="role" title="Role" :values="roles" class="w-full" />
  </Modal>
</template>
