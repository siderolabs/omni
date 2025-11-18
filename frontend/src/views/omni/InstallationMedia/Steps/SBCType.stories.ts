// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import SBCType from './SBCType.vue'

const meta: Meta<typeof SBCType> = {
  component: SBCType,
  args: {
    modelValue: { currentStep: 0 },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
