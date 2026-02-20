// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { TalosHardwareNamespace, TalosPCIDeviceType } from '@/api/resources'

import NodePCIDevices, { type TalosPCIDeviceSpec } from './NodePCIDevices.vue'

const meta: Meta<typeof NodePCIDevices> = {
  component: NodePCIDevices,
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<TalosPCIDeviceSpec>({
          expectedOptions: {
            namespace: TalosHardwareNamespace,
            type: TalosPCIDeviceType,
          },
          initialResources: faker.helpers.multiple(
            (_, i) => ({
              metadata: {
                namespace: TalosHardwareNamespace,
                type: TalosPCIDeviceType,
                id: `0000:00:${String(i).padStart(2, '0')}.0`,
              },
              spec: {
                class: faker.hacker.noun(),
                subclass: faker.hacker.verb(),
                vendor: faker.company.name(),
                product: faker.commerce.productName(),
                driver: faker.hacker.abbreviation(),
              },
            }),
            { count: 99 },
          ),
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
