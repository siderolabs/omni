<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { onBeforeMount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { IdentitySpec, UserSpec } from '@/api/omni/specs/auth.pb'
import { withRuntime } from '@/api/options'
import {
  DefaultNamespace,
  IdentityType,
  RoleAdmin,
  RoleNone,
  RoleOperator,
  RoleReader,
  UserType,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import { canManageUsers } from '@/methods/auth'
import { updateRole } from '@/methods/user'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()

const roles = [RoleNone, RoleReader, RoleOperator, RoleAdmin]

const role: Ref<string | undefined> = ref()

const object = route.query.serviceAccount ? 'Service Account' : 'User'
const id = route.query.identity ?? route.query.serviceAccount
const userID = ref(route.query.user as string)

const { data: user } = useResourceWatch<UserSpec>(() => ({
  resource: {
    id: userID.value,
    namespace: DefaultNamespace,
    type: UserType,
  },
  runtime: Runtime.Omni,
  skip: !userID.value,
}))

onBeforeMount(async () => {
  if (!route.query.serviceAccount) {
    return
  }

  try {
    const identity: Resource<IdentitySpec> = await ResourceService.Get(
      {
        id: route.query.serviceAccount as string,
        namespace: DefaultNamespace,
        type: IdentityType,
      },
      withRuntime(Runtime.Omni),
    )

    userID.value = identity.spec.user_id!
  } catch (e) {
    showError('Failed to fetch service account', e.message)
  }
})

const handleRoleUpdate = async () => {
  if (!role.value) {
    return
  }

  if (!userID.value) {
    showError(`Failed to Update ${object}`, 'User id is not defined')
  }

  try {
    await updateRole(userID.value, role.value)
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
      <h3 class="truncate text-base text-naturals-n14">Edit {{ object }} {{ id }}</h3>
      <CloseButton @click="close" />
    </div>

    <div class="flex flex-wrap gap-4">
      <TSelectList
        v-if="user?.spec?.role"
        class="h-full grow"
        title="Role"
        :values="roles"
        :default-value="user.spec?.role"
        @checked-value="(value) => (role = value)"
      />
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
      >
        Update {{ object }}
      </TButton>
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
