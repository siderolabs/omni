<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import type { Resource } from '@/api/grpc'
import type { IdentityStatusSpec } from '@/api/omni/specs/auth.pb'
import { SAMLLabelPrefix } from '@/api/resources'
import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import TListItem from '@/components/List/TListItem.vue'
import { usePermissions } from '@/methods/auth'
import RoleEditModal from '@/views/Users/components/RoleEditModal.vue'
import UserDestroyModal from '@/views/Users/components/UserDestroyModal.vue'

const { item } = defineProps<{
  item: Resource<IdentityStatusSpec>
  lastActive: string
}>()

const { canManageUsers } = usePermissions()

const userDestroyModal = ref<{
  open: boolean
  identity?: string
}>({
  open: false,
})

const roleEditModal = ref<{
  open: boolean
  identity?: string
  userId?: string
}>({
  open: false,
})

const labels = computed(() => {
  return (
    Object.keys(item.metadata.labels || {})
      .filter((l) => l.startsWith(SAMLLabelPrefix))
      .map((l: string) => l.replace(`${SAMLLabelPrefix}`, '')) || []
  )
})
</script>

<template>
  <TListItem>
    <template #default>
      <div class="flex items-center gap-2">
        <div class="grid flex-1 grid-cols-6 items-center pr-2 text-xs text-naturals-n13 *:truncate">
          <div class="font-bold">{{ item.metadata.id }}</div>
          <div class="max-w-min rounded bg-naturals-n3 px-2 py-1 text-naturals-n10">
            {{ item.spec.role ?? 'None' }}
          </div>
          <div class="text-naturals-n10">{{ lastActive }}</div>
          <div class="col-span-3 flex flex-wrap gap-1">
            <div v-for="label in labels" :key="label" class="resource-label">
              {{ label }}
            </div>
          </div>
        </div>
        <div class="flex justify-between">
          <TActionsBox v-if="canManageUsers">
            <TActionsBoxItem
              icon="edit"
              @select="
                roleEditModal = {
                  open: true,
                  identity: item.metadata.id,
                  userId: item.spec.user_id,
                }
              "
            >
              Edit User
            </TActionsBoxItem>
            <TActionsBoxItem
              icon="delete"
              danger
              @select="userDestroyModal = { open: true, identity: item.metadata.id }"
            >
              Delete User
            </TActionsBoxItem>
          </TActionsBox>
        </div>
      </div>

      <RoleEditModal
        v-if="roleEditModal.identity && roleEditModal.userId"
        v-model:open="roleEditModal.open"
        :identity="roleEditModal.identity"
        :user-id="roleEditModal.userId"
      />

      <UserDestroyModal
        v-if="userDestroyModal.identity"
        v-model:open="userDestroyModal.open"
        :identity="userDestroyModal.identity"
      />
    </template>
  </TListItem>
</template>
