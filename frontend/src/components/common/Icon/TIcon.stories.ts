// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { icons } from './icons'
import TIcon from './TIcon.vue'

const iconKeys = Object.keys(icons)

const meta: Meta<typeof TIcon> = {
  component: TIcon,
  parameters: {
    layout: 'centered',
  },
  argTypes: {
    icon: {
      control: 'select',
      options: iconKeys,
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  args: {
    icon: 'sidero',
  },
}

export const AllIcons: Story = {
  decorators: [() => ({ template: '<div class="grid grid-cols-8 gap-2"><story/></div>' })],
  render: () => ({
    components: { TIcon },
    template: iconKeys
      .map((icon) => `<TIcon icon="${icon}" class="size-6" aria-label="${icon}" />`)
      .join(''),
  }),
}
