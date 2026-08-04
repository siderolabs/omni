// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import Pagination from './Pagination.vue'

const meta: Meta<typeof Pagination> = {
  // https://github.com/storybookjs/storybook/issues/24238
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  component: Pagination as any,
  args: {
    items: Array(100),
    searchOption: '',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
