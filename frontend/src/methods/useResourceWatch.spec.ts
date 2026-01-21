// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createBootstrapEvent, createCreatedEvent } from '@msw/helpers'
import { createWatchStreamMock } from '@msw/server'
import { render, waitFor } from '@testing-library/vue'
import { describe, expect, test } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'

import { useResourceWatch } from './useResourceWatch'

function renderComposable<T>(factory: () => T) {
  let composableResult: T

  const TestComponent = defineComponent({
    setup() {
      composableResult = factory()
    },
    template: '<template />',
  })

  render(TestComponent)

  return composableResult!
}

describe('useResourceWatch', () => {
  test('updates single resource data after watch events', async () => {
    const { pushEvents } = createWatchStreamMock({ skipBootstrap: true })

    const { data, loading, err } = renderComposable(() =>
      useResourceWatch({
        runtime: Runtime.Omni,
        resource: { namespace: 'default', type: 'custom.sidero.dev/Resource', id: 'res-1' },
      }),
    )

    await pushEvents(
      createCreatedEvent({
        metadata: { id: 'res-1', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: { ready: true },
      }),
      createBootstrapEvent(),
    )

    await waitFor(() => {
      expect(data.value?.metadata.id).toBe('res-1')
      expect(loading.value).toBe(false)
      expect(err.value).toBeNull()
    })
  })

  test('maintains a list of resources for multi watch', async () => {
    const { pushEvents } = createWatchStreamMock({ skipBootstrap: true })

    const { data, loading } = renderComposable(() =>
      useResourceWatch({
        runtime: Runtime.Omni,
        resource: { namespace: 'default', type: 'custom.sidero.dev/Resource' },
      }),
    )

    await pushEvents(
      createCreatedEvent({
        metadata: { id: 'res-1', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: { ready: true },
      }),
      createCreatedEvent({
        metadata: { id: 'res-2', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: { ready: false },
      }),
      createBootstrapEvent(),
    )

    await waitFor(() => {
      expect(data.value).toHaveLength(2)
      expect(data.value?.map((item) => item.metadata.id).sort()).toEqual(['res-1', 'res-2'])
      expect(loading.value).toBe(false)
    })
  })

  test('respects skip until enabled', async () => {
    const { pushEvents } = createWatchStreamMock({ skipBootstrap: true })

    const options = ref({
      runtime: Runtime.Omni,
      resource: { namespace: 'default', type: 'custom.sidero.dev/Resource' },
      skip: true,
    })

    const { data, loading } = renderComposable(() => useResourceWatch(options))

    await nextTick()
    expect(loading.value).toBe(false)

    options.value = { ...options.value, skip: false }

    await waitFor(() => expect(loading.value).toBe(true))

    await pushEvents(
      createCreatedEvent({
        metadata: { id: 'res-3', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: { ready: true },
      }),
      createBootstrapEvent(),
    )

    await waitFor(() => {
      expect(data.value).toHaveLength(1)
      expect(data.value?.[0].metadata.id).toBe('res-3')
      expect(loading.value).toBe(false)
    })
  })
})
