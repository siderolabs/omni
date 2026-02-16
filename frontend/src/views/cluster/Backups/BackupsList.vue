<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { useRoute } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { EtcdBackupOverallStatusSpec, EtcdBackupStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  EtcdBackupOverallStatusID,
  EtcdBackupOverallStatusType,
  EtcdBackupStatusType,
  EtcdBackupType,
  ExternalNamespace,
  LabelCluster,
  MetricsNamespace,
} from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { formatBytes, getDocsLink } from '@/methods'
import { canManageBackupStore } from '@/methods/auth'
import { formatISO } from '@/methods/time'
import { useResourceWatch } from '@/methods/useResourceWatch'

const dateFormat = 'HH:mm MMM d y'
const route = useRoute()
const { copy } = useClipboard()

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
    id: route.params.cluster.toString(),
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
    <div class="flex gap-1">
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
    <div v-if="canManageBackupStore" class="flex gap-1">
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
      class="mb-4"
    >
      {{ etcdBackupStatus.spec.error }}
    </TAlert>

    <TList
      :key="etcdBackupStatus?.metadata.updated"
      :opts="{
        resource: {
          namespace: ExternalNamespace,
          type: EtcdBackupType,
        },
        runtime: Runtime.Omni,
        selectors: [`${LabelCluster}=${route.params.cluster}`],
      }"
      search
      :sort-options="[
        { id: 'id', desc: 'Creation Time ⬇', descending: true },
        { id: 'id', desc: 'Creation Time ⬆' },
      ]"
    >
      <template #default="{ items, searchQuery }">
        <div class="mb-1 bg-naturals-n2 px-6 py-2 pl-10 text-xs">
          <div class="grid grid-cols-4 items-center justify-center gap-1 pr-12">
            <div>ID</div>
            <div>Creation Date</div>
            <div>Size</div>
            <div>Snapshot ID</div>
          </div>
        </div>

        <TListItem v-for="item in items" :key="item.metadata.id!">
          <div class="relative pr-3 text-naturals-n12" :class="{ 'pl-7': !item.spec.description }">
            <div class="grid grid-cols-4 items-center justify-center gap-1 pr-12">
              <WordHighlighter
                :query="searchQuery"
                :text-to-highlight="item.metadata.id"
                highlight-class="bg-naturals-n14"
              />
              <div class="text-naturals-n14">
                {{ formatISO(item.spec.created_at as string, dateFormat) }}
              </div>
              <div class="text-naturals-n14">
                {{ formatBytes(parseInt(item.spec.size ?? '0')) }}
              </div>
              <div class="flex items-center gap-2 text-naturals-n14">
                {{ item.spec.snapshot }}
                <IconButton icon="copy" @click="copy(item.spec.snapshot)" />
              </div>
            </div>
          </div>
        </TListItem>
      </template>
    </TList>
  </template>
</template>
