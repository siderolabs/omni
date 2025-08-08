<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Create Service Account</h3>
      <close-button @click="close" />
    </div>

    <div class="flex flex-col w-full h-full gap-4">
      <t-notification v-if="notification" v-bind="notification.props" />

      <template v-if="!key">
        <div class="flex flex-col gap-2">
          <t-input
            title="ID"
            class="flex-1 h-full"
            placeholder="..."
            v-model="name"
            v-if="!route.query.name"
          />
          <t-input
            title="Expiration Days"
            type="number"
            :min="1"
            class="flex-1 h-full"
            v-model="expiration"
          />
          <t-select-list
            class="h-full"
            title="Role"
            :values="roles"
            :defaultValue="RoleReader"
            v-if="!route.query.role"
            @checkedValue="(value) => (role = value)"
          />
        </div>
        <t-button
          type="highlighted"
          @click="handleCreate"
          :disabled="!canManageUsers && authType !== AuthType.SAML"
          class="h-9"
          >Create Service Account</t-button
        >
      </template>
    </div>

    <ServiceAccountKey v-if="key" :secret-key="key" />
  </div>
</template>

<script setup lang="ts">
import type { Ref } from 'vue'
import { ref, shallowRef } from 'vue'
import { createServiceAccount } from '@/methods/user'
import type { Notification } from '@/notification'
import { showError, showSuccess } from '@/notification'
import { useRoute, useRouter } from 'vue-router'
import { RoleNone, RoleReader, RoleOperator, RoleAdmin } from '@/api/resources'

import CloseButton from '@/views/omni/Modals/CloseButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import { canManageUsers } from '@/methods/auth'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import TNotification from '@/components/common/Notification/TNotification.vue'
import ServiceAccountKey from './components/ServiceAccountKey.vue'

const notification: Ref<Notification | null> = shallowRef(null)

const expiration = ref(365)
const router = useRouter()
const route = useRoute()
const name = ref((route.query.name as string) ?? '')

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role: Ref<string> = ref((route.query.role as string) ?? RoleReader)

const key = ref<string>()

const handleCreate = async () => {
  if (name.value === '') {
    showError('Failed to Create Service Account', 'Name is not defined', notification)

    return
  }

  try {
    key.value = await createServiceAccount(name.value, role.value, expiration.value)
  } catch (e) {
    showError('Failed to Create Service Account', e.message, notification)

    return
  }

  showSuccess('Service Account Was Created', undefined, notification)
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

<style scoped>
.window {
  @apply rounded bg-naturals-N2 z-30 w-1/3 flex flex-col p-8;
}

.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}

code {
  @apply break-all rounded bg-naturals-N4;
}
</style>
