// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useRouteQuery } from '@vueuse/router'
import { computed, type MaybeRefOrGetter, type Ref, toValue, watch } from 'vue'
import { useRoute } from 'vue-router'

import { type WatchOptions } from '@/methods/useResourceWatch'

export interface ResourceFilterOption {
  label: string
  value?: string
}

export interface UseResourceSearchOptions {
  filterValue: Ref<string>
  filterOptions?: MaybeRefOrGetter<ResourceFilterOption[]>
}

const LABEL_REG = /^(.+):(.*)$/

export function useResourceSearch({ filterValue, filterOptions = [] }: UseResourceSearchOptions) {
  const route = useRoute()

  const filterValueRoute = useRouteQuery<string>('q', '')
  const selectedFilterOption = useRouteQuery('filter', toValue(filterOptions).at(0)?.value)

  const queryParts = computed(() =>
    filterValue.value
      .split(' ')
      .map((p) => p.trim())
      .filter(Boolean),
  )

  const labelSelectors = computed(() =>
    queryParts.value
      .map((p) => p.match(LABEL_REG))
      .filter((p) => !!p)
      .map(([, label, value]) => `${label}=${value}`),
  )

  const textQueryParts = computed(() => queryParts.value.filter((p) => !LABEL_REG.test(p)))

  const watchOptions = computed<Pick<WatchOptions, 'searchFor' | 'selectors'>>(() => {
    const searchFor = textQueryParts.value.slice()

    if (selectedFilterOption.value) {
      searchFor.unshift(selectedFilterOption.value)
    }

    return {
      searchFor: searchFor.length ? searchFor : undefined,
      selectors: labelSelectors.value.slice(),
    }
  })

  const searchQuery = computed(() => textQueryParts.value.join(' '))

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

  return {
    watchOptions,
    searchQuery,
    selectedFilterOption,
  }
}
