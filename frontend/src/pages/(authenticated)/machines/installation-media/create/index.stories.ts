// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import Entry from './index.vue'

const meta: Meta<typeof Entry> = {
  // Have to manually specify title and ID due to create vs create/index conflict
  id: 'pages-authenticated-machines-installation-media-create-index',
  // eslint-disable-next-line storybook/no-title-property-in-meta
  title: 'pages/(authenticated)/machines/installation-media/create/index',
  component: Entry,
  args: {
    modelValue: {},
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
