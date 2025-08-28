<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, ref, toRefs, watch as vueWatch } from 'vue'

import type { Resource } from '@/api/grpc'
import type { WatchJoinOptions, WatchOptions } from '@/api/watch'
import Watch, { WatchJoin } from '@/api/watch'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import TAlert from '@/components/TAlert.vue'
import storageRef from '@/methods/storage'

defineExpose({
  addFilterLabel: (label: { key: string; value?: string }) => {
    const selector = `${label.key}:${label.value}`
    if (filterValueInternal.value.includes(selector)) {
      return
    }

    filterValueInternal.value += (filterValueInternal.value ? ' ' : '') + selector
  },
})

const dots = '...'

const props = defineProps<{
  pagination?: boolean
  search?: boolean
  opts?: WatchOptions | WatchJoinOptions[] | object
  sortOptions?: { id: string; desc: string; descending?: boolean }[]
  filterOptions?: { query?: string; desc: string }[]
  filterValue?: string
  filterCaption?: string
}>()

const itemsPerPage = [5, 10, 25, 50, 100]

const sortOptionsVariants = computed(() => {
  if (!props.sortOptions) {
    return []
  }

  return props.sortOptions.map((opt) => {
    return opt.desc
  })
})

const filterOptionsVariants = computed(() => {
  if (!props.filterOptions) {
    return []
  }

  return props.filterOptions.map((opt) => {
    return opt.desc
  })
})

const { opts, filterValue } = toRefs(props)

const items: Ref<Resource[]> = ref([])

const optsList = props.opts as WatchJoinOptions[]

const filterValueInternal = ref('')
const currentPage = ref(1)
const selectedItemsPerPage: Ref<number> = storageRef(localStorage, 'itemsPerPage', 10)
const selectedSortOption: Ref<string | undefined> = ref(sortOptionsVariants?.value?.[0])
const selectedFilterOption: Ref<string | undefined> = ref(filterOptionsVariants.value?.[0])

const filterValueComputed = computed(() => {
  return filterValue.value !== undefined ? filterValue.value : filterValueInternal.value
})

const offset = computed(() => {
  return (currentPage.value - 1) * selectedItemsPerPage.value
})

const sortByState = computed(() => {
  if (!props.sortOptions) {
    return {}
  }

  for (const opt of props.sortOptions) {
    if (opt.desc === selectedSortOption?.value) {
      return {
        sortByField: opt.id,
        sortDescending: opt.descending,
      }
    }
  }

  return {}
})

const watchOptions = computed<WatchOptions>(() => {
  const watchSingle = opts?.value
  const watchJoin = opts?.value as WatchJoinOptions[]

  return (watchJoin?.length ? watchJoin[0] : watchSingle) as WatchOptions
})

const paginationState = computed(() => {
  if (!props.pagination) {
    return {}
  }

  return {
    limit: selectedItemsPerPage.value,
    offset: offset.value,
  }
})

// reset the pagination when the search query changes
vueWatch(filterValue, () => {
  currentPage.value = 1
})

