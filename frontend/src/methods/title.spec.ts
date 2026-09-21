// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'

import { useTitle } from './title'

/** Mounts useTitle in its own scope, mimicking a page component. */
function mountTitle(...args: Parameters<typeof useTitle>) {
  const scope = effectScope()

  scope.run(() => useTitle(...args))

  return () => scope.stop()
}

describe('useTitle', () => {
  test('falls back to the bare base title', async () => {
    const unmount = mountTitle(undefined)
    await nextTick()

    expect(document.title).toBe('Omni')

    unmount()
  })

  test('puts the most specific segment first', async () => {
    const unmountParent = mountTitle(['Clusters', 'talos-default'])
    const unmountChild = mountTitle('Nodes')
    await nextTick()

    expect(document.title).toBe('Nodes · talos-default · Clusters · Omni')

    unmountChild()
    unmountParent()
  })

  test('leaves out segments that have not loaded yet', async () => {
    const machine = ref<string>()
    const unmount = mountTitle(() => ['Machines', machine.value])
    await nextTick()

    expect(document.title).toBe('Machines · Omni')

    machine.value = 'my-machine'
    await nextTick()

    expect(document.title).toBe('my-machine · Machines · Omni')

    unmount()
  })

  test('drops only its own segments on unmount', async () => {
    const unmountParent = mountTitle('Clusters')
    const unmountChild = mountTitle('Nodes')
    await nextTick()

    unmountChild()
    await nextTick()

    expect(document.title).toBe('Clusters · Omni')

    unmountParent()
    await nextTick()

    expect(document.title).toBe('Omni')
  })

  test('keeps the entering page title when the leaving page unmounts late', async () => {
    const unmountLeaving = mountTitle('Nodes')
    const unmountEntering = mountTitle('Pods')
    await nextTick()

    // A <Transition> around <RouterView> unmounts the old page after the new
    // one has already mounted and pushed its own segment.
    unmountLeaving()
    await nextTick()

    expect(document.title).toBe('Pods · Omni')

    unmountEntering()
  })
})
