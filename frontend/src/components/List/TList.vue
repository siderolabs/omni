<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T = unknown">
import { useRouteQuery } from '@vueuse/router'
import { computed, watch } from 'vue'
import { useRoute } from 'vue-router'

import Pagination from '@/components/Pagination/Pagination.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourcePagination } from '@/methods/resource/useResourcePagination'
import { type ResourceSortOption, useResourceSort } from '@/methods/resource/useResourceSort'
import {
  useResourceWatch,
  type WatchOptions,
  type WatchOptionsMulti,
} from '@/methods/useResourceWatch'

const emit = defineEmits<{
  filterChanged: [string | undefined]
}>()

const {
  pagination,
  search,
  opts,
  sortOptions = [],
  filterOptions,
  filterCaption,
} = defineProps<{
  pagination?: boolean
  search?: boolean
  // type: T is the only way to type the generic
  opts: WatchOptionsMulti & { type: T }
  sortOptions?: ResourceSortOption[]
  filterOptions?: { query?: string; desc: string }[]
  filterCaption?: string
}>()

const {
  watchOptions: sortByState,
  selectValues: sortOptionsVariants,
  selectedValue: selectedSortOption,
} = useResourceSort({ sortOptions: () => sortOptions })

const {
  total,
  watchOptions: paginationState,
  currentPage,
  currentPageSize: selectedItemsPerPage,
  pageCount,
  pageSizeSelectValues: itemsPerPage,
} = useResourcePagination()

const filterOptionsVariants = computed(() => {
  if (!filterOptions) {
    return []
  }

  return filterOptions.map((opt) => {
    return opt.desc
  })
})

const route = useRoute()
const filterValue = defineModel<string>('filterValue', { default: '' })
const filterValueRoute = useRouteQuery<string>('q', '')

watch(
  filterValueRoute,
  (value) => {
    // No `q` param means the URL has no opinion
    if (route.query.q === undefined) return

    if (filterValue.value !== value) filterValue.value = value
  },
  { immediate: true },
)

watch(filterValue, (value) => {
  if (filterValueRoute.value !== value) filterValueRoute.value = value
})

const selectedFilterOption = useRouteQuery('filter', filterOptionsVariants.value?.[0])

watch(selectedFilterOption, () => {
  emit('filterChanged', selectedFilterOption.value)
})

const searchState = computed<Pick<WatchOptions, 'searchFor' | 'selectors'>>(() => {
  if (!search) {
    return {}
  }

  const parts = filterValue.value.split(' ')
  const selectors: string[] = []
  const searchFor: string[] = []

  if (selectedFilterOption.value) {
    const selectedOptionQuery = filterOptions?.find(
      (item) => item.desc === selectedFilterOption.value,
    )?.query

    if (selectedOptionQuery) {
      searchFor.push(selectedOptionQuery)
    }
  }

  for (const part of parts) {
    const match = part.match(/^(.+):(.*)$/)

    if (!match || match.length < 3) {
      if (part) searchFor.push(part)

      continue
    }

    selectors.push(`${match[1]}=${match[2]}`)
  }

  const res: { selectors?: string[]; searchFor?: string[] } = {
    selectors: (opts.selectors ?? []).concat(selectors),
  }

  if (searchFor.length > 0) {
    res.searchFor = searchFor
  }

  return res
})

// reset the pagination when the search query changes
watch([() => opts, searchState, selectedItemsPerPage], (curr, prev) => {
  if (JSON.stringify(curr) !== JSON.stringify(prev)) currentPage.value = 1
})

const searchQuery = computed(() => {
  if (!searchState.value.searchFor) {
    return undefined
  }

  return searchState.value.searchFor.join(' ')
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
      <template v-if="pagination || search || (pagination && itemsPerPage.length > 1)">
        <slot name="input">
          <TInput v-if="search" v-model="filterValue" icon="search" />
        </slot>

        <div class="flex justify-between gap-2">
          <div class="grow">
            <slot name="extra-controls" :selected-filter-option />
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <TSelectList
              v-if="filterOptions"
              :title="filterCaption ?? 'Filter'"
              :default-value="selectedFilterOption || ''"
              :values="filterOptionsVariants"
              @checked-value="(value) => (selectedFilterOption = value)"
            />

            <TSelectList
              v-if="sortOptions.length"
              v-model="selectedSortOption"
              title="Sort by"
              hide-selected-small-screens
              :values="sortOptionsVariants"
            />

            <TSelectList
              v-if="itemsPerPage.length > 1 && pagination"
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
