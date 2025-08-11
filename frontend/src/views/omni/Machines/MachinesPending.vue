<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import {
  InfraMachineType,
  InfraProviderNamespace,
  LabelInfraProviderID,
  LabelMachinePendingAccept,
} from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatsItem from '@/components/common/Stats/StatsItem.vue'

const router = useRouter()

const watchOpts = computed<WatchOptions>(() => {
  return {
    runtime: Runtime.Omni,
    resource: {
      type: InfraMachineType,
      namespace: InfraProviderNamespace,
    },
    selectors: [`${LabelMachinePendingAccept}`],
  }
})

const acceptMachine = (item: Resource) => {
  router.push({
    query: { modal: 'machineAccept', machine: item.metadata.id },
  })
}

const rejectMachine = (item: Resource) => {
  router.push({
    query: { modal: 'machineReject', machine: item.metadata.id },
  })
}
</script>

<template>
  <div>
    <TList :opts="watchOpts" search pagination>
      <template #header="{ itemsCount, filtered }">
        <div class="flex gap-4">
          <PageHeader title="Pending Machines">
            <StatsItem
              pluralized-text="Machine"
              :count="itemsCount"
              icon="nodes"
              :text="filtered ? ' Found' : ' Total'"
            />
          </PageHeader>
        </div>
      </template>
      <template #default="{ items, searchQuery }">
        <div class="header">
          <p>ID</p>
          <p>Provider</p>
        </div>
        <TListItem v-for="item in items" :key="itemID(item)">
          <div class="flex items-center gap-2">
            <div class="grid flex-1 grid-cols-2 gap-2">
              <WordHighlighter
                :query="searchQuery"
                :text-to-highlight="item.metadata.id"
                highlight-class="bg-naturals-N14"
              />
              <span>{{ item.metadata.labels?.[LabelInfraProviderID] }}</span>
            </div>
            <TButton icon="check" type="highlighted" @click="() => acceptMachine(item)"
              >Accept</TButton
            >
            <TButton icon="close" @click="() => rejectMachine(item)">Reject</TButton>
          </div>
        </TListItem>
      </template>
    </TList>
  </div>
</template>

<style scoped>
.header {
  @apply grid grid-cols-2 gap-2 bg-naturals-N2 p-2 pl-3 pr-56 text-xs text-naturals-N13;
}
</style>
