<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-list-item>
    <template #default>
      <div class="flex items-center gap-2">
        <div class="users-grid text-naturals-N13 flex-1">
          <div class="font-bold">{{ item.metadata.id }}</div>
          <div class="px-2 py-1 max-w-min text-naturals-N10 rounded bg-naturals-N3">
            {{ props.item.spec.role ?? 'None' }}
          </div>
          <div>{{ expiration }}</div>
        </div>
        <div class="flex justify-between">
          <t-actions-box v-if="canManageUsers" style="height: 24px">
            <t-actions-box-item icon="refresh" @click.stop="renewKey">Renew Key</t-actions-box-item>
            <t-actions-box-item
              v-if="item.spec.role !== RoleInfraProvider"
              icon="edit"
              @click.stop="editUser"
              >Edit Service Account</t-actions-box-item
            >
            <t-actions-box-item icon="delete" @click.stop="deleteUser" danger
              >Delete Service Account</t-actions-box-item
            >
          </t-actions-box>
        </div>
      </div>
    </template>
  </t-list-item>
</template>

<script setup lang="ts">
import type { UserSpec, IdentitySpec } from '@/api/omni/specs/auth.pb'
import type { Resource } from '@/api/grpc'

import { useRouter } from 'vue-router'

import { RoleInfraProvider } from '@/api/resources'
import TListItem from '@/components/common/List/TListItem.vue'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import { canManageUsers } from '@/methods/auth'
import { toRefs } from 'vue'

const props = defineProps<{
  expiration?: string
  item: Resource<UserSpec & IdentitySpec>
}>()

const { item } = toRefs(props)

const router = useRouter()

const deleteUser = () => {
  const query: Record<string, string> = {
    user: props.item.spec.user_id!,
    serviceAccount: props.item.metadata.id ?? '',
  }

  router.push({
    query: { modal: 'userDestroy', ...query },
  })
}

const editUser = () => {
  const query: Record<string, string> = {
    serviceAccount: props.item.metadata.id!,
    user: props.item.spec.user_id!,
  }

  router.push({
    query: { modal: 'roleEdit', ...query },
  })
}

const renewKey = () => {
  const query: Record<string, string> = {
    serviceAccount: props.item.metadata.id ?? '',
  }

  router.push({
    query: { modal: 'serviceAccountRenew', ...query },
  })
}
</script>

<style scoped>
.users-grid {
  @apply grid grid-cols-3 pr-2 items-center;
}

.users-grid > * {
  @apply text-xs truncate;
}
</style>
