// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createGetMock, createWatchStreamMock, server } from '@msw/server'
import userEvent from '@testing-library/user-event'
import { render, screen, waitFor } from '@testing-library/vue'
import { http, HttpResponse } from 'msw'
import { expect, test } from 'vitest'
import { createRouter, createWebHistory, type RouteRecordRaw, RouterView } from 'vue-router'

import { ClusterDiagnosticsType, ClusterLocked, MachineSetType } from '@/api/resources'

import ClusterItem from './ClusterItem.vue'

const routes: RouteRecordRaw[] = [
  {
    name: 'ClusterOverview',
    path: '/clusters/:cluster',
    component: RouterView,
  },
]

test('no lock if unlocked', async () => {
  createWatchStreamMock()
  createGetMock()

  render(ClusterItem, {
    global: {
      stubs: ['Tooltip', 'TActionsBox'],
      plugins: [
        createRouter({
          history: createWebHistory(),
          routes,
        }),
      ],
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
      plugins: [
        createRouter({
          history: createWebHistory(),
          routes,
        }),
      ],
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

  let activeWatches = 0

  createWatchStreamMock()
  createGetMock()

  server.use(
    http.post('/omni.resources.ResourceService/Watch', async ({ request }) => {
      const body = (await request.clone().json()) as { type?: string }
      if (![MachineSetType, ClusterDiagnosticsType].includes(body.type ?? '')) return

      activeWatches++
      request.signal.addEventListener('abort', () => activeWatches--)

      return new HttpResponse(new ReadableStream(), {
        headers: {
          'content-type': 'application/json',
          'Grpc-metadata-content-type': 'application/grpc',
        },
      })
    }),
  )

  render(ClusterItem, {
    global: {
      stubs: ['Tooltip', 'TActionsBox'],
      plugins: [createRouter({ history: createWebHistory(), routes })],
    },
    props: {
      item: { spec: {}, metadata: { id: 'collapse-watches-test' } },
      defaultOpen: true,
    },
  })

  await waitFor(() => expect(activeWatches).toBe(2))

  await user.click(screen.getByRole('button', { name: 'collapse-watches-test' }))

  await waitFor(() => expect(activeWatches).toBe(0))
})
