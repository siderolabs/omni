// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, test } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'

import { useResourcePagination, type UseResourcePaginationOptions } from './useResourcePagination'

function mountUseResourcePagination(options?: UseResourcePaginationOptions) {
  let result!: ReturnType<typeof useResourcePagination>

  const TestComponent = defineComponent({
    setup() {
      result = useResourcePagination(options)

      return () => null
    },
  })

  mount(TestComponent)

  return result
}

describe('useResourcePagination', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  test('defaults to page 1 with a page size of 10', () => {
    const { currentPage, currentPageSize } = mountUseResourcePagination()

    expect(currentPage.value).toBe(1)
    expect(currentPageSize.value).toBe(10)
  })

  test('computes limit and offset from the current page and page size', async () => {
    const { currentPage, currentPageSize, watchOptions, total } = mountUseResourcePagination()

    total.value = 100
    currentPageSize.value = 10
    await nextTick()
    currentPage.value = 3
    await nextTick()

    expect(watchOptions.value).toEqual({ limit: 10, offset: 20 })
  })

  test('derives pageCount from total and page size', async () => {
    const { total, currentPageSize, pageCount } = mountUseResourcePagination()

    total.value = 25
    currentPageSize.value = 10
    await nextTick()

    expect(pageCount.value).toBe(3)
  })

  test('resets to page 1 when the page size changes', async () => {
    const { currentPage, currentPageSize, total } = mountUseResourcePagination()

    total.value = 100
    currentPage.value = 3
    await nextTick()

    currentPageSize.value = 25
    await nextTick()

    expect(currentPage.value).toBe(1)
  })

  test('resets to page 1 when a resetOn source changes', async () => {
    const filter = ref('a')
    const { currentPage, total } = mountUseResourcePagination({ resetOn: [filter] })

    total.value = 100
    currentPage.value = 3
    await nextTick()

    filter.value = 'b'
    await nextTick()

    expect(currentPage.value).toBe(1)
  })
})
