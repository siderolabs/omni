// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import CodeBlock from './CodeBlock.vue'

const meta: Meta<typeof CodeBlock> = {
  component: CodeBlock,
  args: {
    code: 'brew install siderolabs/tap/sidero-tools',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
