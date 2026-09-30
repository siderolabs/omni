// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import ProgressBar from './ProgressBar.vue'

const meta: Meta<typeof ProgressBar> = {
  component: ProgressBar,
  args: {
    modelValue: 50,
  },
  argTypes: {
    modelValue: {
      type: 'number',
      control: 'number',
    },
    color: {
      type: 'string',
      control: 'select',
      options: [
        'var(--color-status-success-default)',
        'var(--color-status-warning-default)',
        'var(--color-status-danger-default)',
      ],
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {}
