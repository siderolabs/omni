// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, test } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import { useResourceSearch, type UseResourceSearchOptions } from './useResourceSearch'

async function mountUseResourceSearch(options: UseResourceSearchOptions) {
  let result!: ReturnType<typeof useResourceSearch>

  const TestComponent = defineComponent({
    setup() {
      result = useResourceSearch(options)

      return () => null
    },
  })

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div />' } }],
  })

  await router.push('/')
  await router.isReady()

  const wrapper = mount(TestComponent, { global: { plugins: [router] } })

  return {
    wrapper,
    router,
    get result() {
      return result
    },
  }
}

describe('useResourceSearch', () => {
  test('splits free text and label selectors', async () => {
    const { result } = await mountUseResourceSearch({
      filterValue: ref('hello env:prod world'),
    })

    expect(result.searchQuery.value).toBe('hello world')
    expect(result.watchOptions.value.selectors).toEqual(['env=prod'])
    expect(result.watchOptions.value.searchFor).toEqual(['hello', 'world'])
  })

  test('defaults selectedFilterOption to first filter option', async () => {
    const { result } = await mountUseResourceSearch({
      filterValue: ref(''),
      filterOptions: [
        { label: 'Name', value: 'name' },
        { label: 'ID', value: 'id' },
      ],
    })

    expect(result.selectedFilterOption.value).toBe('name')
    expect(result.watchOptions.value.searchFor).toEqual(['name'])
  })

  test('omits searchFor when there is no text query or selected filter', async () => {
    const { result } = await mountUseResourceSearch({
      filterValue: ref(''),
    })

    expect(result.watchOptions.value.searchFor).toBeUndefined()
  })

  test('syncs filterValue changes into the route query', async () => {
    const filterValue = ref('')
    const { router } = await mountUseResourceSearch({ filterValue })

    filterValue.value = 'foo'
    await flushPromises()

    expect(router.currentRoute.value.query.q).toBe('foo')
  })

  test('syncs route query changes back into filterValue', async () => {
    const filterValue = ref('')
    const { router } = await mountUseResourceSearch({ filterValue })

    await router.push({ query: { q: 'bar' } })
    await nextTick()

    // eslint-disable-next-line vue/no-ref-object-reactivity-loss
    expect(filterValue.value).toBe('bar')
  })
})
