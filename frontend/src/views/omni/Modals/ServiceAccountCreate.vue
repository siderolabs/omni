<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { RoleAdmin, RoleNone, RoleOperator, RoleReader } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { AuthType, authType } from '@/methods'
import { canManageUsers } from '@/methods/auth'
import { createServiceAccount } from '@/methods/user'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

import ServiceAccountKey from './components/ServiceAccountKey.vue'

const expiration = ref(365)
const router = useRouter()
const route = useRoute()
const name = ref((route.query.name as string) ?? '')

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role: Ref<string> = ref((route.query.role as string) ?? RoleReader)

const key = ref<string>()

const handleCreate = async () => {
  if (name.value === '') {
    showError('Failed to Create Service Account', 'Name is not defined')

    return
  }

  try {
    key.value = await createServiceAccount(name.value, role.value, expiration.value)
  } catch (e) {
    showError('Failed to Create Service Account', e.message)

    return
  }

  showSuccess('Service Account Was Created', undefined)
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
      <h3 class="text-base text-naturals-n14">Create Service Account</h3>
      <CloseButton @click="close" />
    </div>

    <template v-if="!key">
      <div class="flex flex-col gap-2">
        <TInput
          v-if="!route.query.name"
          v-model="name"
          title="ID"
          class="h-full flex-1"
          placeholder="..."
        />
        <TInput
          v-model="expiration"
          title="Expiration Days"
          type="number"
          :min="1"
          class="h-full flex-1"
        />
        <TSelectList
          v-if="!route.query.role"
          class="h-full"
          title="Role"
          :values="roles"
          :default-value="RoleReader"
          @checked-value="(value) => (role = value)"
        />
      </div>
      <TButton
        type="highlighted"
        :disabled="!canManageUsers && authType !== AuthType.SAML"
        class="h-9"
        @click="handleCreate"
        >Create Service Account</TButton
      >
    </template>

    <ServiceAccountKey v-if="key" :secret-key="key" />
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

code {
  @apply rounded bg-naturals-n4 break-all;
}
</style>
