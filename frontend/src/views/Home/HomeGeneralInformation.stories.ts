// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { handlers } from './HomeGeneralInformation.mocks'
import HomeGeneralInformation from './HomeGeneralInformation.vue'

faker.seed(0)

const meta: Meta<typeof HomeGeneralInformation> = {
  component: HomeGeneralInformation,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}
