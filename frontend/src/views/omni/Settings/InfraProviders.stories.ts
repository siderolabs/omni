// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { Resource } from '@/api/grpc'
import type { InfraProviderCombinedStatusSpec } from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, InfraProviderCombinedStatusType } from '@/api/resources'

import InfraProviders from './InfraProviders.vue'

const meta: Meta<typeof InfraProviders> = {
  component: InfraProviders,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<InfraProviderCombinedStatusSpec>({
          expectedOptions: {
            namespace: EphemeralNamespace,
            type: InfraProviderCombinedStatusType,
          },
          initialResources: faker.helpers.multiple<Resource<InfraProviderCombinedStatusSpec>>(
            () => ({
              metadata: { id: faker.string.uuid() },
              spec: {
                name: faker.animal.cat(),
                icon: faker.image
                  .dataUri({ type: 'svg-base64' })
                  .replace('data:image/svg+xml;base64,', ''),
                description: faker.hacker.phrase(),
                health: {
                  connected: faker.datatype.boolean(),
                  error: faker.helpers.maybe(() => faker.hacker.phrase()),
                  initialized: faker.datatype.boolean(),
                },
              },
            }),
            { count: 10 },
          ),
        }).handler,
      ],
    },
  },
}
