// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { KernelArgsSpec, KernelArgsStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, KernelArgsStatusType, KernelArgsType } from '@/api/resources'

import MachineKernelArgs from './MachineKernelArgs.vue'

const machineId = faker.string.uuid()

const meta: Meta<typeof MachineKernelArgs> = {
  component: MachineKernelArgs,
  parameters: {
    machine: machineId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<KernelArgsStatusSpec>({
          expectedOptions: {
            type: KernelArgsStatusType,
            namespace: DefaultNamespace,
            id: machineId,
          },
          initialResources: [
            {
              spec: {
                current_args: faker.helpers.multiple(() =>
                  faker.helpers.slugify(faker.hacker.phrase()).toLowerCase(),
                ),
                current_cmdline: faker.helpers.slugify(faker.hacker.phrase()).toLowerCase(),
                unmet_conditions: faker.helpers.multiple(() =>
                  faker.helpers.slugify(faker.hacker.phrase()).toLowerCase(),
                ),
              },
              metadata: {
                type: KernelArgsStatusType,
                namespace: DefaultNamespace,
                id: machineId,
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<KernelArgsSpec>({
          expectedOptions: {
            type: KernelArgsType,
            namespace: DefaultNamespace,
            id: machineId,
          },
          initialResources: [
            {
              spec: {
                args: faker.helpers.multiple(() =>
                  faker.helpers.slugify(faker.hacker.phrase()).toLowerCase(),
                ),
              },
              metadata: {
                type: KernelArgsType,
                namespace: DefaultNamespace,
                id: machineId,
              },
            },
          ],
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
