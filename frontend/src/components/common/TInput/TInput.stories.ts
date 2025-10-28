// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'
import { ref } from 'vue'
import type { ComponentProps } from 'vue-component-type-helpers'

import { icons } from '../Icon/icons'
import TInput from './TInput.vue'

type Props = ComponentProps<typeof TInput>

const meta: Meta<typeof TInput> = {
  // https://github.com/storybookjs/storybook/issues/24238
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  component: TInput as any,
  args: {
    title: 'Title',
    placeholder: 'Placeholder',
    type: 'text',
    icon: 'kubernetes',
    secondary: false,
    focus: false,
    compact: false,
    onClear: fn(),
    'onUpdate:modelValue': fn(),
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

const minMaxModel = ref<string | number>(2048)
export const MinMax: Story = {
  args: {
    type: 'number',
    min: 2048,
    max: 4096,
    // Type issue, but passing a ref is actually allowed here
    modelValue: minMaxModel as never,
    'onUpdate:modelValue': (val) => (minMaxModel.value = val),
  },
}
