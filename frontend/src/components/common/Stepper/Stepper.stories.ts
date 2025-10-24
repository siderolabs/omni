// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import Stepper from './Stepper.vue'

const meta: Meta<typeof Stepper> = {
  component: Stepper,
  parameters: {
    layout: 'centered',
  },
  args: {
    stepCount: 5,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
