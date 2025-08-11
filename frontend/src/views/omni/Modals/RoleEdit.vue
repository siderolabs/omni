<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import {
  DefaultNamespace,
  RoleAdmin,
  RoleNone,
  RoleOperator,
  RoleReader,
  UserType,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import Watch from '@/components/common/Watch/Watch.vue'
import { canManageUsers } from '@/methods/auth'
import { updateRole } from '@/methods/user'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role: Ref<string | undefined> = ref()

const object = route.query.serviceAccount ? 'Service Account' : 'User'
const id = route.query.identity ?? route.query.serviceAccount

const handleRoleUpdate = async () => {
  if (!role.value) {
    return
  }

  if (!route.query.user) {
    showError(`Failed to Update ${object}`, 'User id is not defined')

    return
  }

  try {
    await updateRole(route.query.user as string, role.value)
  } catch (e) {
    showError(`Failed to Update ${object}`, e.message)

    return
  }

  showSuccess(`Successfully Updated ${object} ${id}`)
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
      <h3 class="truncate text-base text-naturals-N14">Edit {{ object }} {{ id }}</h3>
      <CloseButton @click="close" />
    </div>

    <div class="flex flex-wrap gap-4">
      <Watch
        :opts="{
          resource: {
            type: UserType,
            namespace: DefaultNamespace,
            id: $route.query.user as string,
          },
          runtime: Runtime.Omni,
        }"
        class="flex-1"
      >
        <template #default="{ items }">
          <TSelectList
            v-if="items[0]?.spec?.role"
            class="h-full"
            title="Role"
            :values="roles"
            :default-value="items[0]?.spec?.role"
            @checked-value="(value) => (role = value)"
          />
        </template>
      </Watch>
      <TButton
        type="highlighted"
        :disabled="!canManageUsers"
        class="h-9"
        @click="
          () => {
            handleRoleUpdate()
            close()
          }
        "
        >Update {{ object }}</TButton
      >
    </div>
  </div>
</template>

<style scoped>
.window {
  @apply z-30 flex w-1/3 flex-col rounded bg-naturals-N2 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-N14;
}
</style>
