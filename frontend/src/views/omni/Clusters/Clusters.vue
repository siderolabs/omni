<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import {
  ClusterStatusMetricsID,
  ClusterStatusMetricsType,
  ClusterStatusType,
  DefaultNamespace,
  EphemeralNamespace,
  LabelsCompletionType,
  VirtualNamespace,
} from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatsItem from '@/components/common/Stats/StatsItem.vue'
import Watch from '@/components/common/Watch/Watch.vue'
import { canCreateClusters } from '@/methods/auth'
import type { Label } from '@/methods/labels'
import { addLabel, selectors } from '@/methods/labels'
import ClusterItem from '@/views/omni/Clusters/ClusterItem.vue'

import LabelsInput from '../ItemLabels/LabelsInput.vue'

const router = useRouter()

const watchOpts = computed<WatchOptions>(() => {
  return {
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ClusterStatusType,
    },
    selectors: selectors(filterLabels.value),
    sortByField: 'created',
  }
})

const filterValue = ref('')
const filterLabels = ref<Label[]>([])

const openClusterCreate = () => {
  router.push({ name: 'ClusterCreate' })
}

const sortOptions = [
  { id: 'id', desc: 'ID ⬆' },
  { id: 'id', desc: 'ID ⬇', descending: true },
  { id: 'created', desc: 'Creation Time ⬆', descending: true },
  { id: 'created', desc: 'Creation Time ⬇' },
]

const filterOptions = [
  { desc: 'All' },
  { desc: 'Ready', query: 'ready' },
  { desc: 'Not Ready', query: '!ready' },
]
</script>

<template>
  <div>
    <TList
      :opts="watchOpts"
      search
      no-records-alert
      pagination
      errors-alert
      filter-caption="Status"
      :sort-options="sortOptions"
      :filter-options="filterOptions"
      :filter-value="filterValue"
    >
      <template #header="{ itemsCount }">
        <div class="flex items-start gap-1">
          <PageHeader title="Clusters" class="flex-1">
            <StatsItem title="Clusters" :value="itemsCount" icon="clusters" />
            <Watch
              :opts="{
                resource: {
                  type: ClusterStatusMetricsType,
                  id: ClusterStatusMetricsID,
                  namespace: EphemeralNamespace,
                },
                runtime: Runtime.Omni,
              }"
            >
              <template #default="{ data }">
                <StatsItem
                  v-if="data?.spec.not_ready_count"
                  title="Not Ready"
                  :value="data.spec.not_ready_count"
                  icon="warning"
                />
              </template>
            </Watch>
          </PageHeader>
          <TButton :disabled="!canCreateClusters" type="highlighted" @click="openClusterCreate">
            Create Cluster
          </TButton>
        </div>
      </template>
      <template #input>
        <LabelsInput
          v-model:filter-labels="filterLabels"
          v-model:filter-value="filterValue"
          :completions-resource="{
            id: ClusterStatusType,
            type: LabelsCompletionType,
            namespace: VirtualNamespace,
          }"
          class="w-full"
        />
      </template>
      <template #default="{ items, searchQuery }">
        <div class="flex flex-col gap-2">
          <div class="max-lg:hidden">
            <div class="clusters-header">
              <div class="clusters-grid">
                <div class="pl-6">Name</div>
                <div class="pl-6">Machines Healthy</div>
                <div class="pl-6">Phase</div>
                <div class="pl-5">Labels</div>
              </div>
              <div>Actions</div>
            </div>
          </div>
          <ClusterItem
            v-for="(item, index) in items"
            :key="itemID(item)"
            :default-open="index === 0"
            :search-query="searchQuery"
            :item="item"
            @filter-labels="(label) => addLabel(filterLabels, label)"
          />
        </div>
      </template>
    </TList>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.clusters-grid {
  @apply grid flex-1 grid-cols-4 pr-2;
}

.clusters-header {
  @apply mb-1 flex items-center bg-naturals-n2 px-3 py-2.5;
}

.clusters-header > * {
  @apply text-xs;
}
</style>
