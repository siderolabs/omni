// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import InstallationMediaCreate from './InstallationMediaCreate.vue'

const meta: Meta<typeof InstallationMediaCreate> = {
  component: InstallationMediaCreate,
  parameters: {
    layout: 'fullscreen',
  },
  args: {
    class: 'h-screen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
