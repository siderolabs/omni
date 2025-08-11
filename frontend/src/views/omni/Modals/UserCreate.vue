<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import { RoleAdmin, RoleNone, RoleOperator, RoleReader } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TNotification from '@/components/common/Notification/TNotification.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { canManageUsers } from '@/methods/auth'
import { createUser } from '@/methods/user'
import type { Notification } from '@/notification'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const notification: Ref<Notification | null> = shallowRef(null)

const identity = ref('')
const router = useRouter()

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role: Ref<string> = ref(RoleReader)

const handleUserCreate = async () => {
  if (identity.value === '') {
    showError('Failed to Create User', 'User email is not defined', notification)

    return
  }

  try {
    await createUser(identity.value, role.value)
  } catch (e) {
    showError('Failed to Create User', e.message, notification)

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

    <div class="flex h-full w-full flex-col gap-4">
      <TNotification v-if="notification" v-bind="notification.props" />

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
          type="highlighted"
          :disabled="!canManageUsers && authType !== AuthType.SAML"
          class="h-9 w-32"
          @click="handleUserCreate"
          >Create User</TButton
        >
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.window {
  @apply z-30 flex w-1/3 flex-col rounded bg-naturals-n2 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
