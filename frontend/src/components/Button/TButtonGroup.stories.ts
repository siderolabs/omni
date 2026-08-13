// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import TButtonGroup from './TButtonGroup.vue'

const meta: Meta<typeof TButtonGroup> = {
  // https://github.com/storybookjs/storybook/issues/24238
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  component: TButtonGroup as any,
  args: {
    'onUpdate:modelValue': fn(),
    deselectEnabled: true,
    options: faker.helpers.multiple(
      (_, i) => ({
        label: faker.commerce.productName(),
        disabled: faker.datatype.boolean(),
        tooltip: faker.commerce.productDescription(),
        value: i,
      }),
      { count: 5 },
    ),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
