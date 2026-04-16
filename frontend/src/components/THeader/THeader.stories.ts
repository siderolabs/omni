// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import { type NotificationSpec, NotificationSpecType } from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, NotificationType } from '@/api/resources'

import THeader from './THeader.vue'

const meta: Meta<typeof THeader> = {
  component: THeader,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<NotificationSpec>({
          expectedOptions: {
            namespace: EphemeralNamespace,
            type: NotificationType,
          },
          initialResources: faker.helpers.multiple(
            () => ({
              metadata: {
                namespace: EphemeralNamespace,
                type: NotificationType,
                id: faker.string.uuid(),
              },
              spec: {
                type: faker.helpers.enumValue(NotificationSpecType),
                title: faker.commerce.productName(),
                body: faker.commerce.productDescription(),
              },
            }),
            { count: 20 },
          ),
        }).handler,
      ],
    },
  },
}
