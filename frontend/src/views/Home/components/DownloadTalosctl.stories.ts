// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { handlers } from './DownloadTalosctl.mocks'
import DownloadTalosctl from './DownloadTalosctl.vue'

const meta: Meta<typeof DownloadTalosctl> = {
  component: DownloadTalosctl,
  args: {
    open: true,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}
