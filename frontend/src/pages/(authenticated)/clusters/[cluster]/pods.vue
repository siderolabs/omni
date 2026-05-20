<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { PodSpec as V1PodSpec, PodStatus as V1PodStatus } from 'kubernetes-types/core/v1'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { kubernetes } from '@/api/resources'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TPagination from '@/components/Pagination/TPagination.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { TPodsViewFilterOptions } from '@/constants'
import { useResourceWatch } from '@/methods/useResourceWatch'
import TPodsItem from '@/views/Pods/TPodsItem.vue'

definePage({ name: 'Pods' })

const route = useRoute()
const filterOption = ref(TPodsViewFilterOptions.ALL)
const searchOption = ref('')

const filterOptions = Object.values(TPodsViewFilterOptions)

const {
  data: items,
  loading,
  err,
} = useResourceWatch<V1PodSpec, V1PodStatus>(() => ({
  resource: { type: kubernetes.pod },
  runtime: Runtime.Kubernetes,
  context: {
    cluster: route.params.cluster,
  },
}))

function phaseToNum(phase?: string) {
  switch (phase) {
    case 'Failed':
      return 1
    case 'Pending':
      return 2
    case 'Succeeded':
      return 3
    case 'Running':
      return 4
    default:
      return 0
  }
}

const filteredItems = computed(() =>
  items.value
    .filter(({ spec, metadata, status }) => {
      if (
        filterOption.value !== TPodsViewFilterOptions.ALL &&
        status?.phase !== filterOption.value
      ) {
        return false
      }

      if (!searchOption.value) return true

      return [metadata.name, metadata.namespace, spec.nodeName]
        .filter((o) => typeof o === 'string')
        .some((o) => o.includes(searchOption.value))
    })
    .toSorted((a, b) => phaseToNum(a.status?.phase) - phaseToNum(b.status?.phase)),
)
</script>

<template>
  <PageContainer class="flex flex-col">
    <PageHeader title="All Pods" />

    <TSpinner v-if="loading" class="size-6 self-center" />
    <TAlert v-else-if="err" title="Failed to Fetch Data" type="error">{{ err }}.</TAlert>
    <TAlert v-else-if="!items.length" type="info" title="No Records">
      No entries of the requested resource type are found on the server.
    </TAlert>

    <div v-else class="pt-2">
      <div class="mb-3 flex gap-4">
        <TInput v-model="searchOption" icon="search" placeholder="Search..." class="w-full" />
        <TSelectList
          title="Phase"
          :default-value="TPodsViewFilterOptions.ALL"
          :values="filterOptions"
          @checked-value="filterOption = $event"
        />
      </div>

      <ul class="mb-1 flex rounded bg-naturals-n2 px-8 py-2.5 text-xs text-naturals-n13">
        <li class="w-1/6">Namespace</li>
        <li class="w-1/3">Name</li>
        <li class="w-1/6">Phase</li>
        <li class="w-1/3">Node</li>
      </ul>

      <TPagination :items="filteredItems" :per-page="9" :search-option="searchOption">
        <template #default="{ paginatedItems }">
          <div>
            <TPodsItem
              v-for="(item, idx) in paginatedItems"
              :key="`${item.metadata.namespace}/${item.metadata.name || idx}`"
              :search-option="searchOption"
              :item="item"
            />
          </div>
        </template>
      </TPagination>
    </div>
  </PageContainer>
</template>
