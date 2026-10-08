<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { IdentityStatusSpec, ServiceAccountStatusSpec } from '@/api/omni/specs/auth.pb'
import {
  EphemeralNamespace,
  IdentityStatusType,
  LabelIdentityTypeServiceAccount,
  ServiceAccountStatusType,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import Pagination from '@/components/Pagination/Pagination.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { usePermissions } from '@/methods/auth'
import { useResourcePagination } from '@/methods/resource/useResourcePagination'
import { useResourceSearch } from '@/methods/resource/useResourceSearch'
import { relativeISO } from '@/methods/time'
import { useTitle } from '@/methods/title'
import { useResourceWatch } from '@/methods/useResourceWatch'
import RoleEditModal from '@/views/Users/components/RoleEditModal.vue'
import ServiceAccountCreateModal from '@/views/Users/components/ServiceAccountCreateModal.vue'
import ServiceAccountItem from '@/views/Users/components/ServiceAccountItem.vue'
import ServiceAccountRenewKeyModal from '@/views/Users/components/ServiceAccountRenewKeyModal.vue'
import ServiceAccountRevokeKeyModal from '@/views/Users/components/ServiceAccountRevokeKeyModal.vue'
import UserDestroyModal from '@/views/Users/components/UserDestroyModal.vue'

definePage({
  name: 'ServiceAccounts',
})

useTitle('Service Accounts')

const { canManageUsers } = usePermissions()

const serviceAccCreateModalOpen = ref(false)
const filterValue = ref('')

const userDestroyModal = ref<{ open: boolean; identity?: string }>({ open: false })
const roleEditModal = ref<{ open: boolean; identity?: string; userId?: string }>({ open: false })
const rotateKeyModal = ref<{ open: boolean; identity?: string }>({ open: false })
const keyRevokeModal = ref<{ open: boolean; identity?: string; publicKeyId?: string }>({
  open: false,
})

const expandedIds = ref(new Set<string>())

function toggleExpanded(id: string) {
  if (expandedIds.value.has(id)) {
    expandedIds.value.delete(id)
  } else {
    expandedIds.value.add(id)
  }
}

const { watchOptions: searchState, searchQuery } = useResourceSearch({ filterValue })

const {
  total,
  watchOptions: paginationState,
  currentPage,
  currentPageSize,
  pageCount,
  pageSizeSelectValues,
} = useResourcePagination({
  resetOn: [searchState],
})

const { data: identities } = useResourceWatch<IdentityStatusSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: IdentityStatusType,
    namespace: EphemeralNamespace,
  },
  selectors: [LabelIdentityTypeServiceAccount],
})

const {
  data: serviceAccounts,
  loading,
  err,
} = useResourceWatch<ServiceAccountStatusSpec>(
  () => ({
    runtime: Runtime.Omni,
    resource: {
      type: ServiceAccountStatusType,
      namespace: EphemeralNamespace,
    },
    ...paginationState.value,
    ...searchState.value,
  }),
  { total },
)

const identityMap = computed(() => new Map(identities.value.map((s) => [s.metadata.id!, s])))

const keyRevokeModalKeys = computed(
  () =>
    serviceAccounts.value.find((sa) => sa.metadata.id === keyRevokeModal.value.identity)?.spec
      .public_keys ?? [],
)

const getLastActive = (serviceAcc: Resource<ServiceAccountStatusSpec>) => {
  const identity = identityMap.value.get(serviceAcc.metadata.id!)

  return identity?.spec.last_active ? relativeISO(identity.spec.last_active) : 'Never'
}
</script>

<template>
  <PageContainer class="flex h-full flex-col gap-compact">
    <PageHeader title="Settings" subtitle="Service Accounts" />

    <div class="flex grow flex-col gap-tight">
      <div class="flex justify-end">
        <TButton
          icon="plus"
          icon-position="left"
          variant="highlighted"
          :disabled="!canManageUsers"
          @click="serviceAccCreateModalOpen = true"
        >
          Create Service Account
        </TButton>
      </div>

      <TInput v-model="filterValue" icon="search" />

      <TSelectList
        v-model="currentPageSize"
        class="self-end"
        title="Items per Page"
        :values="pageSizeSelectValues"
      />

      <div v-if="loading" class="flex grow items-center justify-center">
        <TSpinner class="size-6" />
      </div>

      <TAlert v-else-if="err" title="Failed to Fetch Data" type="error">{{ err }}.</TAlert>

      <TAlert v-else-if="serviceAccounts.length === 0" type="info" title="No Records">
        No service accounts found.
      </TAlert>

      <div
        v-else
        class="grid grid-cols-[minmax(0,2fr)_repeat(4,minmax(0,1fr))_--spacing(24)] gap-snug"
      >
        <div
          class="col-span-full grid grid-cols-subgrid bg-surface-card px-snug py-2.5 text-xs max-lg:hidden"
        >
          <div class="pl-base">ID</div>
          <div>Role</div>
          <div>Last Active</div>
          <div>Keys</div>
          <div>Expiration</div>
          <div>Actions</div>
        </div>

        <ul class="col-span-full grid grid-cols-subgrid gap-snug">
          <ServiceAccountItem
            v-for="item in serviceAccounts"
            :key="item.metadata.id"
            :item
            :last-active="getLastActive(item)"
            :search-query="searchQuery"
            :model-value="expandedIds.has(item.metadata.id!)"
            @update:model-value="toggleExpanded(item.metadata.id!)"
            @rotate="rotateKeyModal = { open: true, identity: item.metadata.id }"
            @edit="
              roleEditModal = {
                open: true,
                identity: item.metadata.id,
                userId: identityMap.get(item.metadata.id!)?.spec.user_id,
              }
            "
            @delete="userDestroyModal = { open: true, identity: item.metadata.id }"
            @revoke-key="
              (publicKeyId) =>
                (keyRevokeModal = { open: true, identity: item.metadata.id, publicKeyId })
            "
          />
        </ul>
      </div>
    </div>

    <Pagination v-model:current-page="currentPage" :page-count="pageCount" />

    <ServiceAccountCreateModal v-model:open="serviceAccCreateModalOpen" />

    <ServiceAccountRenewKeyModal
      v-if="rotateKeyModal.identity"
      v-model:open="rotateKeyModal.open"
      :identity="rotateKeyModal.identity"
    />

    <ServiceAccountRevokeKeyModal
      v-if="keyRevokeModal.identity && keyRevokeModal.publicKeyId"
      v-model:open="keyRevokeModal.open"
      :identity="keyRevokeModal.identity"
      :public-key-id="keyRevokeModal.publicKeyId"
      :keys="keyRevokeModalKeys"
    />

    <RoleEditModal
      v-if="roleEditModal.identity && roleEditModal.userId"
      v-model:open="roleEditModal.open"
      :identity="roleEditModal.identity"
      :user-id="roleEditModal.userId"
      is-service-account
    />

    <UserDestroyModal
      v-if="userDestroyModal.identity"
      v-model:open="userDestroyModal.open"
      :identity="userDestroyModal.identity"
      is-service-account
    />
  </PageContainer>
</template>
