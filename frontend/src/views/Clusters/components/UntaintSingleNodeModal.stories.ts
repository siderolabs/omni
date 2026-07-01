// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import UntaintSingleNodeModal from './UntaintSingleNodeModal.vue'

const meta: Meta<typeof UntaintSingleNodeModal> = {
  component: UntaintSingleNodeModal,
  args: {
    open: true,
    onContinue: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
