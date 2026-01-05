// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import userEvent from '@testing-library/user-event'
import { render, screen } from '@testing-library/vue'
import { afterEach, expect, test, vi } from 'vitest'

import { getPlatform } from '@/methods'

import DownloadOmnictl from './DownloadOmnictl.vue'

vi.mock('@/methods', () => ({
  getDocsLink: vi.fn(() => 'https://docs.example.com'),
  getPlatform: vi.fn(),
}))

const mockGetPlatform = vi.mocked(getPlatform)

afterEach(() => {
  vi.clearAllMocks()
})

test('sets default value based on platform', async () => {
  mockGetPlatform.mockResolvedValue(['linux', 'amd64'])

  render(DownloadOmnictl)

  expect(await screen.findByLabelText('omnictl')).toHaveTextContent('omnictl-linux-amd64')
})

test('allows selecting other options', async () => {
  const user = userEvent.setup()
  mockGetPlatform.mockResolvedValue(['linux', 'amd64'])

  render(DownloadOmnictl)

  const trigger = await screen.findByLabelText('omnictl')

  // Open dropdown
  await user.click(trigger)

  const option = screen.getByRole('option', { name: 'omnictl-darwin-arm64' })

  // Select option
  await user.click(option)

  expect(trigger).toHaveTextContent('omnictl-darwin-arm64')
})
