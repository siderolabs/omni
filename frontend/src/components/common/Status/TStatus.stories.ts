// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { NodesViewFilterOptions, TCommonStatuses, TPodsViewFilterOptions } from '@/constants'

import TStatus from './TStatus.vue'

const titles = Array.from(
  new Set([
    ...Object.values(NodesViewFilterOptions),
    ...Object.values(TCommonStatuses),
    ...Object.values(TPodsViewFilterOptions),
  ]),
).sort((a, b) => a.localeCompare(b))

const meta: Meta<typeof TStatus> = {
  component: TStatus,
  argTypes: {
    title: {
      control: 'select',
      options: titles,
    },
  },
  args: {
    title: TCommonStatuses.RUNNING,
  },
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const NoTitle: Story = {
  args: {
    title: undefined,
  },
}

export const AllTitles: Story = {
  decorators: [
    () => ({ template: '<div class="grid grid-cols-4 items-center gap-2"><story/></div>' }),
  ],
  render: (args) => ({
    components: { TStatus },
    setup: () => ({ args }),
    template: titles.map((title) => `<TStatus v-bind="args" title="${title}" />`).join(''),
  }),
}
