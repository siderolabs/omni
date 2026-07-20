// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import Switch from './Switch.vue'

const meta: Meta<typeof Switch> = {
  component: Switch,
  args: {
    label: 'Label',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
