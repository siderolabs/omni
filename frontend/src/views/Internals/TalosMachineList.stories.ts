// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import { commonHandlers } from '@/views/Internals/Internals.mocks'

import TalosMachineList from './TalosMachineList.vue'

const meta: Meta<typeof TalosMachineList> = {
  component: TalosMachineList,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    () => ({
      template: '<div class="h-screen"><story /></div>',
    }),
  ],
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...commonHandlers)
  },
}

export const NoMachines: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler({
        expectedOptions: {
          namespace: DefaultNamespace,
          type: MachineStatusType,
        },
      }).handler,
      ...commonHandlers,
    )
  },
}
