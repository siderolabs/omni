// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'
import type { ComponentProps } from 'vue-component-type-helpers'

import SplitButton from './SplitButton.vue'

const variants: ComponentProps<typeof SplitButton>['variant'][] = [
  'primary',
  'secondary',
  'highlighted',
  'subtle',
]

const sizes: ComponentProps<typeof SplitButton>['size'][] = ['md', 'sm', 'xs', 'xxs']

const meta: Meta<typeof SplitButton> = {
  component: SplitButton,
  args: {
    disabled: false,
    actions: faker.helpers.uniqueArray(faker.hacker.verb, 5) as [string, string, ...string[]],
    onClick: fn(),
  },
  argTypes: {
    variant: {
      control: 'select',
      options: variants,
    },
    size: {
      control: 'inline-radio',
      options: sizes,
    },
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
