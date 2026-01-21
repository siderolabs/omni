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
import { type Ref, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineType } from '@/api/resources'
import Watch from '@/api/watch'

describe('watch', () => {
  const items: Ref<Resource<MachineSpec>[]> = ref([])
  const watch = new Watch(items)

  test('event handling', async () => {
    const { pushEvents } = createWatchStreamMock({ skipBootstrap: true })

    await watch.start({
      runtime: Runtime.Omni,
      resource: { type: MachineType, namespace: DefaultNamespace },
    })

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
    expect(items.value).toHaveLength(0)

    // still loading
    expect(watch.loading.value).toBeTruthy()

    // Bootstrap event triggers the loading of queued events
    await pushEvents(createBootstrapEvent())

    await waitFor(() => expect(items.value).toHaveLength(1))

    const machine = items.value[0]
    expect(machine.metadata.id).toBe('1')
    expect(machine.spec.connected).toBeFalsy()
    expect(watch.loading.value).toBeFalsy()

    await pushEvents(createCreatedEvent(machine2))

    await waitFor(() => expect(items.value).toHaveLength(2))

    watch.stop()

    expect(items.value).toHaveLength(0)
  })

  test('restarts handling', async () => {
    const { pushEvents, closeStream } = createWatchStreamMock({ skipBootstrap: true })

    await watch.start({
      runtime: Runtime.Omni,
      resource: { type: MachineType, namespace: DefaultNamespace },
    })

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
      expect(watch.loading.value).toBeFalsy()
      expect(items.value).toHaveLength(4)
    })

    await closeStream(new Error('network error'))

    await populate(2)

    await waitFor(() => {
      expect(watch.loading.value).toBeFalsy()
      expect(items.value).toHaveLength(2)
    })

    watch.stop()
  })
})
