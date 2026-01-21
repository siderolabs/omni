// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { type InfraMachineSpec } from '@/api/omni/specs/infra.pb'
import { InfraMachineType, InfraProviderNamespace, LabelInfraProviderID } from '@/api/resources'

import MachinesPending from './MachinesPending.vue'

const meta: Meta<typeof MachinesPending> = {
  component: MachinesPending,
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<InfraMachineSpec>({
          expectedOptions: { type: InfraMachineType, namespace: InfraProviderNamespace },
          totalResults: 100,
          initialResources: ({ limit = 10, offset = 0 }) => {
            faker.seed(offset)

            return faker.helpers.multiple(
              () => ({
                spec: {},
                metadata: {
                  id: faker.string.uuid(),
                  labels: { [LabelInfraProviderID]: 'bare-metal' },
                },
              }),
              { count: limit },
            )
          },
        }).handler,
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [createWatchStreamHandler().handler],
    },
  },
}
