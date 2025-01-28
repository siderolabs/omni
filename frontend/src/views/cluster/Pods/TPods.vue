<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col">
    <page-header title="All Pods"/>
    <watch :opts="{ resource: { type: kubernetes.pod }, runtime: Runtime.Kubernetes, context }" class="flex-1" noRecordsAlert errorsAlert spinner>
      <template #default="{ items }">
        <div class="pods">
          <div class="pods__search-box">
            <t-input
              secondary
              placeholder="Search..."
              v-model="inputValue"
            />
            <t-select-list
              @checkedValue="setFilterOption"
              title="Phase"
              :defaultValue="TPodsViewFilterOptions.ALL"
              :values="filterOptions"
            />
          </div>
          <ul class="pods__table-heading">
            <li class="pods__row-name">Namespace</li>
            <li class="pods__row-name">Name</li>
            <li class="pods__row-name">Phase</li>
            <li class="pods__row-name">Node</li>
          </ul>
          <t-pods-list
            :items="items"
            :filterOption="filterOption"
            :searchOption="inputValue"
          />
        </div>
      </template>
    </watch>
  </div>
</template>

<script setup lang="ts">
import { getContext } from "@/context";
import { kubernetes } from "@/api/resources";
import { ref } from "vue";
import { TPodsViewFilterOptions } from "@/constants";
import { Runtime } from "@/api/common/omni.pb";

import TInput from "@/components/common/TInput/TInput.vue";
import Watch from "@/components/common/Watch/Watch.vue";
import TPodsList from "@/views/cluster/Pods/components/TPodsList.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import PageHeader from "@/components/common/PageHeader.vue";

const context = getContext();
const filterOption = ref(TPodsViewFilterOptions.ALL);
const inputValue = ref("");

const setFilterOption = (data: TPodsViewFilterOptions) => {
  filterOption.value = data;
};

const filterOptions = Object.keys(TPodsViewFilterOptions).map(key => TPodsViewFilterOptions[key]);
</script>

<style scoped>
.pods {
  @apply flex flex-col h-full pt-2;
}
.pods__search-box {
  @apply flex justify-between mb-3;
}
.pods__table-heading {
  @apply w-full rounded bg-naturals-N2 flex justify-between items-center mb-1;
  padding: 10px 33px;
}
.pods__row-name {
  @apply text-xs text-naturals-N13;
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
