// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useRouteQuery } from '@vueuse/router'
import { computed, type MaybeRefOrGetter, toRef, toValue, watch } from 'vue'

import type { WatchOptions } from '@/methods/useResourceWatch'

export interface ResourceSortOption {
  id: string
  desc: string
  descending?: boolean
}

export interface UseResourceSortOptions {
  sortOptions: MaybeRefOrGetter<ResourceSortOption[]>
}

export function useResourceSort({ sortOptions }: UseResourceSortOptions) {
  const sortField = useRouteQuery<string>('sort', toValue(sortOptions).at(0)?.id ?? '')
  const sortDesc = useRouteQuery<'asc' | 'desc', boolean>(
    'sortDir',
    toValue(sortOptions).at(0)?.descending ? 'desc' : 'asc',
    {
      transform: {
        set: (val) => (val ? 'desc' : 'asc'),
        get: (val) => val === 'desc',
      },
    },
  )

  const selectValues = computed(() => toValue(sortOptions).map((o) => o.desc))
  const selectedValue = computed({
    get() {
      return (
        toValue(sortOptions).find(
          (o) => sortField.value === o.id && sortDesc.value === !!o.descending,
        )?.desc ?? ''
      )
    },
    set(desc) {
      const option = toValue(sortOptions).find((o) => o.desc === desc)

      sortField.value = option?.id ?? ''
      sortDesc.value = !!option?.descending
    },
  })

  const watchOptions = computed<Pick<WatchOptions, 'sortByField' | 'sortDescending'>>(() => {
    if (!sortField.value) return {}

    return {
      sortByField: sortField.value,
      sortDescending: sortDesc.value,
    }
  })

  watch(
    toRef(sortOptions),
    ([firstOption]) => {
      if (!firstOption) return

      if (!sortField.value) {
        sortField.value = firstOption.id
        sortDesc.value = !!firstOption.descending
      }
    },
    { immediate: true },
  )

  return {
    watchOptions,
    selectValues,
    selectedValue,
  }
}
