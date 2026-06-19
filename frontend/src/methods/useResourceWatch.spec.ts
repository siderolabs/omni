// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import {
  createBootstrapEvent,
  createCreatedEvent,
  createDestroyedEvent,
  createUpdatedEvent,
} from '@msw/helpers'
import { createWatchStreamMock } from '@msw/server'
import { waitFor } from '@testing-library/vue'
import { describe, expect, test } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineType } from '@/api/resources'

import { useResourceWatch } from './useResourceWatch'

function renderComposable<T>(factory: () => T) {
  // Testing with effectScope as we do not want to depend on lifecycle hooks
  const scope = effectScope()

  return {
    ...scope.run(factory)!,
    unmount: () => scope.stop(),
  }
}

describe('useResourceWatch', () => {
  test('handles created, updated, and destroyed events across bootstrap', async () => {
    const { pushEvents } = createWatchStreamMock({ skipBootstrap: true })

    const { data, loading, unmount } = renderComposable(() =>
      useResourceWatch<MachineSpec>({
        runtime: Runtime.Omni,
        resource: { type: MachineType, namespace: DefaultNamespace },
      }),
    )

    const machine1: Resource<MachineSpec> = {
      metadata: { id: '1', namespace: 'default', type: MachineType },
      spec: { connected: true, management_address: 'localhost' },
    }

    const machine2: Resource<MachineSpec> = {
      metadata: { id: '2', namespace: 'default', type: MachineType },
      spec: { connected: true, management_address: 'localhost' },
    }

    await pushEvents(
      createCreatedEvent(machine1),
      createCreatedEvent(machine2),
      createDestroyedEvent(machine2),
      createUpdatedEvent(
        {
          ...machine1,
          spec: { ...machine1.spec, connected: false },
        },
        machine1,
      ),
    )

    // not yet bootstrapped
    expect(data.value).toHaveLength(0)

    // still loading
    expect(loading.value).toBeTruthy()

    // Bootstrap event triggers the loading of queued events
    await pushEvents(createBootstrapEvent())

    await waitFor(() => expect(data.value).toHaveLength(1))

    const machine = data.value[0]
    expect(machine.metadata.id).toBe('1')
    expect(machine.spec.connected).toBe(false)
    expect(loading.value).toBe(false)

    await pushEvents(createCreatedEvent(machine2))

    await waitFor(() => expect(data.value).toHaveLength(2))

    unmount()

    expect(data.value).toHaveLength(0)
  })

  test('resets and reloads items after stream reconnect', async () => {
    const { pushEvents, closeStream } = createWatchStreamMock({ skipBootstrap: true })

    const { data, loading } = renderComposable(() =>
      useResourceWatch<MachineSpec>({
        runtime: Runtime.Omni,
        resource: { type: MachineType, namespace: DefaultNamespace },
      }),
    )

    async function populate(count: number) {
      await pushEvents(
        ...new Array(count).fill(null).map((_, i) =>
          createCreatedEvent<MachineSpec>({
            metadata: { id: i.toString(), namespace: 'default', type: MachineType },
            spec: { connected: true, management_address: 'localhost' },
          }),
        ),
        createBootstrapEvent(),
      )
    }

    await populate(4)

    await waitFor(() => {
      expect(loading.value).toBe(false)
      expect(data.value).toHaveLength(4)
    })

    await closeStream(new Error('network error'))

    await populate(2)

    await waitFor(() => {
      expect(loading.value).toBe(false)
      expect(data.value).toHaveLength(2)
    })
  })

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
      runtime: Runtime.Omni as const,
      resource: { namespace: 'default', type: 'custom.sidero.dev/Resource' },
      skip: true,
    })

    const { data, loading } = renderComposable(() => useResourceWatch(options))

    await nextTick()
    expect(loading.value).toBe(false)

    options.value.skip = false

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

  test('stops the watch when the component is unmounted', async () => {
    const { pushEvents, waitForStreamClose } = createWatchStreamMock({ skipBootstrap: true })

    const { data, unmount } = renderComposable(() =>
      useResourceWatch({
        runtime: Runtime.Omni,
        resource: { namespace: 'default', type: 'custom.sidero.dev/Resource' },
      }),
    )

    await pushEvents(
      createCreatedEvent({
        metadata: { id: 'res-1', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: {},
      }),
      createBootstrapEvent(),
    )

    await waitFor(() => expect(data.value).toHaveLength(1))

    const streamClosed = waitForStreamClose()

    unmount()

    await streamClosed
    expect(data.value).toHaveLength(0)
  })

  test('only restarts watch if options actually change', async () => {
    const { pushEvents } = createWatchStreamMock({ skipBootstrap: true })

    const baseOpts = {
      runtime: Runtime.Omni as const,
      resource: { namespace: 'default', type: 'custom.sidero.dev/Resource' },
    }

    const opts = ref({ ...baseOpts })

    const { data, loading } = renderComposable(() => useResourceWatch(opts))

    await pushEvents(
      createCreatedEvent({
        metadata: { id: 'res-1', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: {},
      }),
      createBootstrapEvent(),
    )

    await waitFor(() => {
      expect(loading.value).toBe(false)
      expect(data.value).toHaveLength(1)
    })

    // same content, new object reference — must not restart
    opts.value = { ...baseOpts }
    await nextTick()
    expect(loading.value).toBe(false)
    expect(data.value).toHaveLength(1)

    // genuinely different content — must restart (loading resets, data cleared)
    opts.value = {
      ...baseOpts,
      resource: { namespace: 'other', type: 'custom.sidero.dev/Resource' },
    }
    await nextTick()
    expect(loading.value).toBe(true)
    expect(data.value).toHaveLength(0)
  })
})
