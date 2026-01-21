// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import TCheckbox from './TCheckbox.vue'

const meta: Meta<typeof TCheckbox> = {
  component: TCheckbox,
  args: {
    label: 'Label',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
