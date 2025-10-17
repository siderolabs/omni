// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'
import type { ComponentProps } from 'vue-component-type-helpers'

import { icons } from '../Icon/icons'
import TInput from './TInput.vue'

type Props = ComponentProps<typeof TInput>

const meta: Meta<typeof TInput> = {
  component: TInput,
  args: {
    title: 'Title',
    placeholder: 'Placeholder',
    type: 'text',
    icon: 'kubernetes',
    secondary: false,
    focus: false,
    compact: false,
    onClear: fn(),
    'onUpdate:model-value': fn(),
  },
  argTypes: {
    type: {
      control: 'select',
      options: ['text', 'number', 'password'] satisfies Props['type'][],
    },
    icon: {
      control: 'select',
      options: Object.keys(icons).sort(),
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
