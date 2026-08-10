// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { handlers } from './HomeOngoingOperations.mocks'
import HomeOngoingOperations from './HomeOngoingOperations.vue'

const meta: Meta<typeof HomeOngoingOperations> = {
  component: HomeOngoingOperations,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}
