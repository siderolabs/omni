// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createGetMock, createWatchStreamMock } from '@msw/server'
import { beforeEach, expect, test } from 'vitest'
import { userEvent } from 'vitest/browser'
import { render } from 'vitest-browser-vue'
import { createRouter, createWebHistory, RouterView } from 'vue-router'

import { ClusterLocked } from '@/api/resources'

import ClusterItem from './ClusterItem.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: RouterView },
    { name: 'ClusterOverview', path: '/clusters/:cluster', component: RouterView },
  ],
})

beforeEach(() => {
  createWatchStreamMock()
  createGetMock()
})

test('no lock if unlocked', async () => {
  const screen = await render(ClusterItem, {
    global: {
      stubs: ['Tooltip', 'TActionsBox'],
      plugins: [router],
    },
    props: {
      item: {
        spec: {},
        metadata: { id: 'fake' },
      },
    },
  })

  // TODO: This was query before, needs validating
  await expect.element(screen.getByLabelText('locked')).not.toBeInTheDocument()
})

test('lock if locked', async () => {
  const screen = await render(ClusterItem, {
    global: {
      stubs: ['Tooltip', 'TActionsBox'],
      plugins: [router],
    },
    props: {
      item: {
        spec: {},
        metadata: { id: 'fake', annotations: { [ClusterLocked]: '' } },
      },
    },
  })

  await expect.element(screen.getByLabelText('locked')).toBeInTheDocument()
})

test.skip('collapsing stops ClusterMachines resource watches', async () => {
  const screen = await render(ClusterItem, {
    global: {
      stubs: ['Tooltip', 'TActionsBox'],
      plugins: [router],
    },
    props: {
      item: { spec: {}, metadata: { id: 'collapse-watches-test' } },
      defaultOpen: true,
    },
  })

  const collapsible = screen.getByRole('region', { name: 'collapse-watches-test' })

  await expect.element(collapsible).not.toBeEmptyDOMElement()

  await userEvent.click(screen.getByRole('button', { name: 'collapse-watches-test' }))

  // Asserting watches was flaky, we instead assert component is unmounted to verify that watches are removed
  await expect.element(collapsible).toBeEmptyDOMElement()
})
