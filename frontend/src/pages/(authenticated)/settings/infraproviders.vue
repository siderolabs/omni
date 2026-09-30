<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { IdentitySpec } from '@/api/omni/specs/auth.pb'
import type { InfraProviderCombinedStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  DefaultNamespace,
  EphemeralNamespace,
  IdentityType,
  InfraProviderCombinedStatusType,
  InfraProviderServiceAccountDomain,
  RoleInfraProvider,
} from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TList from '@/components/List/TList.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TStatus from '@/components/Status/TStatus.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { TCommonStatuses } from '@/constants'
import { usePermissions } from '@/methods/auth'
import { useTitle } from '@/methods/title'
import InfraProviderDeleteModal from '@/views/InfraProviders/components/InfraProviderDeleteModal.vue'
import InfraProviderSetupModal from '@/views/InfraProviders/components/InfraProviderSetupModal.vue'
import ServiceAccountCreateModal from '@/views/Users/components/ServiceAccountCreateModal.vue'
import ServiceAccountRenewKeyModal from '@/views/Users/components/ServiceAccountRenewKeyModal.vue'

definePage({
  name: 'InfraProviders',
})

const { canManageUsers } = usePermissions()
const infraProviderSetupModalOpen = ref(false)
const infraProviderDeleteModalOpen = ref(false)
const infraProviderDeleteProviderId = ref<string>()
const serviceAccCreateModal = ref<{
  open: boolean
  name?: string
  role?: string
}>({
  open: false,
})

const rotateKeyModal = ref<{
  open: boolean
  identity?: string
}>({
  open: false,
})

const getStatus = (item: Resource<InfraProviderCombinedStatusSpec>) => {
  if (!item.spec.health?.initialized) {
    return TCommonStatuses.AWAITING_CONNECTION
  }

  if (!item.spec.health?.connected) {
    return TCommonStatuses.DISCONNECTED
  }

  if (item.spec.health?.error) {
    return TCommonStatuses.UNHEALTHY
  }

  return TCommonStatuses.HEALTHY
}

const openRotateSecretKey = async (name: string) => {
  const saName = `${name}@${InfraProviderServiceAccountDomain}`

  let identity: Resource<IdentitySpec> | undefined
  try {
    identity = await ResourceService.Get(
      {
        namespace: DefaultNamespace,
        type: IdentityType,
        id: saName,
      },
      withRuntime(Runtime.Omni),
    )
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      throw e
    }
  }

  if (identity) {
    rotateKeyModal.value = {
      open: true,
      identity: saName,
    }
  } else {
    serviceAccCreateModal.value = {
      open: true,
      name,
      role: RoleInfraProvider,
    }
  }
}

useTitle('Infra Providers')
</script>

<template>
  <PageContainer class="flex h-full flex-col gap-4">
    <div class="flex items-start gap-1">
      <PageHeader title="Settings" subtitle="Infra Providers" class="flex-1" />
    </div>

    <div class="flex grow flex-col gap-2">
      <div class="flex justify-end">
        <TButton
          icon-position="left"
          variant="highlighted"
          :disabled="!canManageUsers"
          @click="infraProviderSetupModalOpen = true"
        >
          New Infra Provider Setup
        </TButton>
      </div>
      <TList
        :opts="{
          type: undefined as unknown as InfraProviderCombinedStatusSpec,
          runtime: Runtime.Omni,
          resource: {
            namespace: EphemeralNamespace,
            type: InfraProviderCombinedStatusType,
          },
          sortByField: 'created',
        }"
        search
        no-records-alert
        pagination
        errors-alert
        filter-caption="Status"
      >
        <template #default="{ items }">
          <div class="flex flex-col gap-2">
            <div
              v-for="item in items"
              :key="item.metadata.id"
              class="grid grid-cols-5 items-center rounded border border-border-strong bg-surface-chrome p-3"
              :class="{ 'border-dashed': !item.spec.name }"
            >
              <div class="flex items-center gap-3">
                <TIcon
                  :svg-base-64="item.spec.icon"
                  icon="cloud-connection"
                  class="size-8 text-content-default"
                />

                <div class="flex flex-col gap-0.5">
                  <span v-if="item.spec.name" class="text-md text-content-default">
                    {{ item.spec.name }}
                  </span>

                  <span class="text-xs font-bold text-content-muted">
                    ID: {{ item.metadata.id }}
                  </span>
                </div>
              </div>

              <div
                class="truncate text-xs"
                :class="item.spec.version ? 'text-content-emphasis' : 'text-content-muted'"
              >
                {{ item.spec.version || 'Unknown version' }}
              </div>

              <Tooltip :description="item.spec.health?.error" placement="top-start">
                <TStatus :title="getStatus(item)" />
              </Tooltip>
              <div class="truncate text-xs">
                {{ item.spec.description }}
              </div>

              <div class="justify-self-end">
                <IconButton icon="key" @click="() => openRotateSecretKey(item.metadata.id!)" />
                <IconButton
                  icon="delete"
                  danger
                  @click="
                    () => {
                      infraProviderDeleteProviderId = item.metadata.id
                      infraProviderDeleteModalOpen = true
                    }
                  "
                />
              </div>
            </div>
          </div>
        </template>
      </TList>
    </div>

    <ServiceAccountCreateModal
      v-model:open="serviceAccCreateModal.open"
      :name="serviceAccCreateModal.name"
      :role="serviceAccCreateModal.role"
    />

    <ServiceAccountRenewKeyModal
      v-if="rotateKeyModal.identity"
      v-model:open="rotateKeyModal.open"
      :identity="rotateKeyModal.identity"
    />

    <InfraProviderSetupModal v-model:open="infraProviderSetupModalOpen" />

    <InfraProviderDeleteModal
      v-if="infraProviderDeleteProviderId"
      v-model:open="infraProviderDeleteModalOpen"
      :provider-id="infraProviderDeleteProviderId"
    />
  </PageContainer>
</template>
