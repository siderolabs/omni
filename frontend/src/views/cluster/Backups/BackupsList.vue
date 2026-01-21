<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
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
import Watch from '@/components/common/Watch/Watch.vue'
import TAlert from '@/components/TAlert.vue'
import { formatBytes, getDocsLink } from '@/methods'
import { canManageBackupStore } from '@/methods/auth'
import { formatISO } from '@/methods/time'

const dateFormat = 'HH:mm MMM d y'
const route = useRoute()
const { copy } = useClipboard()

const sortOptions = [
  { id: 'id', desc: 'Creation Time ⬇', descending: true },
  { id: 'id', desc: 'Creation Time ⬆' },
]

const watchStatusOpts = computed(() => {
  return {
    resource: {
      namespace: MetricsNamespace,
      type: EtcdBackupStatusType,
      id: route.params.cluster as string,
    },
    runtime: Runtime.Omni,
  }
})

const watchOverallStatusOpts = computed(() => {
  return {
    resource: {
      namespace: MetricsNamespace,
      type: EtcdBackupOverallStatusType,
      id: EtcdBackupOverallStatusID,
    },
    runtime: Runtime.Omni,
  }
})

const watchOpts = computed(() => {
  return {
    resource: {
      namespace: ExternalNamespace,
      type: EtcdBackupType,
    },
    runtime: Runtime.Omni,
    selectors: [`${LabelCluster}=${route.params.cluster}`],
  }
})

const docsLink = getDocsLink('omni', '/how-to-guides/etcd-backups#s3-configuration')
const openDocs = () => window.open(docsLink, '_blank')?.focus()
</script>

<template>
  <Watch :opts="watchOverallStatusOpts" spinner no-records-alert errors-alert>
    <template #default="{ data }">
      <TAlert
        v-if="data?.spec?.configuration_error"
        type="warn"
        :title="`The backups storage is not properly configured: ${data.spec?.configuration_error}`"
      >
        <div class="flex gap-1">
          Check the
          <TButton type="subtle" size="xs" @click="openDocs">documentation</TButton>
          on how to configure s3 backups using CLI.
        </div>
        <div v-if="canManageBackupStore" class="flex gap-1">
          Or
          <TButton type="subtle" size="xs" @click="$router.push({ name: 'BackupStorage' })">
            configure backups in the UI.
          </TButton>
        </div>
      </TAlert>
      <Watch v-else :opts="watchStatusOpts" spinner no-records-alert errors-alert>
        <template #default="{ data }">
          <TList
            :key="data?.metadata?.updated"
            :opts="watchOpts"
            search
            :sort-options="sortOptions"
          >
            <template #default="{ items, searchQuery }">
              <div class="header">
                <div class="list-grid">
                  <div>ID</div>
                  <div>Creation Date</div>
                  <div>Size</div>
                  <div>Snapshot ID</div>
                </div>
              </div>
              <TListItem v-for="item in items" :key="item.metadata.id!">
                <div
                  class="relative pr-3 text-naturals-n12"
                  :class="{ 'pl-7': !item.spec.description }"
                >
                  <div class="list-grid">
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
      </Watch>
    </template>
  </Watch>
</template>

<style scoped>
@reference "../../../index.css";

.header {
  @apply mb-1 bg-naturals-n2 px-6 py-2 pl-10 text-xs;
}

.list-grid {
  @apply grid grid-cols-4 items-center justify-center gap-1 pr-12;
}
</style>
