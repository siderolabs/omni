// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import CopyButton from './CopyButton.vue'

const meta: Meta<typeof CopyButton> = {
  component: CopyButton,
  args: {
    text: faker.lorem.sentence(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
