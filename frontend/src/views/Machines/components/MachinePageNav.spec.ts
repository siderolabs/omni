// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render, screen } from '@testing-library/vue'
import { beforeEach, describe, expect, test } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import MachinePageNav, { type MachinePageRoute, type MachinePageTab } from './MachinePageNav.vue'

const view = { template: '<div />' }

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      'NodeOverview',
      'NodeConfig',
      'NodePatches',
      'NodeDisks',
      'NodeDevices',
      'MachineConfigPatches',
      'MachinePatchEdit',
    ].map((name) => ({ name, path: `/${name}`, component: view })),
  })
}

let router: ReturnType<typeof makeRouter>

beforeEach(() => {
  router = makeRouter()
})

async function renderAt(route: MachinePageRoute, tabs: MachinePageTab[]) {
  await router.push(`/${route}`)

  render(MachinePageNav, { props: { tabs }, global: { plugins: [router] } })
}

const clusterTabs = (canReadConfig = true): MachinePageTab[] => [
  { name: 'Overview', pages: [{ name: 'Overview', route: 'NodeOverview' }] },
  {
    name: 'Configuration',
    pages: [
      { name: 'Current', route: 'NodeConfig', disabled: !canReadConfig },
      { name: 'Patches', route: 'NodePatches' },
    ],
  },
  {
    name: 'System',
    pages: [
      { name: 'Disks', route: 'NodeDisks', disabled: true },
      { name: 'Devices', route: 'NodeDevices', disabled: true },
    ],
  },
]

describe('MachinePageNav', () => {
  test('shows no second row on a tab that is a single page', async () => {
    await renderAt('NodeOverview', clusterTabs())

    expect(screen.getAllByRole('navigation')).toHaveLength(1)
  })

  test('on a page of a grouped tab, marks the tab and lists its pages in a second row', async () => {
    await renderAt('NodePatches', clusterTabs())

    const tab = screen.getByRole('link', { name: 'Configuration' })

    expect(tab).toHaveAttribute('aria-current', 'true')

    const second = screen.getByRole('navigation', { name: 'Configuration' })

    expect(second).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'Patches' }).at(-1)).toHaveAttribute(
      'aria-current',
      'page',
    )
  })

  test('opens the first page of a tab that the user may read', async () => {
    await renderAt('NodeOverview', clusterTabs(false))

    expect(screen.getByRole('link', { name: 'Configuration' })).toHaveAttribute(
      'href',
      '/NodePatches',
    )
  })

  test('disables a tab whose pages are all disabled', async () => {
    await renderAt('NodeOverview', clusterTabs())

    const tab = screen.getByRole('link', { name: 'System' })

    expect(tab).toHaveAttribute('aria-disabled', 'true')
    expect(tab).not.toHaveAttribute('href')
  })

  test('marks a tab current on a route that belongs to one of its pages', async () => {
    await renderAt('MachinePatchEdit', [
      {
        name: 'Configuration',
        pages: [{ name: 'Patches', route: 'MachineConfigPatches', alsoOn: ['MachinePatchEdit'] }],
      },
    ])

    expect(screen.getByRole('link', { name: 'Configuration' })).toHaveAttribute(
      'data-state',
      'active',
    )
    expect(screen.getAllByRole('navigation')).toHaveLength(1)
  })
})
