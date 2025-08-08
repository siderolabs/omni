<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2">
    <div class="flex justify-end">
      <t-button
        @click="openInfraProviderSetup"
        icon-position="left"
        type="highlighted"
        :disabled="!canManageUsers"
        >New Infra Provider Setup</t-button
      >
    </div>
    <t-list :opts="watchOpts" search noRecordsAlert pagination errorsAlert filterCaption="Status">
      <template #default="{ items }">
        <div class="flex flex-col gap-2">
          <list-item-box
            v-for="item in items"
            listID="providers"
            :itemID="item.metadata.id!"
            :key="itemID(item)"
            :class="{ 'border-dashed': !item.spec.name }"
          >
            <template #default>
              <div class="flex w-full">
                <div class="providers-grid">
                  <div class="flex gap-3 items-center text-md text-naturals-N13">
                    <t-icon :svg-base-64="item.spec.icon" icon="cloud-connection" class="w-8 h-8" />
                    <div class="flex flex-col gap-0.5">
                      <div>{{ item.spec.name }}</div>
                      <div class="text-xs text-naturals-N10 font-bold">
                        ID: {{ item.metadata.id! }}
                      </div>
                    </div>
                  </div>
                  <t-status :title="getStatus(item)" />
                  <div class="truncate text-xs">
                    {{ item.spec.description }}
                  </div>
                </div>
                <icon-button icon="key" @click="() => openRotateSecretKey(item.metadata.id!)" />
                <icon-button
                  icon="delete"
                  danger
                  @click="() => openInfraProviderDelete(item.metadata.id!)"
                />
              </div>
            </template>
          </list-item-box>
        </div>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { WatchOptions } from '@/api/watch'
import { itemID } from '@/api/watch'

import ListItemBox from '@/components/common/List/ListItemBox.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

import TList from '@/components/common/List/TList.vue'
import { computed, ref } from 'vue'
import type { Label } from '@/methods/labels'
import { selectors } from '@/methods/labels'
import {
  DefaultNamespace,
  EphemeralNamespace,
  IdentityType,
  InfraProviderCombinedStatusType,
  InfraProviderServiceAccountDomain,
  RoleInfraProvider,
} from '@/api/resources'
import TStatus from '@/components/common/Status/TStatus.vue'
import TButton from '@/components/common/Button/TButton.vue'
import { TCommonStatuses } from '@/constants'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { InfraProviderCombinedStatusSpec } from '@/api/omni/specs/omni.pb'
import IconButton from '@/components/common/Button/IconButton.vue'
import { canManageUsers } from '@/methods/auth'
import { useRouter } from 'vue-router'
import { withRuntime } from '@/api/options'
import { Code } from '@/api/google/rpc/code.pb'
import type { IdentitySpec } from '@/api/omni/specs/auth.pb'

const router = useRouter()

const watchOpts = computed<WatchOptions>(() => {
  return {
    runtime: Runtime.Omni,
    resource: {
      namespace: EphemeralNamespace,
      type: InfraProviderCombinedStatusType,
    },
    selectors: selectors(filterLabels.value),
    sortByField: 'created',
  }
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

const filterLabels = ref<Label[]>([])

const openInfraProviderSetup = () => {
  router.push({
    query: { modal: 'infraProviderSetup' },
  })
}

const openInfraProviderDelete = (name: string) => {
  router.push({
    query: { modal: 'infraProviderDelete', provider: name },
  })
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
    router.push({
      query: { modal: 'serviceAccountRenew', serviceAccount: saName },
    })
  } else {
    router.push({
      query: { modal: 'serviceAccountCreate', name: name, role: RoleInfraProvider },
    })
  }
}
</script>

<style scoped>
.providers-grid {
  @apply flex-1 grid grid-cols-4 mx-1 -my-2 items-center;
}
</style>
