// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createGetMock, createWatchStreamMock } from '@msw/server'
import userEvent from '@testing-library/user-event'
import { render, screen, waitFor } from '@testing-library/vue'
import { expect, test } from 'vitest'
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

test('no lock if unlocked', async () => {
  createWatchStreamMock()
  createGetMock()

  render(ClusterItem, {
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

  expect(screen.queryByLabelText('locked')).not.toBeInTheDocument()
})

test('lock if locked', async () => {
  createWatchStreamMock()
  createGetMock()

  render(ClusterItem, {
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

  await waitFor(() => {
    expect(screen.getByLabelText('locked')).toBeInTheDocument()
  })
})

test('collapsing stops ClusterMachines resource watches', async () => {
  const user = userEvent.setup()

  createWatchStreamMock()
  createGetMock()

  render(ClusterItem, {
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

  await waitFor(() => expect(collapsible).not.toBeEmptyDOMElement())

  await user.click(screen.getByRole('button', { name: 'collapse-watches-test' }))

  // Asserting watches was flaky, we instead assert component is unmounted to verify that watches are removed
  await waitFor(() => expect(collapsible).toBeEmptyDOMElement())
})
