<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="list__wrapper">
    <t-pagination
      :items="filteredItems"
      :perPage="PAGINATION_PER_PAGE"
      :searchOption="searchOption"
    >
      <template #default="{ paginatedItems }">
        <div class="list">
          <t-pods-item
            :searchOption="searchOption"
            :item="item"
            v-for="(item, idx) in paginatedItems"
            :key="item?.metadata?.namespace + '/' + item?.metadata?.name || idx"
          />
        </div>
      </template>
    </t-pagination>
  </div>
</template>

<script setup lang="ts">
import { computed, toRefs } from "vue";
import { TPodsViewFilterOptions } from "@/constants";
import { Pod } from "kubernetes-types/core/v1";

import TPodsItem from "@/views/cluster/Pods/components/TPodsItem.vue";
import TPagination from "@/components/common/Pagination/TPagination.vue";

type Props = {
  items: any;
  filterOption: string;
  searchOption: string;
};

const props = defineProps<Props>();

const PAGINATION_PER_PAGE = 9;
const { items, filterOption, searchOption } = toRefs(props);

const filteredItems = computed(() => {
  if (filterOption.value === TPodsViewFilterOptions.ALL && searchOption.value === "") {
    return items?.value;
  }

  return items?.value?.filter((elem: Pod) => {
    if (filterOption.value !== TPodsViewFilterOptions.ALL && elem?.status?.phase !== filterOption.value) {
      return false;
    }

    if (searchOption.value === "") {
      return true;
    }

    const searchOptions = [
      elem?.metadata?.name,
      elem?.metadata?.namespace,
      elem?.spec?.nodeName,
    ];

    for (const value of searchOptions) {
      if (value?.includes(searchOption.value)) {
        return true;
      }
    }

    return false;
  })
});
</script>

<style scoped>
.list {
  overflow: visible;
  flex-grow: 1;
}
.list__wrapper {
  @apply flex flex-col h-full;
}
</style>
