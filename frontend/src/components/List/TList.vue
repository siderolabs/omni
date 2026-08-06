<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T = unknown">
import { watch } from 'vue'

import Pagination from '@/components/Pagination/Pagination.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourcePagination } from '@/methods/resource/useResourcePagination'
import { type ResourceFilterOption, useResourceSearch } from '@/methods/resource/useResourceSearch'
import { type ResourceSortOption, useResourceSort } from '@/methods/resource/useResourceSort'
import { useResourceWatch, type WatchOptionsMulti } from '@/methods/useResourceWatch'

const emit = defineEmits<{
  filterChanged: [string | undefined]
}>()

const {
  pagination,
  search,
  opts,
  sortOptions = [],
  filterOptions = [],
  filterCaption = 'Filter',
} = defineProps<{
  pagination?: boolean
  search?: boolean
  // type: T is the only way to type the generic
  opts: WatchOptionsMulti & { type: T }
  sortOptions?: ResourceSortOption[]
  filterOptions?: ResourceFilterOption[]
  filterCaption?: string
}>()

const filterValue = defineModel<string>('filterValue', { default: '' })

const {
  watchOptions: sortByState,
  selectValues: sortOptionsVariants,
  selectedValue: selectedSortOption,
} = useResourceSort({ sortOptions: () => sortOptions })

const {
  watchOptions: searchState,
  searchQuery,
  selectedFilterOption,
} = useResourceSearch({ filterValue, filterOptions: () => filterOptions })

const {
  total,
  watchOptions: paginationState,
  currentPage,
  currentPageSize: selectedItemsPerPage,
  pageCount,
  pageSizeSelectValues: itemsPerPage,
} = useResourcePagination({
  resetOn: [() => opts, searchState],
})

watch(selectedFilterOption, () => {
  emit('filterChanged', selectedFilterOption.value)
})

const {
  data: items,
  err,
  loading,
} = useResourceWatch<T>(
  () => ({
    ...opts,
    ...(pagination ? paginationState.value : {}),
    ...searchState.value,
    ...sortByState.value,
    selectors: [...(opts.selectors ?? []), ...(searchState.value.selectors ?? [])],
  }),
  { total },
)

defineExpose({
  items,
  searchQuery,
  currentPage,
})
</script>

<template>
  <div class="flex max-w-full flex-col gap-2">
    <slot
      name="header"
      :items-count="total"
      :filtered="searchState.searchFor?.length || searchState.selectors?.length"
    />

    <div class="flex grow flex-col gap-4 overflow-hidden">
      <template v-if="pagination || search">
        <slot name="input">
          <TInput v-if="search" v-model="filterValue" icon="search" />
        </slot>

        <div class="flex justify-between gap-2">
          <div class="grow">
            <slot name="extra-controls" :selected-filter-option />
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <TSelectList
              v-if="filterOptions.length"
              v-model="selectedFilterOption"
              :title="filterCaption"
              :values="filterOptions"
            />

            <TSelectList
              v-if="sortOptions.length"
              v-model="selectedSortOption"
              title="Sort by"
              hide-selected-small-screens
              :values="sortOptionsVariants"
            />

            <TSelectList
              v-if="pagination"
              v-model="selectedItemsPerPage"
              title="Items per Page"
              :values="itemsPerPage"
            />
          </div>
        </div>
      </template>

      <div class="grow overflow-auto">
        <div v-if="loading" class="flex size-full flex-row items-center justify-center">
          <TSpinner class="absolute top-2/4 size-6" />
        </div>

        <slot v-else-if="err" name="error" :err="err">
          <TAlert title="Failed to Fetch Data" type="error">{{ err }}.</TAlert>
        </slot>

        <slot v-else-if="items.length === 0" name="norecords">
          <TAlert type="info" title="No Records">
            No entries of the requested resource type are found on the server.
          </TAlert>
        </slot>

        <div v-show="!loading && !err && items.length > 0" class="size-full">
          <slot :items="items" :search-query="searchQuery" />
        </div>
      </div>
    </div>

    <Pagination v-if="pagination" v-model:current-page="currentPage" :page-count="pageCount" />
  </div>
</template>
