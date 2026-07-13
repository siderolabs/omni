// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import CodeEditor from './CodeEditor.vue'

const meta: Meta<typeof CodeEditor> = {
  component: CodeEditor,
  args: {
    class: 'h-screen',
  },
  argTypes: {
    talosVersion: {
      control: 'select',
      options: Array(7)
        .fill(null)
        .map((_, index) => `1.${index + 7}`),
    },
  },
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
