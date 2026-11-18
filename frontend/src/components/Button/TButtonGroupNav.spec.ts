// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen } from '@testing-library/vue'
import { beforeEach, describe, expect, test } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import TButtonGroupNav from './TButtonGroupNav.vue'

const view = { template: '<div />' }

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: ['/current', '/patches', '/history'].map((path) => ({ path, component: view })),
  })
}

let router: ReturnType<typeof makeRouter>

beforeEach(async () => {
  router = makeRouter()
  await router.push('/current')

  render(TButtonGroupNav, {
    props: {
      label: 'Configuration',
      items: [
        { label: 'Current', to: '/current' },
        { label: 'Patches', to: '/patches' },
        { label: 'Change History', to: '/history', disabled: true },
      ],
    },
    global: { plugins: [router] },
  })
})

describe('TButtonGroupNav', () => {
  test('renders a labelled nav with the current page checked', () => {
    expect(screen.getByRole('navigation', { name: 'Configuration' })).toBeInTheDocument()

    const current = screen.getByRole('link', { name: 'Current' })

    expect(current).toHaveAttribute('aria-current', 'page')
    expect(current).toHaveAttribute('data-state', 'checked')
    expect(screen.getByRole('link', { name: 'Patches' })).toHaveAttribute('data-state', 'unchecked')
  })

  test('styles the links themselves, through the as-child wrappers', () => {
    expect(screen.getByRole('link', { name: 'Patches' })).toHaveClass('rounded', 'px-tight')
    expect(screen.getByRole('link', { name: 'Change History' })).toHaveClass('rounded', 'px-tight')
    expect(screen.getByRole('list')).toHaveClass('bg-surface-raised')
  })

  test('navigates in the app on click', async () => {
    await userEvent.click(screen.getByRole('link', { name: 'Patches' }))

    expect(router.currentRoute.value.path).toBe('/patches')
    expect(screen.getByRole('link', { name: 'Patches' })).toHaveAttribute('aria-current', 'page')
  })

  test('still marks a disabled item as current when its page is open', async () => {
    await router.push('/history')

    const item = screen.getByRole('link', { name: 'Change History' })

    expect(item).toHaveAttribute('aria-current', 'page')
    expect(item).toHaveAttribute('data-state', 'checked')
  })

  test('renders a disabled item as a disabled link with no href', () => {
    const item = screen.getByRole('link', { name: 'Change History' })

    expect(item).toHaveAttribute('aria-disabled', 'true')
    expect(item).not.toHaveAttribute('href')
  })
})
