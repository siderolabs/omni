<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { IdentityStatusSpec } from '@/api/omni/specs/auth.pb'
import {
  EphemeralNamespace,
  IdentityStatusType,
  LabelIdentityTypeServiceAccount,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TList from '@/components/List/TList.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import { AuthType, authType } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { relativeISO } from '@/methods/time'
import UserCreateModal from '@/views/Users/components/UserCreateModal.vue'
import UserItem from '@/views/Users/UserItem.vue'

definePage({
  name: 'Users',
})

const { canManageUsers } = usePermissions()

const useCreateModalOpen = ref(false)

const getLastActive = (item: Resource<IdentityStatusSpec>) => {
  if (!item.spec.last_active) return 'Never'

  return relativeISO(item.spec.last_active)
}
</script>

<template>
  <PageContainer class="flex h-full flex-col gap-4">
    <div class="flex items-start gap-1">
      <PageHeader title="Settings" subtitle="Users" class="flex-1" />
    </div>

    <div class="flex grow flex-col gap-2">
      <div class="flex justify-end">
        <TButton
          icon="user-plus"
          icon-position="left"
          variant="highlighted"
          :disabled="!canManageUsers || authType === AuthType.SAML"
          @click="useCreateModalOpen = true"
        >
          Add User
        </TButton>
      </div>
      <TList
        :opts="{
          type: undefined as unknown as IdentityStatusSpec,
          runtime: Runtime.Omni,
          resource: {
            type: IdentityStatusType,
            namespace: EphemeralNamespace,
          },
          selectors: [`!${LabelIdentityTypeServiceAccount}`],
        }"
        pagination
        class="flex-1"
        search
      >
        <template #default="{ items }">
          <div class="mb-1 bg-naturals-n2 px-4 py-2.5 text-xs">
            <div class="grid grid-cols-6 pr-2">
              <div>Email</div>
              <div>Role</div>
              <div>Last Active</div>
              <div class="col-span-3">Labels</div>
            </div>
          </div>
          <UserItem
            v-for="item in items"
            :key="item.metadata.id"
            :item="item"
            :last-active="getLastActive(item)"
          />
        </template>
      </TList>
    </div>

    <UserCreateModal v-model:open="useCreateModalOpen" />
  </PageContainer>
</template>
