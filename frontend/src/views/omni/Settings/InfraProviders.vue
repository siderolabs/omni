<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

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
import type { WatchOptions } from '@/api/watch'
import { itemID } from '@/api/watch'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TList from '@/components/common/List/TList.vue'
import TStatus from '@/components/common/Status/TStatus.vue'
import { TCommonStatuses } from '@/constants'
import { canManageUsers } from '@/methods/auth'
import type { Label } from '@/methods/labels'
import { selectors } from '@/methods/labels'

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

<template>
  <div class="flex flex-col gap-2">
    <div class="flex justify-end">
      <TButton
        icon-position="left"
        variant="highlighted"
        :disabled="!canManageUsers"
        @click="openInfraProviderSetup"
      >
        New Infra Provider Setup
      </TButton>
    </div>
    <TList
      :opts="watchOpts"
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
            :key="itemID(item)"
            class="grid grid-cols-4 items-center rounded border border-naturals-n5 bg-naturals-n1 p-3"
            :class="{ 'border-dashed': !item.spec.name }"
          >
            <div class="flex items-center gap-3">
              <TIcon
                :svg-base-64="item.spec.icon"
                icon="cloud-connection"
                class="size-8 text-naturals-n13"
              />

              <div class="flex flex-col gap-0.5">
                <span v-if="item.spec.name" class="text-md text-naturals-n13">
                  {{ item.spec.name }}
                </span>

                <span class="text-xs font-bold text-naturals-n10">ID: {{ item.metadata.id }}</span>
              </div>
            </div>

            <TStatus :title="getStatus(item)" />
            <div class="truncate text-xs">
              {{ item.spec.description }}
            </div>

            <div class="justify-self-end">
              <IconButton icon="key" @click="() => openRotateSecretKey(item.metadata.id!)" />
              <IconButton
                icon="delete"
                danger
                @click="() => openInfraProviderDelete(item.metadata.id!)"
              />
            </div>
          </div>
        </div>
      </template>
    </TList>
  </div>
</template>
