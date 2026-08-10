// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { cloudProvider, handlers } from './arch.mocks'
import MachineArch from './arch.vue'

const meta: Meta<typeof MachineArch> = {
  component: MachineArch,
  args: {
    modelValue: { hardwareType: 'metal' },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}

export const ForCloud: Story = {
  ...Default,
  args: {
    modelValue: {
      hardwareType: 'cloud',
      cloudPlatform: cloudProvider.metadata.id,
    },
  },
}
