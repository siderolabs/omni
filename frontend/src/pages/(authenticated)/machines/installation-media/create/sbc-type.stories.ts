// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { handlers } from './sbc-type.mocks'
import SBCType from './sbc-type.vue'

const meta: Meta<typeof SBCType> = {
  component: SBCType,
  args: {
    modelValue: {},
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}
