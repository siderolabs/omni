<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { ManagementService } from '@/api/omni/management/management.pb'
import { RoleAdmin, RoleNone, RoleOperator, RoleReader } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TInput from '@/components/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/Modals/CloseButton.vue'

const identity = ref('')
const router = useRouter()
const { canManageUsers } = usePermissions()

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role: Ref<string> = ref(RoleReader)

const handleUserCreate = async () => {
  if (identity.value === '') {
    showError('Failed to Create User', 'User email is not defined')

    return
  }

  try {
    await ManagementService.CreateUser({ email: identity.value, role: role.value })
  } catch (e) {
    showError('Failed to Create User', e.message)

    return
  }

  close()

  showSuccess(`The User ${identity.value} is created`)
}

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Create User</h3>
      <CloseButton @click="close" />
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <TInput v-model="identity" title="User Email" class="h-full flex-1" placeholder="..." />
      <TSelectList
        class="h-full"
        title="Role"
        :values="roles"
        :default-value="RoleReader"
        @checked-value="(value) => (role = value)"
      />
      <TButton
        variant="highlighted"
        :disabled="!canManageUsers && authType !== AuthType.SAML"
        class="h-9 w-32"
        @click="handleUserCreate"
      >
        Create User
      </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../index.css";

.window {
  @apply z-30 flex w-1/3 flex-col rounded bg-naturals-n2 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
