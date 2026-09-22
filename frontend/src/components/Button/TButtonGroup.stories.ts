// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import TButtonGroup from './TButtonGroup.vue'

faker.seed(0)

const options = faker.helpers.multiple(
  (_, i) => ({
    label: faker.commerce.productName(),
    disabled: faker.datatype.boolean(),
    tooltip: faker.commerce.productDescription(),
    value: i,
  }),
  { count: 5 },
)

const meta: Meta<typeof TButtonGroup> = {
  component: TButtonGroup,
  args: {
    'onUpdate:modelValue': fn(),
    defaultValue: options[0].value,
    options,
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
