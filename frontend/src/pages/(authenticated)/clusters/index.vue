<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterStatusMetricsSpec, ClusterStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterStatusMetricsID,
  ClusterStatusMetricsType,
  ClusterStatusType,
  DefaultNamespace,
  EphemeralNamespace,
  LabelsCompletionType,
  VirtualNamespace,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TList from '@/components/List/TList.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import StatsItem from '@/components/Stats/StatsItem.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { addLabel, selectors, useLabelRouteQuery } from '@/methods/labels'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ClusterItem from '@/views/Clusters/ClusterItem.vue'
import LabelsInput from '@/views/ItemLabels/LabelsInput.vue'

definePage({ name: 'Clusters' })

const { canCreateClusters } = usePermissions()

const { data } = useResourceWatch<ClusterStatusMetricsSpec>({
  resource: {
    type: ClusterStatusMetricsType,
    id: ClusterStatusMetricsID,
    namespace: EphemeralNamespace,
  },
  runtime: Runtime.Omni,
})

const filterLabels = useLabelRouteQuery()
const filterValue = useRouteQuery('q', '')

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
  <PageContainer>
    <TList
      :opts="{
        type: undefined as unknown as ClusterStatusSpec,
        runtime: Runtime.Omni,
        resource: {
          namespace: DefaultNamespace,
          type: ClusterStatusType,
        },
        selectors: selectors(filterLabels),
        sortByField: 'created',
      }"
      search
      no-records-alert
      pagination
      errors-alert
      filter-caption="Status"
      :sort-options="sortOptions"
      :filter-options="filterOptions"
      :filter-value="filterValue"
    >
      <template #norecords>
        <TAlert type="info" title="No clusters found">
          To create your first cluster, click "Create Cluster" above or
          <RouterLink class="link-primary" :to="{ name: 'ClusterCreate' }">here</RouterLink>
          to get started. You can check out our
          <a
            target="_blank"
            rel="noopener noreferrer"
            :href="getDocsLink('omni', '/getting-started/create-a-cluster')"
            class="link-primary"
          >
            documentation
          </a>
          for more information.
        </TAlert>
      </template>

      <template #header="{ itemsCount }">
        <div class="flex items-start gap-1">
          <PageHeader title="Clusters" class="flex-1">
            <StatsItem title="Clusters" :value="itemsCount" icon="clusters" />
            <StatsItem
              v-if="data?.spec.not_ready_count"
              title="Not Ready"
              :value="data.spec.not_ready_count"
              icon="warning"
            />
          </PageHeader>
          <TButton
            is="router-link"
            :disabled="!canCreateClusters"
            variant="highlighted"
            :to="{ name: 'ClusterCreate' }"
          >
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
        <div class="grid grid-cols-[repeat(4,1fr)_--spacing(24)] gap-3">
          <div
            class="col-span-full grid grid-cols-subgrid bg-naturals-n2 px-3 py-2.5 text-xs max-lg:hidden"
          >
            <div class="pl-6">Name</div>
            <div>Machines Healthy</div>
            <div>Phase</div>
            <div>Versions</div>
            <div>Actions</div>
          </div>

          <ul class="col-span-full grid grid-cols-subgrid gap-3">
            <ClusterItem
              v-for="(item, index) in items"
              :key="item.metadata.id"
              :default-open="index === 0"
              :search-query="searchQuery"
              :item="item"
              @filter-labels="(label) => (filterLabels = addLabel(filterLabels, label))"
            />
          </ul>
        </div>
      </template>
    </TList>
  </PageContainer>
</template>
