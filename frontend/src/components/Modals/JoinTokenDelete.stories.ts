// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { fn } from 'storybook/test'

import type { JoinTokenStatusSpec } from '@/api/omni/specs/siderolink.pb'
import { DefaultNamespace, JoinTokenStatusType } from '@/api/resources'

import JoinTokenDelete from './JoinTokenDelete.vue'

const token = faker.string.alphanumeric(32)
const machineCount = 3

const meta: Meta<typeof JoinTokenDelete> = {
  component: JoinTokenDelete,
  args: {
    open: true,
    'onUpdate:open': fn(),
    token,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<JoinTokenStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: JoinTokenStatusType,
            id: token,
          },
          initialResources: [
            {
              spec: {
                use_count: machineCount.toString(),
                warnings: faker.helpers.multiple(
                  () => ({
                    machine: faker.hacker.noun(),
                    message: faker.lorem.lines(1),
                  }),
                  { count: machineCount - 1 },
                ),
              },
              metadata: {
                namespace: DefaultNamespace,
                type: JoinTokenStatusType,
                id: token,
              },
            },
          ],
        }).handler,
      ],
    },
  },
}

export const NoWarnings: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<JoinTokenStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: JoinTokenStatusType,
            id: token,
          },
          initialResources: [
            {
              spec: {},
              metadata: {
                namespace: DefaultNamespace,
                type: JoinTokenStatusType,
                id: token,
              },
            },
          ],
        }).handler,
      ],
    },
  },
}
