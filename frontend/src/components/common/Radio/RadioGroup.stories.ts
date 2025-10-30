// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import RadioGroup from './RadioGroup.vue'

const meta: Meta<typeof RadioGroup> = {
  // https://github.com/storybookjs/storybook/issues/24238
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  component: RadioGroup as any,
  args: {
    label: faker.company.name(),
    options: faker.helpers
      .uniqueArray(() => faker.commerce.isbn(), 10)
      .map((value) => ({
        label: faker.commerce.productName(),
        description: faker.helpers.maybe(() => faker.commerce.productDescription()),
        value,
      })),
    'onUpdate:modelValue': fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
