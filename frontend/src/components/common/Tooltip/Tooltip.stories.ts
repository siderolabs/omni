// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import Tooltip from './Tooltip.vue'

const meta: Meta<typeof Tooltip> = {
  component: Tooltip,
  args: {
    placement: 'auto-start',
    offsetDistance: 10,
    offsetSkid: 30,
  },
  argTypes: {
    placement: {
      type: 'string',
      control: 'select',
      options: [
        'auto',
        'auto-start',
        'auto-end',
        'top',
        'top-start',
        'top-end',
        'bottom',
        'bottom-start',
        'bottom-end',
        'right',
        'right-start',
        'right-end',
        'left',
        'left-start',
        'left-end',
      ],
    },
    offsetDistance: { type: 'number', control: 'number' },
    offsetSkid: { type: 'number', control: 'number' },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: () => ({
    components: { Tooltip },
    template: `
      <Tooltip description="${faker.hacker.phrase()}">
        <button class="px-2 py-1 rounded bg-amber-400 text-black">Hover over me</button>
      </Tooltip>
    `,
  }),
}
