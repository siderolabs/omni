<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import { type DefaultJoinTokenSpec, JoinTokenStatusSpecState } from '@/api/omni/specs/siderolink.pb'
import { withRuntime } from '@/api/options'
import {
  DefaultJoinTokenID,
  DefaultJoinTokenType,
  DefaultNamespace,
  JoinTokenStatusType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import TStatus from '@/components/common/Status/TStatus.vue'
import { TCommonStatuses } from '@/constants'
import { downloadMachineJoinConfig, getKernelArgs } from '@/methods'
import { canManageUsers, unrevokeJoinToken } from '@/methods/auth'
import { relativeISO } from '@/methods/time'
import { showError } from '@/notification'

const router = useRouter()
const { copy } = useClipboard()

const showTokens = ref(false)

const watchOpts = [
  {
    runtime: Runtime.Omni,
    resource: {
      type: JoinTokenStatusType,
      namespace: DefaultNamespace,
    },
  },
]

const getStatusString = (state: JoinTokenStatusSpecState): TCommonStatuses => {
  switch (state) {
    case JoinTokenStatusSpecState.ACTIVE:
      return TCommonStatuses.ACTIVE
    case JoinTokenStatusSpecState.EXPIRED:
      return TCommonStatuses.EXPIRED
    case JoinTokenStatusSpecState.REVOKED:
      return TCommonStatuses.REVOKED
  }

  return TCommonStatuses.UNKNOWN
}

const openUserCreate = () => {
  router.push({
    query: { modal: 'joinTokenCreate' },
  })
}

const copyValue = (value: string) => {
  return copy(value)
}

const copyKernelParams = async (token: string) => {
  copy(await getKernelArgs(token))
}

const makeDefault = async (token: string) => {
  const defaultJoinToken: Resource<DefaultJoinTokenSpec> = await ResourceService.Get(
    {
      namespace: DefaultNamespace,
      id: DefaultJoinTokenID,
      type: DefaultJoinTokenType,
    },
    withRuntime(Runtime.Omni),
  )

  defaultJoinToken.spec.token_id = token

  try {
    await ResourceService.Update(
      defaultJoinToken,
      defaultJoinToken.metadata.version,
      withRuntime(Runtime.Omni),
    )
  } catch (e) {
    showError('Failed to Update Default Join Token', e.message)
  }
}

const getMachineJoinConfig = (token: string) => {
  downloadMachineJoinConfig(token)
}

const openDownloadInstallationMedia = (token: string) => {
  router.push({
    query: { modal: 'downloadInstallationMedia', joinToken: token },
  })
}

const openRevokeToken = (token: string) => {
  router.push({
    query: { modal: 'joinTokenRevoke', token: token },
  })
}

const openDeleteToken = (token: string) => {
  router.push({
    query: { modal: 'joinTokenDelete', token: token },
  })
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <div class="flex items-start gap-1">
      <PageHeader title="Machine Join Tokens" class="flex-1" />
    </div>
    <div class="flex justify-end">
      <TButton
        icon="plus"
        icon-position="left"
        type="highlighted"
        :disabled="!canManageUsers"
        @click="openUserCreate"
      >
        Create Join Token
      </TButton>
    </div>
    <TList :opts="watchOpts" pagination class="flex-1" search>
      <template #default="{ items }">
        <div class="tokens-header">
          <div class="tokens-grid">
            <div>Name</div>
            <div>Token</div>
            <div>Status</div>
            <div>Expiration</div>
            <div>Use Count</div>
          </div>
        </div>
        <TListItem v-for="item in items" :key="itemID(item)">
          <div class="flex gap-2">
            <div class="tokens-grid flex-1">
              <div class="flex items-center gap-2">
                <span class="truncate">{{ item.spec.name ?? 'initial token' }}</span>
                <div
                  v-if="item.spec.is_default"
                  class="rounded bg-primary-p3/10 px-2 py-1 text-primary-p3"
                >
                  Default
                </div>
              </div>
              <div
                class="cursor-pointer truncate font-mono"
                @click="() => (showTokens = !showTokens)"
              >
                {{ showTokens ? item.metadata.id : item.metadata.id?.replace(/./g, '•') }}
              </div>
              <TStatus :title="getStatusString(item.spec.state)" />
              <div v-if="item.spec.expiration_time">
                {{ relativeISO(item.spec.expiration_time) }}
              </div>
              <div v-else>Never</div>
              <div>
                {{ item.spec.use_count ?? 0 }}
              </div>
            </div>
            <TActionsBox>
              <template v-if="item.spec.state === JoinTokenStatusSpecState.ACTIVE">
                <TActionsBoxItem icon="copy" @click="() => copyValue(item.metadata.id!)">
                  Copy Token
                </TActionsBoxItem>
                <TActionsBoxItem icon="copy" @click="() => copyKernelParams(item.metadata.id!)">
                  Copy Kernel Params
                </TActionsBoxItem>
                <TActionsBoxItem
                  icon="long-arrow-down"
                  @click="() => getMachineJoinConfig(item.metadata.id!)"
                >
                  Download Machine Join Config
                </TActionsBoxItem>
                <TActionsBoxItem
                  icon="long-arrow-down"
                  @click="() => openDownloadInstallationMedia(item.metadata.id!)"
                >
                  Download Installation Media
                </TActionsBoxItem>
                <div class="my-0.5 w-full border-b border-naturals-n5" />
                <TActionsBoxItem
                  v-if="!item.spec.is_default"
                  icon="check"
                  @click="() => makeDefault(item.metadata.id!)"
                >
                  Make Default
                </TActionsBoxItem>

                <TActionsBoxItem
                  icon="error"
                  danger
                  @click="() => openRevokeToken(item.metadata.id!)"
                >
                  Revoke
                </TActionsBoxItem>
              </template>
              <template v-else>
                <TActionsBoxItem icon="reset" @click="unrevokeJoinToken(item.metadata.id!)">
                  Unrevoke
                </TActionsBoxItem>
                <TActionsBoxItem
                  icon="delete"
                  danger
                  @click="() => openDeleteToken(item.metadata.id!)"
                >
                  Delete
                </TActionsBoxItem>
              </template>
            </TActionsBox>
          </div>
        </TListItem>
      </template>
    </TList>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.tokens-grid {
  @apply grid grid-cols-5 items-center gap-4 pr-10;
}

.tokens-header {
  @apply mb-1 bg-naturals-n2 px-3 py-2 pr-12;
}

.tokens-header > * {
  @apply text-xs;
}
</style>
