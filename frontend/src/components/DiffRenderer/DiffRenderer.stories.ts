// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import DiffRenderer from './DiffRenderer.vue'
import sampleDiff from './sample_diff.diff?raw'

const meta: Meta<typeof DiffRenderer> = {
  component: DiffRenderer,
  args: {
    withSearch: true,
    diff: sampleDiff,
  },
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  decorators: [() => ({ template: '<div class="h-screen p-6"><story/></div>' })],
}
