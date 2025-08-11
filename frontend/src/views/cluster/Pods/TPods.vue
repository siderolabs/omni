<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { kubernetes } from '@/api/resources'
import PageHeader from '@/components/common/PageHeader.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import Watch from '@/components/common/Watch/Watch.vue'
import { TPodsViewFilterOptions } from '@/constants'
import { getContext } from '@/context'
import TPodsList from '@/views/cluster/Pods/components/TPodsList.vue'

const context = getContext()
const filterOption = ref(TPodsViewFilterOptions.ALL)
const inputValue = ref('')

const setFilterOption = (data: TPodsViewFilterOptions) => {
  filterOption.value = data
}

const filterOptions = Object.keys(TPodsViewFilterOptions).map((key) => TPodsViewFilterOptions[key])
</script>

<template>
  <div class="flex flex-col">
    <PageHeader title="All Pods" />
    <Watch
      :opts="{ resource: { type: kubernetes.pod }, runtime: Runtime.Kubernetes, context }"
      class="flex-1"
      no-records-alert
      errors-alert
      spinner
    >
      <template #default="{ items }">
        <div class="pods">
          <div class="pods__search-box">
            <TInput v-model="inputValue" secondary placeholder="Search..." />
            <TSelectList
              title="Phase"
              :default-value="TPodsViewFilterOptions.ALL"
              :values="filterOptions"
              @checked-value="setFilterOption"
            />
          </div>
          <ul class="pods__table-heading">
            <li class="pods__row-name">Namespace</li>
            <li class="pods__row-name">Name</li>
            <li class="pods__row-name">Phase</li>
            <li class="pods__row-name">Node</li>
          </ul>
          <TPodsList :items="items" :filter-option="filterOption" :search-option="inputValue" />
        </div>
      </template>
    </Watch>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.pods {
  @apply flex h-full flex-col pt-2;
}
.pods__search-box {
  @apply mb-3 flex justify-between;
}
.pods__table-heading {
  @apply mb-1 flex w-full items-center justify-between rounded bg-naturals-n2;
  padding: 10px 33px;
}
.pods__row-name {
  @apply text-xs text-naturals-n13;
}
.pods__row-name:nth-child(1) {
  width: 17%;
}
.pods__row-name:nth-child(2) {
  width: 33%;
}
.pods__row-name:nth-child(3) {
  width: 17%;
}
.pods__row-name:nth-child(4) {
  width: 33%;
}
</style>
