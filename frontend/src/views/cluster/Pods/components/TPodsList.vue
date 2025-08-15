<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Pod as V1Pod } from 'kubernetes-types/core/v1'
import { computed, toRefs } from 'vue'

import TPagination from '@/components/common/Pagination/TPagination.vue'
import { TPodsViewFilterOptions } from '@/constants'
import TPodsItem from '@/views/cluster/Pods/components/TPodsItem.vue'

type Props = {
  items: any
  filterOption: string
  searchOption: string
}

const props = defineProps<Props>()

const PAGINATION_PER_PAGE = 9
const { items, filterOption, searchOption } = toRefs(props)

const filteredItems = computed(() => {
  if (filterOption.value === TPodsViewFilterOptions.ALL && searchOption.value === '') {
    return items?.value
  }

  return items?.value?.filter((elem: V1Pod) => {
    if (
      filterOption.value !== TPodsViewFilterOptions.ALL &&
      elem?.status?.phase !== filterOption.value
    ) {
      return false
    }

    if (searchOption.value === '') {
      return true
    }

    const searchOptions = [elem?.metadata?.name, elem?.metadata?.namespace, elem?.spec?.nodeName]

    for (const value of searchOptions) {
      if (value?.includes(searchOption.value)) {
        return true
      }
    }

    return false
  })
})
</script>

<template>
  <div class="list__wrapper">
    <TPagination
      :items="filteredItems"
      :per-page="PAGINATION_PER_PAGE"
      :search-option="searchOption"
    >
      <template #default="{ paginatedItems }">
        <div class="list">
          <TPodsItem
            v-for="(item, idx) in paginatedItems"
            :key="item?.metadata?.namespace + '/' + item?.metadata?.name || idx"
            :search-option="searchOption"
            :item="item"
          />
        </div>
      </template>
    </TPagination>
  </div>
</template>

<style scoped>
@reference "../../../../index.css";

.list {
  @apply grow overflow-visible;
}
.list__wrapper {
  @apply flex h-full flex-col;
}
</style>
