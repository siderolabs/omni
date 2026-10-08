<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import prettyBytes from 'pretty-bytes'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type {
  EtcdBackupOverallStatusSpec,
  EtcdBackupSpec,
  EtcdBackupStatusSpec,
} from '@/api/omni/specs/omni.pb'
import {
  EtcdBackupOverallStatusID,
  EtcdBackupOverallStatusType,
  EtcdBackupStatusType,
  EtcdBackupType,
  ExternalNamespace,
  LabelCluster,
  MetricsNamespace,
} from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import TList from '@/components/List/TList.vue'
import TListItem from '@/components/List/TListItem.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { formatISO } from '@/methods/time'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { clusterId } = defineProps<{
  clusterId: string
}>()

const dateFormat = 'HH:mm MMM d y'
const { copy } = useClipboard()
const { canManageBackupStore } = usePermissions()

const {
  data: etcdBackupOverallStatus,
  loading: etcdBackupOverallStatusLoading,
  err: etcdBackupOverallStatusErr,
} = useResourceWatch<EtcdBackupOverallStatusSpec>({
  resource: {
    namespace: MetricsNamespace,
    type: EtcdBackupOverallStatusType,
    id: EtcdBackupOverallStatusID,
  },
  runtime: Runtime.Omni,
})

const {
  data: etcdBackupStatus,
  loading: etcdBackupStatusLoading,
  err: etcdBackupStatusErr,
} = useResourceWatch<EtcdBackupStatusSpec>(() => ({
  skip: !etcdBackupOverallStatus.value || !!etcdBackupOverallStatus.value.spec.configuration_error,
  resource: {
    namespace: MetricsNamespace,
    type: EtcdBackupStatusType,
    id: clusterId,
  },
  runtime: Runtime.Omni,
}))
</script>

<template>
  <div
    v-if="etcdBackupOverallStatusLoading || etcdBackupStatusLoading"
    class="flex size-full items-center justify-center"
  >
    <TSpinner class="size-6" />
  </div>

  <TAlert
    v-else-if="etcdBackupOverallStatusErr || etcdBackupStatusErr"
    title="Failed to Fetch Data"
    type="error"
  >
    {{ etcdBackupOverallStatusErr || etcdBackupStatusErr }}.
  </TAlert>

  <TAlert v-else-if="!etcdBackupOverallStatus" type="info" title="No Records">
    No entries of the requested resource type are found on the server.
  </TAlert>

  <TAlert
    v-else-if="etcdBackupOverallStatus.spec.configuration_error"
    type="warn"
    :title="`The backups storage is not properly configured: ${etcdBackupOverallStatus.spec.configuration_error}`"
  >
    <div class="flex gap-micro">
      Check the
      <TButton
        is="a"
        variant="subtle"
        size="xs"
        :href="getDocsLink('omni', '/cluster-management/etcd-backups#s3-configuration')"
        target="_blank"
        rel="noopener noreferrer"
      >
        documentation
      </TButton>
      on how to configure s3 backups using CLI.
    </div>
    <div v-if="canManageBackupStore" class="flex gap-micro">
      Or
      <TButton is="router-link" variant="subtle" size="xs" :to="{ name: 'BackupStorage' }">
        configure backups in the UI.
      </TButton>
    </div>
  </TAlert>

  <TAlert v-else-if="!etcdBackupStatus" type="info" title="No Records">
    No entries of the requested resource type are found on the server.
  </TAlert>

  <template v-else>
    <TAlert
      v-if="etcdBackupStatus.spec.error"
      type="warn"
      title="There was an issue creating the backup"
      class="mb-compact"
    >
      {{ etcdBackupStatus.spec.error }}
    </TAlert>

    <TList
      :key="etcdBackupStatus?.metadata.updated"
      :opts="{
        type: undefined as unknown as EtcdBackupSpec,
        resource: {
          namespace: ExternalNamespace,
          type: EtcdBackupType,
        },
        runtime: Runtime.Omni,
        selectors: [`${LabelCluster}=${clusterId}`],
      }"
      search
      :sort-options="[
        { id: 'id', desc: 'Creation Time ⬇', descending: true },
        { id: 'id', desc: 'Creation Time ⬆' },
      ]"
    >
      <template #default="{ items, searchQuery }">
        <div class="mb-micro bg-surface-card px-base py-tight pl-10 text-xs">
          <div class="grid grid-cols-4 items-center justify-center gap-micro pr-12">
            <div>ID</div>
            <div>Creation Date</div>
            <div>Size</div>
            <div>Snapshot ID</div>
          </div>
        </div>

        <TListItem v-for="item in items" :key="item.metadata.id!">
          <div class="relative pr-snug pl-7 text-content-default">
            <div class="grid grid-cols-4 items-center justify-center gap-micro pr-12">
              <WordHighlighter
                :query="searchQuery"
                :text-to-highlight="item.metadata.id"
                highlight-class="search-match"
              />
              <div class="text-content-emphasis">
                {{ formatISO(item.spec.created_at as string, dateFormat) }}
              </div>
              <div class="text-content-emphasis">
                {{ prettyBytes(parseInt(item.spec.size ?? '0')) }}
              </div>
              <div class="flex items-center gap-tight text-content-emphasis">
                {{ item.spec.snapshot }}
                <IconButton icon="copy" @click="copy(item.spec.snapshot ?? '')" />
              </div>
            </div>
          </div>
        </TListItem>
      </template>
    </TList>
  </template>
</template>
