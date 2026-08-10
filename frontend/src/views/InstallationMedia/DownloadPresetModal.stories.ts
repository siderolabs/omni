// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import { handlers } from './DownloadPresetModal.mocks'
import DownloadPresetModal from './DownloadPresetModal.vue'

const meta: Meta<typeof DownloadPresetModal> = {
  component: DownloadPresetModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
    id: `${faker.hacker.noun()} preset`,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}
