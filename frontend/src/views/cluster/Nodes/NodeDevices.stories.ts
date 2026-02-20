// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { TalosHardwareNamespace, TalosPCIDeviceType } from '@/api/resources'

import NodeDevices, { type TalosPCIDeviceSpec } from './NodeDevices.vue'

const meta: Meta<typeof NodeDevices> = {
  component: NodeDevices,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [() => ({ template: '<div class="h-screen"><story/></div>' })],
}

export default meta
type Story = StoryObj<typeof meta>

const classes = faker.helpers.multiple(
  () => ({
    id: faker.string.uuid(),
    label: faker.hacker.noun(),
    subclasses: faker.helpers.maybe(() =>
      faker.helpers.multiple(
        () => ({
          id: faker.string.uuid(),
          label: faker.hacker.noun(),
        }),
        { count: 3 },
      ),
    ),
  }),
  { count: 15 },
)

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
            (_, i) => {
              const classNode = faker.helpers.maybe(() => faker.helpers.arrayElement(classes))
              const subclassNode = classNode?.subclasses
                ? faker.helpers.maybe(() => faker.helpers.arrayElement(classes))
                : undefined

              return {
                metadata: {
                  namespace: TalosHardwareNamespace,
                  type: TalosPCIDeviceType,
                  id: `0000:00:${String(i).padStart(2, '0')}.0`,
                },
                spec: {
                  class: classNode?.label,
                  class_id: classNode?.id,
                  subclass: subclassNode?.label,
                  subclass_id: subclassNode?.id,
                  vendor: faker.company.name(),
                  product: faker.commerce.productName(),
                  driver: faker.hacker.abbreviation(),
                },
              }
            },
            { count: 300 },
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
