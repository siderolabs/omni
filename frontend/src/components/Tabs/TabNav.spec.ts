// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen } from '@testing-library/vue'
import { beforeEach, describe, expect, test } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import TabNav from './TabNav.vue'

const view = { template: '<div />' }

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: ['/overview', '/logs', '/config/current', '/config/patches', '/system'].map((path) => ({
      path,
      component: view,
    })),
  })
}

let router: ReturnType<typeof makeRouter>

beforeEach(async () => {
  router = makeRouter()
  await router.push('/overview')
})

function renderNav(current = false) {
  return render(TabNav, {
    props: {
      label: 'Machine',
      items: [
        { label: 'Overview', to: '/overview' },
        { label: 'Logs', to: '/logs' },
        { label: 'Configuration', to: '/config/current', current },
        { label: 'System', to: '/system', disabled: true },
      ],
    },
    global: { plugins: [router] },
  })
}

describe('TabNav', () => {
  test('renders a labelled nav of links', () => {
    renderNav()

    const nav = screen.getByRole('navigation', { name: 'Machine' })

    expect(nav.tagName).toBe('NAV')
    expect(screen.getByRole('link', { name: 'Logs' })).toHaveAttribute('href', '/logs')
  })

  test('styles the links themselves, through the as-child wrappers', () => {
    renderNav()

    expect(screen.getByRole('link', { name: 'Logs' })).toHaveClass('text-sm', 'relative')
    expect(screen.getByRole('link', { name: 'System' })).toHaveClass('text-sm', 'relative')
  })

  test('marks the link to the current page', () => {
    renderNav()

    expect(screen.getByRole('link', { name: 'Overview' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Logs' })).not.toHaveAttribute('aria-current')
  })

  test('marks a tab standing for the current page without calling it the page', async () => {
    await router.push('/config/patches')
    renderNav(true)

    const tab = screen.getByRole('link', { name: 'Configuration' })

    expect(tab).toHaveAttribute('aria-current', 'true')
    expect(tab).toHaveAttribute('data-state', 'active')
  })

  test('navigates in the app on click', async () => {
    renderNav()

    await userEvent.click(screen.getByRole('link', { name: 'Logs' }))

    expect(router.currentRoute.value.path).toBe('/logs')
    expect(screen.getByRole('link', { name: 'Logs' })).toHaveAttribute('aria-current', 'page')
  })

  test('moves focus between links with the arrow keys', async () => {
    renderNav()

    screen.getByRole('link', { name: 'Overview' }).focus()
    await userEvent.keyboard('{ArrowRight}')

    expect(screen.getByRole('link', { name: 'Logs' })).toHaveFocus()
  })

  test('still marks a disabled tab as current when its page is open', async () => {
    await router.push('/system')
    renderNav()

    const tab = screen.getByRole('link', { name: 'System' })

    expect(tab).toHaveAttribute('aria-current', 'page')
    expect(tab).toHaveAttribute('data-state', 'active')
  })

  test('renders a disabled tab as a disabled link with no href', () => {
    renderNav()

    const tab = screen.getByRole('link', { name: 'System' })

    expect(tab).toHaveAttribute('aria-disabled', 'true')
    expect(tab).not.toHaveAttribute('href')
  })
})
