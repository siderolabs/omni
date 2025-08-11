<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'
import { useRouter } from 'vue-router'

import type { Resource } from '@/api/grpc'
import type { IdentitySpec, UserSpec } from '@/api/omni/specs/auth.pb'
import { SAMLLabelPrefix } from '@/api/resources'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import { canManageUsers } from '@/methods/auth'

const props = defineProps<{
  item: Resource<UserSpec & IdentitySpec>
}>()

const { item } = toRefs(props)

const router = useRouter()

const labels = computed(() => {
  return (
    Object.keys(item?.value?.metadata?.labels || {})
      .filter((l) => l.startsWith(SAMLLabelPrefix))
      .map((l: string) => l.replace(`${SAMLLabelPrefix}`, '')) || []
  )
})

const deleteUser = () => {
  const query: Record<string, string> = {
    user: props.item.spec.user_id!,
    identity: props.item.metadata.id ?? '',
  }

  router.push({
    query: { modal: 'userDestroy', ...query },
  })
}

const editUser = () => {
  const query: Record<string, string> = {
    user: props.item.spec.user_id!,
    identity: props.item.metadata.id ?? '',
  }

  router.push({
    query: { modal: 'roleEdit', ...query },
  })
}
</script>

<template>
  <TListItem>
    <template #default>
      <div class="flex items-center gap-2">
        <div class="users-grid flex-1 text-naturals-N13">
          <div class="font-bold">{{ item.metadata.id }}</div>
          <div class="max-w-min rounded bg-naturals-N3 px-2 py-1 text-naturals-N10">
            {{ props.item.spec.role ?? 'None' }}
          </div>
          <div class="col-span-3 flex flex-wrap gap-1">
            <div v-for="label in labels" :key="label" class="resource-label label-light6 text-xs">
              {{ label }}
            </div>
          </div>
        </div>
        <div class="flex justify-between">
          <TActionsBox v-if="canManageUsers" style="height: 24px">
            <TActionsBoxItem icon="edit" @click.stop="editUser">Edit User</TActionsBoxItem>
            <TActionsBoxItem icon="delete" danger @click.stop="deleteUser"
              >Delete User</TActionsBoxItem
            >
          </TActionsBox>
        </div>
      </div>
    </template>
  </TListItem>
</template>

<style scoped>
.users-grid {
  @apply grid grid-cols-5 items-center pr-2;
}

.users-grid > * {
  @apply truncate text-xs;
}

.scope > * {
  @apply bg-naturals-N4 p-0.5 px-1 text-naturals-N10;
}

.scope-action-enabled {
  @apply bg-naturals-N4 p-0.5 px-1 text-green-G1;
}

.scope > *:first-child {
  @apply rounded-l;
}

.scope > *:last-child {
  @apply rounded-r;
}
.label-light6 {
  --label-h: 208;
  --label-s: 70;
  --label-l: 86;
  --lighten-by: 0;
}
</style>
