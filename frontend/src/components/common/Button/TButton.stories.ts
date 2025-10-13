// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import type { ComponentProps } from 'vue-component-type-helpers'

import TButton from './TButton.vue'

const types: ComponentProps<typeof TButton>['type'][] = [
  'primary',
  'secondary',
  'highlighted',
  'subtle',
]

const sizes: ComponentProps<typeof TButton>['size'][] = ['md', 'sm', 'xs', 'xxs']

const iconPositions: ComponentProps<typeof TButton>['iconPosition'][] = ['left', 'right']

const meta: Meta<typeof TButton> = {
  component: TButton,
  argTypes: {
    type: {
      control: 'select',
      options: types,
    },
    size: {
      control: 'inline-radio',
      options: sizes,
    },
    iconPosition: {
      control: 'inline-radio',
      options: iconPositions,
    },
    icon: {
      control: 'boolean',
      mapping: {
        true: 'delete',
      },
    },
  },
  args: {
    icon: 'delete',
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: (args) => ({
    components: { TButton },
    setup: () => ({ args }),
    template: `<TButton v-bind="args">Button</TButton>`,
  }),
}

export const AllButtons: Story = {
  decorators: [
    () => ({ template: '<div class="grid grid-cols-4 items-center gap-2"><story/></div>' }),
  ],
  render: (args) => ({
    components: { TButton },
    setup: () => ({ args }),
    template: iconPositions
      .flatMap((iconPosition) =>
        sizes.flatMap((size) =>
          types.map(
            (type) =>
              `<TButton type="${type}" size="${size}" icon-position="${iconPosition}" v-bind="args">${type}-${size}</TButton>`,
          ),
        ),
      )
      .join(''),
  }),
}
