// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { MetaNamespace, ResourceDefinitionType } from '@/api/resources'
import { commonHandlers, machineStatuses } from '@/views/Internals/Internals.mocks'

import ResourceDefinitionList from './ResourceDefinitionList.vue'

const meta: Meta<typeof ResourceDefinitionList> = {
  component: ResourceDefinitionList,
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

export const Omni: Story = {
  args: {
    target: { runtime: 'omni' },
  },
  beforeEach({ msw }) {
    msw.use(...commonHandlers)
  },
}

export const Talos: Story = {
  args: {
    target: { runtime: 'talos', machine: machineStatuses[0].metadata.id! },
  },
  beforeEach({ msw }) {
    msw.use(...commonHandlers)
  },
}

export const NoDefinitions: Story = {
  args: {
    target: { runtime: 'omni' },
  },
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler({
        expectedOptions: {
          namespace: MetaNamespace,
          type: ResourceDefinitionType,
        },
      }).handler,
      ...commonHandlers,
    )
  },
}
