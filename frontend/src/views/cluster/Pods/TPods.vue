<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { PodSpec as V1PodSpec, PodStatus as V1PodStatus } from 'kubernetes-types/core/v1'
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { kubernetes } from '@/api/resources'
import PageHeader from '@/components/common/PageHeader.vue'
import TPagination from '@/components/common/Pagination/TPagination.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import TAlert from '@/components/TAlert.vue'
import { TPodsViewFilterOptions } from '@/constants'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'
import TPodsItem from '@/views/cluster/Pods/components/TPodsItem.vue'

const context = getContext()
const filterOption = ref(TPodsViewFilterOptions.ALL)
const searchOption = ref('')

const filterOptions = Object.values(TPodsViewFilterOptions)

const {
  data: items,
  loading,
  err,
} = useResourceWatch<V1PodSpec, V1PodStatus>({
  resource: { type: kubernetes.pod },
  runtime: Runtime.Kubernetes,
  context,
})

const filteredItems = computed(() =>
  items.value.filter(({ spec, metadata, status }) => {
    if (filterOption.value !== TPodsViewFilterOptions.ALL && status?.phase !== filterOption.value) {
      return false
    }

    if (!searchOption.value) return true

    return [metadata.name, metadata.namespace, spec.nodeName]
      .filter((o) => typeof o === 'string')
      .some((o) => o.includes(searchOption.value))
  }),
)
</script>

<template>
  <div class="flex flex-col">
    <PageHeader title="All Pods" />

    <TSpinner v-if="loading" class="size-6 self-center" />
    <TAlert v-else-if="err" title="Failed to Fetch Data" type="error">{{ err }}.</TAlert>
    <TAlert v-else-if="!items.length" type="info" title="No Records">
      No entries of the requested resource type are found on the server.
    </TAlert>

    <div v-else class="pt-2">
      <div class="mb-3 flex justify-between">
        <TInput v-model="searchOption" secondary placeholder="Search..." />
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
  </div>
</template>
