// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createGetMock, createWatchStreamMock } from '@msw/server'
import { render, screen, waitFor } from '@testing-library/vue'
import { expect, test } from 'vitest'
import { createRouter, createWebHistory } from 'vue-router'

import { ClusterLocked } from '@/api/resources'
import { routes } from '@/router'

import ClusterItem from './ClusterItem.vue'

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
