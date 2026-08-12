// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useLocalStorage, useOffsetPagination } from '@vueuse/core'
import { computed, ref } from 'vue'

import type { WatchOptions } from '@/methods/useResourceWatch'

const pageSizeSelectValues = [5, 10, 25, 50, 100]

export function useResourcePagination() {
  const total = ref(0)

  const { currentPage, currentPageSize, pageCount } = useOffsetPagination({
    total,
    pageSize: useLocalStorage('itemsPerPage', 10),
  })

  const watchOptions = computed<Pick<WatchOptions, 'limit' | 'offset'>>(() => {
    return {
      limit: currentPageSize.value,
      offset: (currentPage.value - 1) * currentPageSize.value,
    }
  })

  return {
    total,
    watchOptions,
    currentPage,
    currentPageSize,
    pageCount,
    pageSizeSelectValues,
  }
}
