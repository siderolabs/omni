<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { IdentitySpec } from '@/api/omni/specs/auth.pb'
import {
  DefaultNamespace,
  IdentityType,
  LabelIdentityTypeServiceAccount,
  LabelIdentityUserID,
  UserType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import { AuthType, authType } from '@/methods'
import { canManageUsers } from '@/methods/auth'
import UserItem from '@/views/omni/Users/UserItem.vue'

const router = useRouter()

const watchOpts = [
  {
    runtime: Runtime.Omni,
    resource: {
      type: IdentityType,
      namespace: DefaultNamespace,
    },
    idFunc: (res: Resource<IdentitySpec>) =>
      `default.${(res?.metadata?.labels || {})[LabelIdentityUserID] ?? ''}`,
    selectors: [`!${LabelIdentityTypeServiceAccount}`],
  },
  {
    runtime: Runtime.Omni,
    resource: {
      type: UserType,
      namespace: DefaultNamespace,
    },
  },
]

const openUserCreate = () => {
  router.push({
    query: { modal: 'userCreate' },
  })
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <div class="flex justify-end">
      <TButton
        icon="user-add"
        icon-position="left"
        type="highlighted"
        :disabled="!canManageUsers || authType === AuthType.SAML"
        @click="openUserCreate"
      >
        Add User
      </TButton>
    </div>
    <TList :opts="watchOpts" pagination class="flex-1" search>
      <template #default="{ items }">
        <div class="users-header">
          <div class="users-grid">
            <div>Email</div>
            <div>Role</div>
            <div class="col-span-3">Labels</div>
          </div>
        </div>
        <UserItem v-for="item in items" :key="itemID(item)" :item="item" />
      </template>
    </TList>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.users-grid {
  @apply grid grid-cols-5 pr-2;
}

.users-header {
  @apply mb-1 bg-naturals-n2;
  padding: 10px 16px;
}

.users-header > * {
  @apply text-xs;
}
</style>
