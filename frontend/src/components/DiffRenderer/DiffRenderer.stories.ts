// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import DiffRenderer from './DiffRenderer.vue'
import sampleDiff1 from './sample_config_diff_1.diff?raw'
import sampleDiff2 from './sample_config_diff_2.diff?raw'

const meta: Meta<typeof DiffRenderer> = {
  component: DiffRenderer,
  args: {
    withSearch: true,
    diffs: [
      {
        id: 'diff-1',
        diff: sampleDiff1,
        label: 'Created on 2026-02-12 19:13:29',
      },
      {
        id: 'diff-2',
        diff: sampleDiff2,
        label: 'Created on 2026-01-27 13:47:31',
      },
    ],
  },
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [() => ({ template: '<div class="h-screen p-6"><story/></div>' })],
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const SingleDiff: Story = {
  args: {
    diffs: [{ id: 'diff-1', diff: sampleDiff1 }],
  },
}