const searchState = computed(() => {
  if (!props.search) {
    return {}
  }

  const o = watchOptions.value

  if (!o) {
    return {}
  }

  // do not proceed if the pagination is not reset yet - when the currentPage is reset, this will get triggered again
  if (currentPage.value !== 1) {
    return {}
  }

  const parts = filterValueComputed.value.split(' ')
  const selectors: string[] = []
  const searchFor: string[] = []

  if (selectedFilterOption.value) {
    const selectedOptionQuery = props.filterOptions?.find(
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
    selectors: (o.selectors ?? []).concat(selectors),
  }

  if (searchFor.length > 0) {
    res.searchFor = searchFor
  }

  return res
})

const searchQuery = computed(() => {
  if (!searchState.value.searchFor) {
    return undefined
  }

  return searchState.value.searchFor.join(' ')
})

const setupWatch = () => {
  const w = new Watch(items)

  w.setup(
    computed(() => {
      if (!opts?.value) {
        return
      }

      return {
        ...paginationState.value,
        ...(opts.value as WatchOptions),
        ...searchState.value,
        ...sortByState.value,
      }
    }),
  )

  return w
}

const setupJoinWatch = () => {
  const w = new WatchJoin(items)

  w.setup(
    computed(() => {
      if (!opts?.value) {
        return
      }

      return {
        ...paginationState.value,
        ...(opts.value as WatchJoinOptions[])[0],
        ...searchState.value,
        ...sortByState.value,
      }
    }),
    computed(() => {
      if (!opts?.value) {
        return
      }

      const o = opts.value as WatchJoinOptions[]

      return o.slice(1, o.length)
    }),
  )

  return w
}

const paginationRange = computed(() => {
  let ranges: number[][]
  if (totalPageCount.value < 20) {
    ranges = [[1, totalPageCount.value]]
  } else {
    if (currentPage.value < 5 || currentPage.value > totalPageCount.value - 4) {
      ranges = [
        [1, 5],
        [totalPageCount.value - 4, totalPageCount.value],
      ]
    } else {
      ranges = [
        [1, 3],
        [currentPage.value - 1, currentPage.value + 1],
        [totalPageCount.value - 2, totalPageCount.value],
      ]
    }
  }

  const res: (string | number)[] = []
  for (let i: number = 0; i < ranges.length; i++) {
    for (let j: number = ranges[i][0]; j <= ranges[i][1]; j++) {
      res.push(j)
    }

    if (i !== ranges.length - 1) {
      res.push(dots)
    }
  }

  return res
})

const watch = optsList?.length ? setupJoinWatch() : setupWatch()
const err = watch.err
const loading = watch.loading
const itemsCount = watch.total

const totalPageCount = computed(() => {
  return Math.ceil(watch.total.value / selectedItemsPerPage.value)
})

const showPageSelector = computed(() => {
  return props.pagination && totalPageCount.value > 1
})

const prevPage = () => {
  currentPage.value = Math.max(1, currentPage.value - 1)
}

const nextPage = () => {
  currentPage.value = Math.min(totalPageCount.value, currentPage.value + 1)
}

const openPage = (page: number | string) => {
  if (page === dots) {
    return
  }

  currentPage.value = page as number
}
</script>

<template>
  <div>
    <slot
      name="header"
      :items-count="itemsCount"
      :filtered="searchState.searchFor?.length || searchState.selectors?.length"
    />

    <div class="flex flex-col gap-4">
      <template v-if="pagination || search || (pagination && itemsPerPage?.length > 1)">
        <slot name="input">
          <TInput v-if="search" v-model="filterValueInternal" class="grow" icon="search" />
          <div v-else class="grow" />
        </slot>

        <div class="flex items-center gap-2">
          <slot name="extra-controls" />

          <div class="grow" />

          <TSelectList
            v-if="filterOptions"
            :title="filterCaption ?? 'Filter'"
            :default-value="selectedFilterOption || ''"
            :values="filterOptionsVariants"
            @checked-value="
              (value: string) => {
                selectedFilterOption = value
              }
            "
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
            title="Items per Page"
            :default-value="selectedItemsPerPage"
            :values="itemsPerPage"
            @checked-value="
              (value: number) => {
                selectedItemsPerPage = value
                currentPage = 1
              }
            "
          />
        </div>
      </template>

      <div class="grow">
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
          <slot :items="items" :watch="watch" :search-query="searchQuery" />
        </div>
      </div>

      <div v-if="showPageSelector" class="flex items-center justify-end gap-2">
        <TIcon
          icon="arrow-left"
          class="size-5 cursor-pointer fill-current transition-all duration-200 hover:text-naturals-n10"
          :class="currentPage === 1 ? 'text-naturals-n6' : 'text-naturals-n8'"
          @click="prevPage"
        />

        <div class="flex items-center gap-2 transition-all duration-200">
          <span
            v-for="(item, index) in paginationRange ?? []"
            :key="index"
            class="flex size-7 items-center justify-center rounded transition-all duration-200 select-none"
            :class="[
              item === currentPage ? 'bg-naturals-n4 text-naturals-n12' : 'text-naturals-n8',
              item === dots
                ? 'cursor-default hover:text-naturals-n8'
                : 'cursor-pointer hover:text-naturals-n9',
            ]"
            @click="() => openPage(item)"
          >
            {{ item }}
          </span>
        </div>

        <TIcon
          icon="arrow-right"
          class="size-5 cursor-pointer fill-current transition-all duration-200 hover:text-naturals-n10"
          :class="[currentPage === totalPageCount ? 'text-naturals-n6' : 'text-naturals-n8']"
          @click="nextPage"
        />
      </div>
    </div>
  </div>
</template>
