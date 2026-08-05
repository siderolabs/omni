// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { mount } from '@vue/test-utils'
import { describe, expect, test } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import {
  type ResourceSortOption,
  useResourceSort,
  type UseResourceSortOptions,
} from './useResourceSort'

async function mountUseResourceSort(options: UseResourceSortOptions) {
  let result!: ReturnType<typeof useResourceSort>

  const TestComponent = defineComponent({
    setup() {
      result = useResourceSort(options)

      return () => null
    },
  })

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div />' } }],
  })

  await router.push('/')
  await router.isReady()

  mount(TestComponent, { global: { plugins: [router] } })

  return result
}

const sortOptions: ResourceSortOption[] = [
  { id: 'name', desc: 'Name' },
  { id: 'created', desc: 'Created (newest)', descending: true },
]

describe('useResourceSort', () => {
  test('exposes the display labels for each sort option', async () => {
    const { selectValues } = await mountUseResourceSort({ sortOptions })

    expect(selectValues.value).toEqual(['Name', 'Created (newest)'])
  })

  test('defaults to the first sort option', async () => {
    const { selectedValue, watchOptions } = await mountUseResourceSort({ sortOptions })

    expect(selectedValue.value).toBe('Name')
    expect(watchOptions.value).toEqual({ sortByField: 'name', sortDescending: false })
  })

  test('updates sortByField and sortDescending when selectedValue is set', async () => {
    const { selectedValue, watchOptions } = await mountUseResourceSort({ sortOptions })

    selectedValue.value = 'Created (newest)'
    await nextTick()

    expect(watchOptions.value).toEqual({ sortByField: 'created', sortDescending: true })
  })

  test('returns no sort watch options when there are no sort options', async () => {
    const { watchOptions } = await mountUseResourceSort({ sortOptions: [] })

    expect(watchOptions.value).toEqual({})
  })
})
