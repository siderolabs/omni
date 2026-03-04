<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { IdentityStatusSpec, ServiceAccountStatusSpec } from '@/api/omni/specs/auth.pb'
import {
  EphemeralNamespace,
  IdentityStatusType,
  LabelIdentityTypeServiceAccount,
  ServiceAccountStatusType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { canManageUsers } from '@/methods/auth'
import { relativeISO } from '@/methods/time'
import ServiceAccountItem from '@/views/omni/Users/ServiceAccountItem.vue'

const router = useRouter()

const watchOpts = [
  {
    runtime: Runtime.Omni,
    resource: {
      type: ServiceAccountStatusType,
      namespace: EphemeralNamespace,
    },
    idFunc: (res: Resource) => res.metadata.id!,
  },
  {
    runtime: Runtime.Omni,
    resource: {
      type: IdentityStatusType,
      namespace: EphemeralNamespace,
    },
    selectors: [LabelIdentityTypeServiceAccount],
    idFunc: (res: Resource) => res.metadata.id!,
  },
]

const getLastActive = (item: Resource<IdentityStatusSpec>) => {
  if (!item.spec.last_active) return 'Never'

  return relativeISO(item.spec.last_active)
}

const getExpiration = (item: Resource<ServiceAccountStatusSpec>) => {
  return relativeISO(
    item.spec.public_keys?.[(item.spec.public_keys?.length ?? 0) - 1].expiration ?? '',
  )
}

const openUserCreate = () => {
  router.push({
    query: { modal: 'serviceAccountCreate' },
  })
}
</script>

<template>
  <PageContainer class="flex h-full flex-col gap-4">
    <div class="flex items-start gap-1">
      <PageHeader title="Settings" class="flex-1" subtitle="Service Accounts" />
    </div>

    <div class="flex grow flex-col gap-2">
      <div class="flex justify-end">
        <TButton
          icon="plus"
          icon-position="left"
          variant="highlighted"
          :disabled="!canManageUsers"
          @click="openUserCreate"
        >
          Create Service Account
        </TButton>
      </div>
      <TList :opts="watchOpts" pagination class="flex-1" search>
        <template #default="{ items }">
          <div class="users-header">
            <div class="users-grid">
              <div>ID</div>
              <div>Role</div>
              <div>Last Active</div>
              <div>Expiration</div>
            </div>
          </div>
          <ServiceAccountItem
            v-for="item in items"
            :key="itemID(item)"
            :item="item"
            :last-active="getLastActive(item)"
            :expiration="getExpiration(item)"
          />
        </template>
      </TList>
    </div>
  </PageContainer>
</template>

<style scoped>
@reference "../../../index.css";

.users-grid {
  @apply grid grid-cols-4 pr-10;
}

.users-header {
  @apply mb-1 bg-naturals-n2;
  padding: 10px 16px;
}

.users-header > * {
  @apply text-xs;
}
</style>
