// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import type { ComponentProps } from 'vue-component-type-helpers'

import TButton from './TButton.vue'

const variants: ComponentProps<typeof TButton>['variant'][] = [
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
    variant: {
      control: 'select',
      options: variants,
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
// Discriminated unions in vue don't play well with storybook
type Story = StoryObj /*<typeof meta>*/

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
          variants.map(
            (variant) =>
              `<TButton variant="${variant}" size="${size}" icon-position="${iconPosition}" v-bind="args">${variant}-${size}</TButton>`,
          ),
        ),
      )
      .join(''),
  }),
}
