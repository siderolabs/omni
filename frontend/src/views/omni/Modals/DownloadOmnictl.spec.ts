// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { beforeEach, expect, test, vi } from 'vitest'
import { userEvent } from 'vitest/browser'
import { render } from 'vitest-browser-vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import { getPlatform } from '@/methods'

import DownloadOmnictl from './DownloadOmnictl.vue'

vi.mock('@/methods', () => ({
  getDocsLink: vi.fn(() => 'https://docs.example.com'),
  getPlatform: vi.fn(),
}))

const mockGetPlatform = vi.mocked(getPlatform)

let router: ReturnType<typeof createRouter>

beforeEach(() => {
  vi.clearAllMocks()

  router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/',
        component: { template: '<RouterView />' },
      },
    ],
  })
})

test('sets default value based on platform', async () => {
  mockGetPlatform.mockResolvedValue(['linux', 'amd64'])

  const screen = await render(DownloadOmnictl, {
    global: {
      plugins: [router],
    },
  })

  await expect.element(screen.getByLabelText('omnictl')).toHaveTextContent('omnictl-linux-amd64')
})

test('allows selecting other options', async () => {
  mockGetPlatform.mockResolvedValue(['linux', 'amd64'])

  const screen = await render(DownloadOmnictl, {
    global: {
      plugins: [router],
    },
  })

  const trigger = screen.getByLabelText('omnictl')

  // Open dropdown
  await userEvent.click(trigger)

  const option = screen.getByRole('option', { name: 'omnictl-darwin-arm64' })

  // Select option
  await userEvent.click(option)

  await expect.element(trigger).toHaveTextContent('omnictl-darwin-arm64')
})
