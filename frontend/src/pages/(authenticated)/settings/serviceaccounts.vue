<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'

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
import TList from '@/components/List/TList.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import { usePermissions } from '@/methods/auth'
import { relativeISO } from '@/methods/time'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ServiceAccountCreateModal from '@/views/Users/components/ServiceAccountCreateModal.vue'
import ServiceAccountItem from '@/views/Users/ServiceAccountItem.vue'

definePage({
  name: 'ServiceAccounts',
})

const { canManageUsers } = usePermissions()

const serviceAccCreateModalOpen = ref(false)

const { data: identities } = useResourceWatch<IdentityStatusSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: IdentityStatusType,
    namespace: EphemeralNamespace,
  },
  selectors: [LabelIdentityTypeServiceAccount],
})

const getLastActive = (serviceAcc: Resource<ServiceAccountStatusSpec>) => {
  const identity = identities.value.find((s) => s.metadata.id === serviceAcc.metadata.id)

  if (!identity?.spec.last_active) return 'Never'

  return relativeISO(identity.spec.last_active)
}

const getExpiration = (serviceAcc: Resource<ServiceAccountStatusSpec>) => {
  return relativeISO(serviceAcc.spec.expiration ?? '')
}
</script>

<template>
  <PageContainer class="flex h-full flex-col gap-4">
    <div class="flex items-start gap-1">
      <PageHeader title="Settings" subtitle="Service Accounts" class="flex-1" />
    </div>

    <div class="flex grow flex-col gap-2">
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
      <TList
        :opts="{
          type: undefined as unknown as ServiceAccountStatusSpec,
          runtime: Runtime.Omni,
          resource: {
            type: ServiceAccountStatusType,
            namespace: EphemeralNamespace,
          },
        }"
        pagination
        class="flex-1"
        search
      >
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
            :key="item.metadata.id"
            :item="{
              ...item,
              spec: {
                ...item.spec,
                ...identities.find((s) => s.metadata.id === item.metadata.id)?.spec,
              },
            }"
            :last-active="getLastActive(item)"
            :expiration="getExpiration(item)"
          />
        </template>
      </TList>
    </div>

    <ServiceAccountCreateModal v-model:open="serviceAccCreateModalOpen" />
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
