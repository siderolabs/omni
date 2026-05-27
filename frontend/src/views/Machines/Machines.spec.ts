// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createGetMock, createWatchStreamMock } from '@msw/server'
import userEvent from '@testing-library/user-event'
import { render, screen, waitFor } from '@testing-library/vue'
import { beforeEach, expect, test } from 'vitest'
import { createRouter, createWebHistory, RouterView } from 'vue-router'

import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import { MachineStatusLinkType, MetricsNamespace } from '@/api/resources'

import Machines from './Machines.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: RouterView },
    { name: 'MachineLogs', path: '/machines/:machine/logs', component: RouterView },
    {
      name: 'MachineConfigPatches',
      path: '/machines/:machine/config-patches',
      component: RouterView,
    },
    { name: 'MachineKernelArgs', path: '/machines/:machine/kernel-args', component: RouterView },
    { name: 'InstallationMedia', path: '/installation-media', component: RouterView },
    { name: 'ClusterOverview', path: '/clusters/:cluster', component: RouterView },
  ],
})

function makeMachine(id: string): Resource<MachineStatusLinkSpec> {
  return {
    metadata: { id, namespace: MetricsNamespace, type: MachineStatusLinkType },
    spec: {
      message_status: {
        network: { hostname: id },
      },
    },
  }
}

beforeEach(() => {
  createGetMock()
})

test('select all selects all displayed machines', async () => {
  const user = userEvent.setup()

  createWatchStreamMock()
  createWatchStreamMock({
    expectedOptions: { type: MachineStatusLinkType },
    initialResources: [makeMachine('machine-1'), makeMachine('machine-2')],
  })

  render(Machines, {
    global: {
      stubs: ['AddingMachinesTutorial', 'MachineDetailsPanel', 'StatsItem'],
      plugins: [router],
    },
  })

  await waitFor(() => {
    expect(screen.getAllByRole('checkbox')).toHaveLength(2)
  })

  await user.click(screen.getByRole('button', { name: /select all/i }))

  await waitFor(() => {
    const checkboxes = screen.getAllByRole('checkbox')
    expect(checkboxes).toHaveLength(2)
    checkboxes.forEach((cb) => expect(cb).toBeChecked())
  })
})

test('select all with search filter selects only filtered machines', async () => {
  const user = userEvent.setup()

  const machine1 = makeMachine('machine-1')
  const machine2 = makeMachine('machine-2')

  createWatchStreamMock()
  createWatchStreamMock({
    expectedOptions: { type: MachineStatusLinkType },
    initialResources: (options) => {
      if (options.search_for?.includes('machine-1')) {
        return [machine1]
      }
      return [machine1, machine2]
    },
  })

  render(Machines, {
    global: {
      stubs: ['AddingMachinesTutorial', 'MachineDetailsPanel', 'StatsItem'],
      plugins: [router],
    },
  })

  await waitFor(() => {
    expect(screen.getAllByRole('checkbox')).toHaveLength(2)
  })

  await user.type(screen.getByPlaceholderText('Search ...'), 'machine-1')

  await waitFor(() => {
    expect(screen.getAllByRole('checkbox')).toHaveLength(1)
  })

  await user.click(screen.getByRole('button', { name: /select all/i }))

  await waitFor(() => {
    const checkboxes = screen.getAllByRole('checkbox')
    expect(checkboxes).toHaveLength(1)
    checkboxes.forEach((cb) => expect(cb).toBeChecked())
  })
})
