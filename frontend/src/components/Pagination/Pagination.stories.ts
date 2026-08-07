// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import Pagination from './Pagination.vue'

const meta: Meta<typeof Pagination> = {
  component: Pagination,
  args: {
    pageCount: 12,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
