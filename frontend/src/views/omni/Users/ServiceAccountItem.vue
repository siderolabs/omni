<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { toRefs } from 'vue'
import { useRouter } from 'vue-router'

import type { Resource } from '@/api/grpc'
import type { IdentitySpec, UserSpec } from '@/api/omni/specs/auth.pb'
import { RoleInfraProvider } from '@/api/resources'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import { canManageUsers } from '@/methods/auth'

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

<template>
  <TListItem>
    <template #default>
      <div class="flex items-center gap-2">
        <div class="users-grid flex-1 text-naturals-n13">
          <div class="font-bold">{{ item.metadata.id }}</div>
          <div class="max-w-min rounded bg-naturals-n3 px-2 py-1 text-naturals-n10">
            {{ props.item.spec.role ?? 'None' }}
          </div>
          <div>{{ expiration }}</div>
        </div>
        <div class="flex justify-between">
          <TActionsBox v-if="canManageUsers" style="height: 24px">
            <TActionsBoxItem icon="refresh" @click.stop="renewKey">Renew Key</TActionsBoxItem>
            <TActionsBoxItem
              v-if="item.spec.role !== RoleInfraProvider"
              icon="edit"
              @click.stop="editUser"
            >
              Edit Service Account
            </TActionsBoxItem>
            <TActionsBoxItem icon="delete" danger @click.stop="deleteUser">
              Delete Service Account
            </TActionsBoxItem>
          </TActionsBox>
        </div>
      </div>
    </template>
  </TListItem>
</template>

<style scoped>
@reference "../../../index.css";

.users-grid {
  @apply grid grid-cols-3 items-center pr-2;
}

.users-grid > * {
  @apply truncate text-xs;
}
</style>
