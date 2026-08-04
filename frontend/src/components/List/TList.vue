<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T = unknown">
import { useLocalStorage, useOffsetPagination } from '@vueuse/core'
import { useRouteQuery } from '@vueuse/router'
import { computed, ref, watch } from 'vue'

import Pagination from '@/components/Pagination/Pagination.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import {
  useResourceWatch,
  type WatchOptions,
  type WatchOptionsMulti,
} from '@/methods/useResourceWatch'

defineExpose({
  addFilterLabel: (label: { key: string; value?: string }) => {
    const selector = `${label.key}:${label.value}`
    if (filterValueInternal.value.includes(selector)) {
      return
    }

    filterValueInternal.value += (filterValueInternal.value ? ' ' : '') + selector
  },
})

const emit = defineEmits<{
  filterChanged: [string | undefined]
}>()

const { pagination, search, opts, sortOptions, filterOptions, filterValue, filterCaption } =
  defineProps<{
    pagination?: boolean
    search?: boolean
    // type: T is the only way to type the generic
    opts: WatchOptionsMulti & { type: T }
    sortOptions?: { id: string; desc: string; descending?: boolean }[]
    filterOptions?: { query?: string; desc: string }[]
    filterValue?: string
    filterCaption?: string
  }>()

const itemsPerPage = [5, 10, 25, 50, 100]

const sortOptionsVariants = computed(() => {
  if (!sortOptions) {
    return []
  }

  return sortOptions.map((opt) => {
    return opt.desc
  })
})

const filterOptionsVariants = computed(() => {
  if (!filterOptions) {
    return []
  }

  return filterOptions.map((opt) => {
    return opt.desc
  })
})

const filterValueInternal = useRouteQuery('q', '')
const currentPage = ref(1)
const selectedItemsPerPage = useLocalStorage('itemsPerPage', 10)
const selectedSortOption = useRouteQuery('sort', sortOptionsVariants?.value?.[0])
const selectedFilterOption = useRouteQuery('filter', filterOptionsVariants.value?.[0])
const sidePanelOpen = ref(false)
const sidePanelSelectedItemId = ref<string>()

watch(selectedFilterOption, () => {
  emit('filterChanged', selectedFilterOption.value)
})

const filterValueComputed = computed(() => {
  return filterValue !== undefined ? filterValue : filterValueInternal.value
})

const sortByState = computed(() => {
  if (!sortOptions) {
    return {}
  }

  for (const opt of sortOptions) {
    if (opt.desc === selectedSortOption?.value) {
      return {
        sortByField: opt.id,
        sortDescending: opt.descending,
      }
    }
  }

  return {}
})

const paginationState = computed(() => {
  if (!pagination) {
    return {}
  }

  return {
    limit: selectedItemsPerPage.value,
    offset: (currentPage.value - 1) * selectedItemsPerPage.value,
  }
})

const searchState = computed<Pick<WatchOptions, 'searchFor' | 'selectors'>>(() => {
  if (!search) {
    return {}
  }

  const parts = filterValueComputed.value.split(' ')
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

// Close sidepanel when changing page
watch(currentPage, () => (sidePanelOpen.value = false))

// reset the pagination when the search query changes
watch([() => opts, searchState], (curr, prev) => {
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
  total,
} = useResourceWatch<T>(() => ({
  ...opts,
  ...paginationState.value,
  ...searchState.value,
  ...sortByState.value,
}))

const { pageCount } = useOffsetPagination({
  total,
  pageSize: selectedItemsPerPage,
  page: currentPage,
})
</script>

<template>
  <div class="flex h-full gap-2 overflow-hidden">
    <div class="flex max-w-full grow flex-col gap-2">
      <slot
        name="header"
        :items-count="total"
        :filtered="searchState.searchFor?.length || searchState.selectors?.length"
      />

      <div class="flex grow flex-col gap-4 overflow-hidden">
        <template v-if="pagination || search || (pagination && itemsPerPage?.length > 1)">
          <slot name="input">
            <TInput v-if="search" v-model="filterValueInternal" icon="search" />
          </slot>

          <div class="flex justify-between gap-2">
            <div class="grow">
              <slot name="extra-controls" :selected-filter-option />
            </div>

            <div class="flex items-center gap-2">
              <TSelectList
                v-if="filterOptions"
                :title="filterCaption ?? 'Filter'"
                :default-value="selectedFilterOption || ''"
                :values="filterOptionsVariants"
                @checked-value="(value) => (selectedFilterOption = value)"
              />

              <TSelectList
                v-if="sortOptions"
                title="Sort by"
                hide-selected-small-screens
                :default-value="selectedSortOption || ''"
                :values="sortOptionsVariants"
                @checked-value="
                  (value: string) => {
                    selectedSortOption = value
                  }
                "
              />

              <TSelectList
                v-if="itemsPerPage?.length > 1 && pagination"
                v-model="selectedItemsPerPage"
                title="Items per Page"
                :values="itemsPerPage"
                @checked-value="currentPage = 1"
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
            <slot
              :items="items"
              :search-query="searchQuery"
              :side-panel-open
              :side-panel-selected-item-id
              :open-panel="
                (id: string) => {
                  sidePanelOpen = !sidePanelOpen || sidePanelSelectedItemId !== id
                  sidePanelSelectedItemId = id
                }
              "
            />
          </div>
        </div>
      </div>

      <Pagination v-if="pagination" v-model:current-page="currentPage" :page-count="pageCount" />
    </div>

    <div
      v-if="$slots.sidePanel"
      class="overflow-hidden max-lg:absolute max-lg:inset-0 max-lg:z-10 lg:transition-all"
      :class="sidePanelOpen ? 'max-lg:w-full lg:w-sm' : 'pointer-events-none opacity-0 lg:w-0'"
    >
      <slot
        name="sidePanel"
        :items
        :search-query
        :side-panel-open
        :side-panel-selected-item-id
        :close-panel="() => (sidePanelOpen = false)"
      />
    </div>
  </div>
</template>
